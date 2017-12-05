package proto

import (
	"encoding/json"

	"github.com/openatx/androidutils"
)

type MessageType int

const (
	DeviceInfoMessage = MessageType(0)
	PingMessage       = MessageType(1)
)

type CommonMessage struct {
	Type MessageType
	Data interface{}
}

func (m *CommonMessage) MarshalJSON() []byte {
	data, _ := json.Marshal(m)
	return data
}

type DeviceInfo struct {
	Udid         string               `json:"udid"`   // Unique device identifier
	Serial       string               `json:"serial"` // ro.serialno
	Brand        string               `json:"brand"`  // ro.product.brand
	Model        string               `json:"model"`  // ro.product.model
	HWAddr       string               `json:"hwaddr"` // persist.sys.wifi.mac
	IP           string               `json:"ip,omitempty"`
	AgentVersion string               `json:"agentVersion"`
	Battery      androidutils.Battery `json:"battery"`
	Display      androidutils.Display `json:"display"`

	ConnectionCount int    `json:"-"` // > 1 happended when phone redial server
	Reserved        string `json:"reserved,omitempty"`
	Ready           bool   `json:"ready,omitempty"`
}
