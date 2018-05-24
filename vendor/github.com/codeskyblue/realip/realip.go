package realip

import (
	"net"
	"net/http"
	"strings"
)

// FromRequest return real request IP
func FromRequest(r *http.Request) string {
	for _, h := range []string{"X-Forwarded-For", "X-Real-IP"} {
		for _, ip := range strings.Split(r.Header.Get(h), ", ") {
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	remoteIp := r.RemoteAddr
	if strings.ContainsRune(r.RemoteAddr, ':') {
		remoteIp, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	return remoteIp
}
