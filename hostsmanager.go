package main

import (
	"sync"

	"github.com/openatx/atx-server/proto"
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

// A return value of nil indicates not found
func (t *HostsManager) FromIP(ip string) *proto.DeviceInfo {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, info := range t.maps {
		if info.IP == ip {
			return info
		}
	}
	return nil
}

// A return value of nil indicates not found
func (t *HostsManager) FromUdid(udid string) *proto.DeviceInfo {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.maps[udid]
}

func (t *HostsManager) AddFromDeviceInfo(devInfo *proto.DeviceInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()
	udid := devInfo.Udid
	if info, ok := t.maps[udid]; ok {
		info.IP = devInfo.IP
		info.ConnectionCount++
	} else {
		devInfo.ConnectionCount = 1
		t.maps[udid] = devInfo
	}
}

func (t *HostsManager) Remove(udid string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if info, ok := t.maps[udid]; ok {
		info.ConnectionCount--
		if info.ConnectionCount <= 0 {
			delete(t.maps, udid)
		}
	}
}
