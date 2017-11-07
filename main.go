package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/openatx/atx-server/proto"
)

const (
	version         = "dev"
	atxAgentVersion = "0.0.8"
)

var (
	upgrader     = websocket.Upgrader{}
	addr         = flag.String("addr", ":8080", "http service address")
	hostsManager = NewHostsManager()

	// Time allowed to write message to the client
	wsWriteWait = 10 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	wsPingPeriod = 10 * time.Second

	// Time allowed to read the next pong message from client
	wsPongWait = wsPingPeriod * 3
)

type HostsManager struct {
	maps map[string]*proto.DeviceInfo
	mu   sync.RWMutex
}

func NewHostsManager() *HostsManager {
	return &HostsManager{
		maps: make(map[string]*proto.DeviceInfo),
	}
}

func (t *HostsManager) Get(host string) *proto.DeviceInfo {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.maps[host]
}

func (t *HostsManager) Add(host string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if info, ok := t.maps[host]; ok {
		info.IP = host
	} else {
		t.maps[host] = &proto.DeviceInfo{
			IP:              host,
			ConnectionCount: 1,
		}
	}
}

func (t *HostsManager) AddFromDeviceInfo(devInfo *proto.DeviceInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()
	udid := devInfo.Udid
	if info, ok := t.maps[udid]; ok {
		info.IP = devInfo.IP
	} else {
		devInfo.ConnectionCount = 1
		t.maps[udid] = devInfo
	}
}

func (t *HostsManager) Remove(udid string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if info, ok := t.maps[udid]; ok {
		info.ConnectionCount--
		if info.ConnectionCount <= 0 {
			delete(t.maps, udid)
		}
	}
}

func handleWebsocketMessage(host string, message []byte) {
	msg := &proto.CommonMessage{}
	reader := json.NewDecoder(bytes.NewReader(message))
	if err := reader.Decode(msg); err != nil {
		return
	}
	fmt.Printf("msg type: %v\n", msg.Type)
	if msg.Type == proto.DeviceInfoMessage {
		jsonData, _ := json.Marshal(msg.Data)
		devInfo := hostsManager.maps[host] // TODO: lock and unlock
		json.NewDecoder(bytes.NewReader(jsonData)).Decode(devInfo)
		fmt.Printf("brand: %s\n", devInfo.Brand)
	}
}

func echo(w http.ResponseWriter, r *http.Request) {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	log.Printf("new connection: %s", host)

	defer func() {
		log.Printf("connection lost: %s", host)
		ws.Close()
	}()

	// hostsManager.Add(host)

	ws.SetReadDeadline(time.Now().Add(wsPongWait))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	// Read device info
	message := &proto.CommonMessage{}
	if err := ws.ReadJSON(message); err != nil {
		log.Println("error: read json message")
		return
	}
	if message.Type != proto.DeviceInfoMessage {
		log.Printf("error: first message must be device info, but got %v", message.Type)
		return
	}
	devInfo := new(proto.DeviceInfo)
	jsonData, _ := json.Marshal(message.Data)
	json.NewDecoder(bytes.NewReader(jsonData)).Decode(devInfo)
	if devInfo.Udid == "" {
		log.Printf("error: udid is empty")
		return
	}
	devInfo.IP = host
	hostsManager.AddFromDeviceInfo(devInfo)
	defer func(udid string) {
		hostsManager.Remove(udid)
	}(devInfo.Udid)

	// ping ticker
	go func() {
		pingTicker := time.NewTicker(wsPingPeriod)
		defer pingTicker.Stop()
		for {
			select {
			case <-pingTicker.C:
				ws.SetWriteDeadline(time.Now().Add(wsWriteWait))
				// here, writeMessage is not thread safe
				if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
					return
				}
			}
		}
	}()

	// Listen device info update
	for {
		mt, message, err := ws.ReadMessage()
		if err != nil {
			log.Println(host, "websocket connection closed")
			break
		}
		if mt == websocket.TextMessage {
			handleWebsocketMessage(host, message)
		}
	}
}

func unlockAll() {
	for host := range hostsManager.maps {
		fmt.Printf("unlock %s\n", host)
	}
}

func main() {
	flag.Parse()
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	http.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(map[string]string{
			"server":    version,
			"atx-agent": atxAgentVersion,
		})
	})

	http.HandleFunc("/echo", echo)

	batchRunCommand := func(command string) {
		wg := sync.WaitGroup{}
		failCount := 0
		for _, devInfo := range hostsManager.maps {
			wg.Add(1)
			go func(host string) {
				u := &url.URL{}
				params := url.Values{}
				params.Add("command", command)
				u.RawQuery = params.Encode()
				u.Scheme = "http"
				u.Path = "/shell"
				u.Scheme = "http"
				u.Host = host + ":7912"
				log.Println(u.String())
				resp, err := http.Get(u.String())
				if err != nil {
					failCount++
				} else {
					resp.Body.Close()
				}
				wg.Done()
			}(devInfo.IP)
		}
		wg.Wait()
	}
	http.HandleFunc("/api/v1/batch/unlock", func(w http.ResponseWriter, r *http.Request) {
		batchRunCommand("am start -W --user 0 -a com.github.uiautomator.ACTION_IDENTIFY; input keyevent HOME")
		io.WriteString(w, "Success")
	})
	http.HandleFunc("/api/v1/batch/lock", func(w http.ResponseWriter, r *http.Request) {
		batchRunCommand("input keyevent POWER")
		io.WriteString(w, "Success")
	})
	http.HandleFunc("/api/v1/batch/shell", func(w http.ResponseWriter, r *http.Request) {
		command := r.FormValue("command")
		batchRunCommand(command)
		io.WriteString(w, "Success")
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		devices := make([]*proto.DeviceInfo, 0)
		for _, info := range hostsManager.maps {
			devices = append(devices, info)
			// fmt.Printf("%s: %s %s %s\n", host, info.Serial, info.Brand, info.Model)
		}
		template.Must(template.ParseFiles("index.html")).Execute(w, devices)
	})
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "favicon.ico")
	})

	http.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		devices := make([]*proto.DeviceInfo, 0)
		for _, info := range hostsManager.maps {
			devices = append(devices, info)
			// fmt.Printf("%s: %s %s %s\n", host, info.Serial, info.Brand, info.Model)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(devices)
	})
	log.Fatal(http.ListenAndServe(*addr, nil))
}
