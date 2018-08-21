package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/codeskyblue/dingrobot"
	"github.com/gorilla/websocket"
	"github.com/openatx/atx-server/proto"
	"github.com/qiniu/log"
)

const (
	version                = "dev"
	defaultATXAgentVersion = "0.4.3"
)

var (
	port            = kingpin.Flag("port", "http server listen port").Short('p').Default("8000").Int()
	addr            = kingpin.Flag("addr", "http server listen address").Default(":8000").String()
	rdbAddr         = kingpin.Flag("rdbaddr", "rethinkdb address").Default("localhost:28015").String()
	rdbName         = kingpin.Flag("rdbname", "rethinkdb database name").Default("atxserver").String()
	videoBackend    = kingpin.Flag("video-backend", "backend service for encoding images to video").Default("http://localhost:7000").String()
	atxAgentVersion string
	dingtalkToken   string
)

func handleWebsocketMessage(host string, message []byte) {
	return
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

	if devInfo.Memory != nil {
		around := int(math.Ceil(float64(devInfo.Memory.Total-512*1024) / 1024.0 / 1024.0)) // around
		devInfo.Memory.Around = fmt.Sprintf("%d GB", around)
	}

	db.DeviceUpdateOrInsert(*devInfo)
	defer func() {
		db.SetDeviceAbsent(devInfo.Udid)
		// TODO(ssx): global var, not very function programing
		if info, err := db.DeviceGet(devInfo.Udid); err == nil && dingtalkToken != "" {
			robot := dingrobot.New(dingtalkToken)
			if err := robot.Text(info.PropertyId + " " + info.Serial + "\n" + info.Brand + " " + info.Model + " " + info.IP + " offline"); err != nil {
				log.Println("dingding send text err:", err)
			}
		}
	}()

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

	devices, _ := db.DeviceList()
	for _, devInfo := range devices {
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
	// Refs: atx-agent version https://github.com/openatx/atx-agent/releases
	kingpin.Flag("agent", "atx-agent version").Default(defaultATXAgentVersion).StringVar(&atxAgentVersion)
	// FIXME(ssx): Ding talk is disabled because of too many boring messages
	kingpin.Flag("ding-token", "DingDing robot token (env: DING_TOKEN)").OverrideDefaultFromEnvar("DING_TOKEN").StringVar(&dingtalkToken)
	kingpin.Version(version)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	// log.SetFlags(log.Lshortfile | log.LstdFlags)
	// log.SetLevel(log.DebugLevel)
	// log.SetFormatter(&log.TextFormatter{})
	// inforus.AddHookDefault()

	if *port != 8000 {
		*addr = fmt.Sprintf(":%d", *port)
	}

	if dingtalkToken != "" {
		log.Println("dingtalk notification enabled")
		if err := dingrobot.New(dingtalkToken).Text("atx-server started"); err != nil {
			log.Println("dingtalk test notification err:", err)
		}
	}

	log.Info("initial database")
	initDB(*rdbAddr, *rdbName)
	log.Info("listen address", *addr)
	log.Fatal(http.ListenAndServe(*addr, newHandler()))
}
