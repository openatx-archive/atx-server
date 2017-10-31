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

	"github.com/gorilla/websocket"
	"github.com/openatx/atx-server/proto"
)

var (
	upgrader     = websocket.Upgrader{}
	addr         = flag.String("addr", ":8080", "http service address")
	hostsManager = NewHostsManager()
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

func (t *HostsManager) Remove(host string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if info, ok := t.maps["host"]; ok {
		info.ConnectionCount--
		if info.ConnectionCount <= 0 {
			delete(t.maps, host)
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
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	log.Printf("new connection: %s", host)
	hostsManager.Add(host)

	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println(host, "websocket connection closed")
			break
		}
		if mt == websocket.TextMessage {
			handleWebsocketMessage(host, message)
		}
		// log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
	log.Printf("off connection: %s", host)
	hostsManager.Remove(host)
}

func unlockAll() {
	for host := range hostsManager.maps {
		fmt.Printf("unlock %s\n", host)
	}
}

func main() {
	flag.Parse()
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	http.HandleFunc("/echo", echo)

	batchRunCommand := func(command string) {
		wg := sync.WaitGroup{}
		failCount := 0
		for host := range hostsManager.maps {
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
			}(host)
		}
		wg.Wait()
	}
	http.HandleFunc("/api/v1/batch/unlock", func(w http.ResponseWriter, r *http.Request) {
		batchRunCommand("am start --user 0 -a com.github.uiautomator.ACTION_IDENTIFY; input keyevent HOME")
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
