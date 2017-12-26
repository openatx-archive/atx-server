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
	"github.com/tomasen/realip"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// Time allowed to write message to the client
	wsWriteWait = 10 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	wsPingPeriod = 10 * time.Second

	// Time allowed to read the next pong message from client
	wsPongWait = wsPingPeriod * 3
)

func renderHTML(w http.ResponseWriter, filename string, value interface{}) {
	tmpl := template.Must(template.New("").Delims("[[", "]]").ParseGlob("templates/*.html"))
	tmpl.ExecuteTemplate(w, filename, value)
	// content, _ := ioutil.ReadFile("templates/" + filename)
	// template.Must(template.New(filename).Parse(string(content))).Execute(w, nil)
}

func newHandler() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(map[string]string{
			"server":    version,
			"atx-agent": atxAgentVersion,
		})
	})

	// 设备信息修改
	r.HandleFunc("/devices/{udid}/edit", func(w http.ResponseWriter, r *http.Request) {
		udid := mux.Vars(r)["udid"]
		renderHTML(w, "edit.html", udid)
	}).Methods("GET")

	r.HandleFunc("/products/{brand}/{model}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		brand, model := vars["brand"], vars["model"]
		products, err := db.ProductsFindAll(brand, model)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(products)
	})

	r.HandleFunc("/devices/{udid}/product", func(w http.ResponseWriter, r *http.Request) {
		var product proto.Product
		err := json.NewDecoder(r.Body).Decode(&product)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if product.Id == "" {
			http.Error(w, "product id is required", http.StatusForbidden)
			return
		}
		if err := db.ProductUpdate(product.Id, product); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		err = db.DeviceUpdate(proto.DeviceInfo{
			Udid: mux.Vars(r)["udid"],
			Product: &proto.Product{
				Id: product.Id,
			},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
	}).Methods("PUT")

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
			ws.WriteJSON(map[string]string{
				"error": err.Error(),
			})
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
		renderHTML(w, "index.html", nil)
	})
	r.Handle("/assets/{(.*)}", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "assets/favicon.ico")
	})

	r.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		devices := db.DeviceList()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(devices)
	})

	r.HandleFunc("/devices/{query}/info", func(w http.ResponseWriter, r *http.Request) {
		query := mux.Vars(r)["query"]
		udid, err := deviceQueryToUdid(query)
		if err != nil {
			io.WriteString(w, "Failure, device "+query+" not found")
			return
		}
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			info, _ := db.DeviceGet(udid)
			json.NewEncoder(w).Encode(info)
			return
		}
		// POST
		var info proto.DeviceInfo
		if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
			io.WriteString(w, err.Error())
			return
		}
		info.Udid = udid
		db.DeviceUpdate(info) // TODO: update database
		io.WriteString(w, "Success")
	}).Methods("GET", "POST")

	r.HandleFunc("/property", func(w http.ResponseWriter, r *http.Request) {
		clientIp := realip.FromRequest(r)
		udid, err := deviceQueryToUdid("ip:" + clientIp)
		if err != nil {
			io.WriteString(w, "init with uiautomator2")
			return
		}
		info, err := db.DeviceGet(udid)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if r.Method == "POST" {
			var id string = r.FormValue("id")
			if id == "" && r.FormValue("id_number") != "" {
				id = "HIH-PHO-" + r.FormValue("id_number")
			}
			db.DeviceUpdate(proto.DeviceInfo{
				Udid:       info.Udid,
				PropertyId: id,
			})
			info.PropertyId = id
			io.WriteString(w, "<h1>Updated to "+id+"</h1>")
			return
		}
		renderHTML(w, "property.html", info.PropertyId)
	}).Methods("GET", "POST")

	// TODO
	// Must put in front of "/devices/{query}/reserved"
	// r.HandleFunc("/devices/:random/reserved", func(w http.ResponseWriter, r *http.Request) {
	// 	info, _ := hostsManager.Random()
	// 	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// 	json.NewEncoder(w).Encode(info)
	// }).Methods("POST")

	r.HandleFunc("/devices/{query}/reserved", func(w http.ResponseWriter, r *http.Request) {
		query := mux.Vars(r)["query"]
		udid, err := deviceQueryToUdid(query)
		// info := hostsManager.Lookup(query)
		if err != nil {
			http.Error(w, "Device not found "+err.Error(), http.StatusGone)
			return
		}
		if r.Method == "POST" {
			info, err := db.DeviceGet(udid)
			if err != nil {
				http.Error(w, "Device get error "+err.Error(), http.StatusGone)
				return
			}
			if toBool(info.Using) {
				http.Error(w, "Device is using", http.StatusForbidden)
				return
			}
			db.DeviceUpdate(proto.DeviceInfo{
				Udid:  info.Udid,
				Using: newBool(true),
			})
			io.WriteString(w, "Success")
			return
		}
		// DELETE
		db.DeviceUpdate(proto.DeviceInfo{
			Udid:  udid,
			Using: newBool(false),
		})
		io.WriteString(w, "Release success")
	}).Methods("POST", "DELETE")

	r.HandleFunc("/devices/{query}/shell", func(w http.ResponseWriter, r *http.Request) {
		query := mux.Vars(r)["query"]
		udid, err := deviceQueryToUdid(query)
		if err != nil {
			http.Error(w, "Device not found", 410)
			return
		}
		info, err := db.DeviceGet(udid)
		if err != nil {
			http.Error(w, "Device get error "+err.Error(), 500)
			return
		}

		command := r.FormValue("command")
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		output, err := runAndroidShell(info.IP, command)
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
