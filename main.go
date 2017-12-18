package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
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
	atxAgentVersion = "0.1.1"
)

var addr = flag.String("addr", ":8080", "http service address")

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

func runAndroidShell(ip string, command string) (output string, err error) {
	u, _ := url.Parse("http://" + ip + ":7912/shell")
	params := url.Values{}
	params.Add("command", command)
	u.RawQuery = params.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		return
	}
	defer resp.Body.Close()
	jsondata, err := ioutil.ReadAll(resp.Body)
	return string(jsondata), err
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
	log.Fatal(http.ListenAndServe(*addr, newHandler()))

}
