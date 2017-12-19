package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	accesslog "github.com/mash/go-accesslog"
	"github.com/openatx/atx-server/proto"
	log "github.com/sirupsen/logrus"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	hostsManager = NewHostsManager()

	// Time allowed to write message to the client
	wsWriteWait = 10 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	wsPingPeriod = 10 * time.Second

	// Time allowed to read the next pong message from client
	wsPongWait = wsPingPeriod * 3
)

func newHandler() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(map[string]string{
			"server":    version,
			"atx-agent": atxAgentVersion,
		})
	})
	r.HandleFunc("/echo", echo)

	r.HandleFunc("/feeds", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer ws.Close()
		feeds, cancel, err := db.WatchDeviceChanges()
		if err != nil {
			ws.WriteMessage(websocket.TextMessage, []byte("database error: "+err.Error()))
			return
		}
		go func() {
			defer cancel()
			for {
				_, _, err := ws.ReadMessage()
				if err != nil {
					break
				}
			}
			log.Debug("ws read closed")
		}()
		for change := range feeds {
			buf := bytes.NewBuffer(nil)
			json.NewEncoder(buf).Encode(map[string]interface{}{
				"new": change.NewValue,
				"old": change.OldValue,
			})
			err := ws.WriteMessage(websocket.TextMessage, buf.Bytes()) // []byte(`{"new": "haha", "old": "wowo"}`))
			if err != nil {
				break
			}
		}
		log.Debug("ws write closed")
	})

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
		devices := db.DeviceList()
		// devices := make([]*proto.DeviceInfo, 0)
		// for _, info := range hostsManager.maps {
		// 	devices = append(devices, info)
		// 	// fmt.Printf("%s: %s %s %s\n", host, info.Serial, info.Brand, info.Model)
		// }
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
		db.UpdateOrInsertDevice(*info) // TODO: update database
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
			db.UpdateOrInsertDevice(proto.DeviceInfo{
				Udid:  info.Udid,
				Using: newBool(true),
			})
			io.WriteString(w, "Success")
			return
		}
		info.Reserved = ""
		db.UpdateOrInsertDevice(proto.DeviceInfo{
			Udid:  info.Udid,
			Using: newBool(false),
		})
		// db.UpdateOrInsertDevice(*info)
		// TODO: implement lookup in rethinkdb
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
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		output, err := runAndroidShell(dev.IP, command)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
		} else {
			io.WriteString(w, output) // the output is already json
		}
	}).Methods("POST")

	return accesslog.NewLoggingHandler(r, HTTPLogger{})
}
