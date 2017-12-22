package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/codeskyblue/inforus"

	"github.com/gorilla/websocket"
	"github.com/openatx/atx-server/proto"
	log "github.com/sirupsen/logrus"
)

const (
	version         = "dev"
	atxAgentVersion = "0.1.2"
)

var (
	addr    = flag.String("addr", ":8000", "http service address")
	rdbAddr = flag.String("rdbaddr", "localhost:28015", "rethinkdb address")
	rdbName = flag.String("rdbname", "atxserver", "rethinkdb database name")
)

func handleWebsocketMessage(host string, message []byte) {
	return
	// msg := &proto.CommonMessage{}
	// reader := json.NewDecoder(bytes.NewReader(message))
	// if err := reader.Decode(msg); err != nil {
	// 	return
	// }
	// fmt.Printf("msg type: %v\n", msg.Type)
	// if msg.Type == proto.DeviceInfoMessage {
	// 	jsonData, _ := json.Marshal(msg.Data)
	// 	// devInfo := hostsManager.maps[host] // TODO: lock and unlock
	// 	json.NewDecoder(bytes.NewReader(jsonData)).Decode(devInfo)
	// 	fmt.Printf("brand: %s\n", devInfo.Brand)
	// }
}

func echo(w http.ResponseWriter, r *http.Request) {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	log.Debugf("new connection: %s", host)

	defer func() {
		log.Debugf("connection lost: %s", host)
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
		log.Warn("error: read json message")
		return
	}
	if message.Type != proto.DeviceInfoMessage {
		log.Warnf("error: first message must be device info, but got %v", message.Type)
		return
	}
	devInfo := new(proto.DeviceInfo)
	jsonData, _ := json.Marshal(message.Data)
	json.NewDecoder(bytes.NewReader(jsonData)).Decode(devInfo)
	if devInfo.Udid == "" {
		log.Warnf("error: udid is empty")
		return
	}
	devInfo.IP = host
	log.Debugf("client ip:%s product:%s brand:%s", devInfo.IP, devInfo.Model, devInfo.Brand)

	db.UpdateOrInsertDevice(*devInfo)
	defer db.SetDeviceAbsent(devInfo.Udid)

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

	for _, devInfo := range db.DeviceList() {
		if devInfo.Present == nil || !*devInfo.Present {
			continue
		}

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
	// log.SetFlags(log.Lshortfile | log.LstdFlags)
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{})
	inforus.AddHookDefault()

	initDB(*rdbAddr, *rdbName)
	log.Fatal(http.ListenAndServe(*addr, newHandler()))
}
