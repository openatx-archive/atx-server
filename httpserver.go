package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/codeskyblue/heartbeat"
	"github.com/codeskyblue/realip"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/koding/websocketproxy"
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

	// Time allowed to write message to the client
	wsWriteWait = 10 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	wsPingPeriod = 10 * time.Second

	// Time allowed to read the next pong message from client
	wsPongWait = wsPingPeriod * 3

	funcMap template.FuncMap
)

func init() {
	funcMap = template.FuncMap{
		"title": strings.Title,
		"urlhash": func(s string) string {
			path := strings.TrimPrefix(s, "/")
			info, err := os.Stat(path)
			if err != nil {
				return s + "#no-such-file"
			}
			return fmt.Sprintf("%s?t=%d", s, info.ModTime().Unix())
		},
	}
}

func renderHTML(w http.ResponseWriter, filename string, value interface{}) {
	tmpl := template.Must(template.New("").Funcs(funcMap).Delims("[[", "]]").ParseGlob("templates/*.html"))
	tmpl.ExecuteTemplate(w, filename, value)
	// content, _ := ioutil.ReadFile("templates/" + filename)
	// template.Must(template.New(filename).Parse(string(content))).Execute(w, nil)
}

func renderJSON(w http.ResponseWriter, data interface{}) {
	js, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(js)))
	w.Write(js)
}

func newHandler() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		renderJSON(w, map[string]string{
			"server":    version,
			"atx-agent": atxAgentVersion,
		})
	})

	// 设备远程控制
	r.HandleFunc("/devices/ip:{ip}/remote", func(w http.ResponseWriter, r *http.Request) {
		ip := mux.Vars(r)["ip"]
		renderHTML(w, "remote.html", ip)
	}).Methods("GET")

	r.HandleFunc("/devices/{udid}/remote", func(w http.ResponseWriter, r *http.Request) {
		udid := mux.Vars(r)["udid"]
		info, err := db.DeviceGet(udid)
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		renderHTML(w, "remote.html", info.IP)
	}).Methods("GET")

	// 设备信息修改
	r.HandleFunc("/devices/{udid}/edit", func(w http.ResponseWriter, r *http.Request) {
		udid := mux.Vars(r)["udid"]
		renderHTML(w, "edit.html", udid)
	}).Methods("GET")

	// Video-backend starts
	videoProxyURL, _ := url.Parse(*videoBackend)
	wsProxyURL, _ := url.Parse(*videoBackend)
	wsProxyURL.Scheme = "ws"

	videoProxy := httputil.NewSingleHostReverseProxy(videoProxyURL)
	wsVideoProxy := websocketproxy.NewProxy(wsProxyURL)

	r.PathPrefix("/videos").Handler(videoProxy).Methods("GET", "DELETE")
	r.Handle("/video/images2video", videoProxy) // not working with POST proxy
	r.PathPrefix("/static/videos/").Handler(videoProxy)
	r.Handle("/video/convert", wsVideoProxy)
	// End of video-backend

	r.HandleFunc("/products/{brand}/{model}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		brand, model := vars["brand"], vars["model"]
		products, err := db.ProductsFindAll(brand, model)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		renderJSON(w, products)
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
		renderJSON(w, map[string]interface{}{
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

	r.PathPrefix("/assets").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "assets/favicon.ico")
	})

	r.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		devices := db.DeviceList()
		renderJSON(w, devices)
	})

	r.HandleFunc("/devices/{query}/info", func(w http.ResponseWriter, r *http.Request) {
		query := mux.Vars(r)["query"]
		udid, err := deviceQueryToUdid(query)
		if err != nil {
			io.WriteString(w, "Failure, device "+query+" not found")
			return
		}
		if r.Method == "GET" {
			info, _ := db.DeviceGet(udid)
			renderJSON(w, info)
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

	r.HandleFunc("/devices/{query}/reserved", func(w http.ResponseWriter, r *http.Request) {
		query := mux.Vars(r)["query"]
		udid, err := deviceQueryToUdid(query)
		if err != nil {
			http.Error(w, "Device not found "+err.Error(), http.StatusGone)
			return
		}
		info, err := db.DeviceGet(udid)
		if err != nil {
			http.Error(w, "Device get error "+err.Error(), http.StatusGone)
			return
		}
		// create websocket connection
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		defer ws.Close()
		if toBool(info.Using) {
			log.Printf("Device %s is using", udid)
			return
		}
		db.DeviceUpdate(proto.DeviceInfo{
			Udid:         info.Udid,
			Using:        newBool(true),
			UsingBeganAt: time.Now(),
		})
		defer func() {
			db.DeviceUpdate(proto.DeviceInfo{
				Udid:  udid,
				Using: newBool(false),
			})
			go func() {
				port := info.Port
				if port == 0 {
					port = 7912
				}
				reqURL := "http://" + info.IP + ":" + strconv.Itoa(port) + "/shell"
				req, _ := http.NewRequest("GET", reqURL, nil)
				q := req.URL.Query()
				q.Add("command", "am start -n com.github.uiautomator/.IdentifyActivity")
				req.URL.RawQuery = q.Encode()

				resp, err := http.DefaultClient.Do(req)
				if err == nil {
					resp.Body.Close()
				}
			}()
		}()
		// wait until ws disconnected
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	}).Methods("GET")

	r.HandleFunc("/devices/{query}/reserved", func(w http.ResponseWriter, r *http.Request) {
		query := mux.Vars(r)["query"]
		udid, err := deviceQueryToUdid(query)
		// info := hostsManager.Lookup(query)
		if err != nil {
			http.Error(w, "Device not found "+err.Error(), http.StatusGone)
			return
		}
		log.Println("HEllo")
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
				Udid:         info.Udid,
				Using:        newBool(true),
				UsingBeganAt: time.Now(),
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
		output, err := runAndroidShell(info.IP, command)
		if err != nil {
			renderJSON(w, map[string]string{
				"error": err.Error(),
			})
		} else {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			io.WriteString(w, output) // the output is already json
		}
	}).Methods("POST")

	// heartbeat for reverse proxies (adb forward device 7912 port)
	hbs := heartbeat.NewServer("hello kitty", 15*time.Second)
	hbs.OnConnect = func(identifier string, r *http.Request) {
		host := realip.FromRequest(r)
		db.UpdateOrInsertDevice(proto.DeviceInfo{
			Udid: identifier,
			IP:   host,
		})
		log.Println(identifier, "is online")
	}

	// Called when client ip changes
	hbs.OnReconnect = func(identifier string, r *http.Request) {
		host := realip.FromRequest(r)
		db.UpdateOrInsertDevice(proto.DeviceInfo{
			Udid: identifier,
			IP:   host,
		})
		log.Println(identifier, "is reconnected")
	}

	hbs.OnDisconnect = func(identifier string) {
		db.SetDeviceAbsent(identifier)
		log.Println(identifier, "is offline")
	}
	r.Handle("/heartbeat", hbs)

	return accesslog.NewLoggingHandler(r, HTTPLogger{})
}
