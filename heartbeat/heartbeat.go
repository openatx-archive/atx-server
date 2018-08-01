/*
FormValue id and port is required

Client send request example

$ curl -X POST -F id=cfa124af -F port=8000
*/
package heartbeat

import (
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/tomasen/realip"
)

type Server struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	receiver Receiver
}

// NewServer return http.Handler
func NewServer(receiver Receiver) *Server {
	return &Server{
		sessions: make(map[string]*Session),
		receiver: receiver,
	}
}

func (h *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	if id == "" {
		http.Error(w, "param id is required", 400)
		return
	}
	port, _ := strconv.Atoi(r.FormValue("port"))
	if port == 0 {
		http.Error(w, "param port is required", 400)
		return
	}
	ip := r.FormValue("ip")
	if ip == "" {
		ip = realip.FromRequest(r)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	ctx := Context{
		IP:      ip,
		ID:      id,
		Request: r,
	}
	sess, exists := h.sessions[id]
	if !exists {
		if err := h.receiver.OnConnect(ctx); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		h.sessions[id] = &Session{
			Timeout:    time.Second * 15,
			sigC:       make(chan bool),
			remoteIP:   ip,
			remotePort: port,
		}
		go func() {
			h.sessions[id].drain()
			h.receiver.OnDisconnect(id)
			delete(h.sessions, id)
		}()
	} else {
		if ip != sess.remoteIP || port != sess.remotePort {
			sess.remoteIP = ip
			sess.remotePort = port
			if err := h.receiver.OnConnect(ctx); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
		}
		sess.Update()
	}
	if r.FormValue("data") == "" || r.FormValue("data") == "null" {
		io.WriteString(w, "success ping\n")
		return
	}
	if err := h.receiver.OnRequest(ctx); err != nil {
		http.Error(w, err.Error(), 400)
	} else {
		io.WriteString(w, "success request\n")
	}
}

// Receiver defines on request
type Receiver interface {
	OnConnect(ctx Context) error
	OnDisconnect(id string)
	OnRequest(ctx Context) error
}

type Session struct {
	id         string
	remoteIP   string
	remotePort int
	Timeout    time.Duration
	sigC       chan bool
}

func (hs *Session) Update() {
	select {
	case hs.sigC <- true:
	case <-time.After(100 * time.Millisecond):
	}
}

func (hs *Session) drain() {
	for {
		select {
		case <-time.After(hs.Timeout):
			return
		case <-hs.sigC:
		}
	}
}

type Context struct {
	Request *http.Request
	IP      string
	ID      string
}
