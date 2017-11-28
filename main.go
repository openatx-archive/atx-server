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

	"github.com/gorilla/mux"

	"github.com/gorilla/websocket"
	accesslog "github.com/mash/go-accesslog"
	"github.com/openatx/atx-server/proto"
)

const (
	version         = "dev"
	atxAgentVersion = "0.1.0"
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
	log.Printf("client ip:%s product:%s brand:%s", devInfo.IP, devInfo.Model, devInfo.Brand)
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

func runAndroidShell(ip string, command string) string {
	u, _ := url.Parse("http://" + ip + ":7912/shell")
	params := url.Values{}
	params.Add("command", command)
	u.RawQuery = params.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
	} else {
		resp.Body.Close()
	}
	return ""
}

func batchRunCommand(command string) {
	wg := sync.WaitGroup{}
	// failCount := 0
	for _, devInfo := range hostsManager.maps {
		wg.Add(1)
		go func(ip string) {
			runAndroidShell(ip, command)
			wg.Done()
		}(devInfo.IP)
	}
	wg.Wait()
}

func main() {
	flag.Parse()
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	r := mux.NewRouter()

	r.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(map[string]string{
			"server":    version,
			"atx-agent": atxAgentVersion,
		})
	})

	r.HandleFunc("/echo", echo)

	r.HandleFunc("/api/v1/batch/unlock", func(w http.ResponseWriter, r *http.Request) {
		batchRunCommand("am start -W --user 0 -a com.github.uiautomator.ACTION_IDENTIFY; input keyevent HOME")
		io.WriteString(w, "Success")
	})
	r.HandleFunc("/api/v1/batch/lock", func(w http.ResponseWriter, r *http.Request) {
		batchRunCommand("input keyevent POWER")
		io.WriteString(w, "Success")
	})
	r.HandleFunc("/api/v1/batch/shell", func(w http.ResponseWriter, r *http.Request) {
		command := r.FormValue("command")
		batchRunCommand(command)
		io.WriteString(w, "Success")
	})

	// r.HandleFunc("/api/v1/phones/identify")
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		devices := make([]*proto.DeviceInfo, 0)
		for _, info := range hostsManager.maps {
			devices = append(devices, info)
			// fmt.Printf("%s: %s %s %s\n", host, info.Serial, info.Brand, info.Model)
		}
		tmpl := template.Must(template.New("").Delims("[[", "]]").ParseGlob("templates/*.html"))
		tmpl.ExecuteTemplate(w, "index.html", devices)
	})
	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "assets/favicon.ico")
	})

	r.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		devices := make([]*proto.DeviceInfo, 0)
		for _, info := range hostsManager.maps {
			devices = append(devices, info)
			// fmt.Printf("%s: %s %s %s\n", host, info.Serial, info.Brand, info.Model)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(devices)
	})

	r.HandleFunc("/devices/{query}/info", func(w http.ResponseWriter, r *http.Request) {
		query := mux.Vars(r)["query"]
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		info := hostsManager.Lookup(query)
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(info)
			return
		}
		if info == nil {
			io.WriteString(w, "Failure, device "+query+" not found")
			return
		}
		json.NewDecoder(r.Body).Decode(info)
		io.WriteString(w, "Success")
	}).Methods("GET", "POST")

	// Must put in front of "/devices/{query}/reserved"
	r.HandleFunc("/devices/:random/reserved", func(w http.ResponseWriter, r *http.Request) {
		info, _ := hostsManager.Random()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(info)
	}).Methods("POST")

	r.HandleFunc("/devices/{query}/reserved", func(w http.ResponseWriter, r *http.Request) {
		query := mux.Vars(r)["query"]
		info := hostsManager.Lookup(query)
		if info == nil {
			http.Error(w, "Device not found", http.StatusGone)
			return
		}
		if r.Method == "POST" {
			if info.Reserved != "" {
				http.Error(w, "Device is using", http.StatusForbidden)
				return
			}
			info.Reserved = "hzsunshx"
			io.WriteString(w, "Success")
			return
		}
		info.Reserved = ""
		io.WriteString(w, "Release success")
	}).Methods("POST", "DELETE")

	r.HandleFunc("/devices/{query}/shell", func(w http.ResponseWriter, r *http.Request) {
		query := mux.Vars(r)["query"]
		dev := hostsManager.Lookup(query)
		if dev == nil {
			http.Error(w, "Device not found", 410)
			return
		}
		command := r.FormValue("command")
		runAndroidShell(dev.IP, command) //"am start -W --user 0 -a com.github.uiautomator.ACTION_IDENTIFY -e theme red")
		io.WriteString(w, "Locate success")
	}).Methods("POST")

	rt := accesslog.NewLoggingHandler(r, HTTPLogger{})
	log.Fatal(http.ListenAndServe(*addr, rt))
}
