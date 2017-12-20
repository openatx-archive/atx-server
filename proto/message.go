package proto

import (
	"encoding/json"
	"time"

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
	Udid         string                `json:"udid,omitempty"`   // Unique device identifier
	Serial       string                `json:"serial,omitempty"` // ro.serialno
	Brand        string                `json:"brand,omitempty"`  // ro.product.brand
	Model        string                `json:"model,omitempty"`  // ro.product.model
	HWAddr       string                `json:"hwaddr,omitempty"` // persist.sys.wifi.mac
	IP           string                `json:"ip,omitempty"`
	AgentVersion string                `json:"agentVersion,omitempty"`
	Display      *androidutils.Display `json:"display,omitempty"`
	Battery      *androidutils.Battery `json:"battery,omitempty"`

	ConnectionCount   int       `json:"-"` // > 1 happended when phone redial server
	Reserved          string    `json:"reserved,omitempty"`
	CreatedAt         time.Time `json:"-" gorethink:"createdAt,omitempty"`
	PresenceChangedAt time.Time `json:"presenceChangedAt,omitempty"`

	Ready   *bool `json:"ready,omitempty"`
	Present *bool `json:"present,omitempty"`
	Using   *bool `json:"using,omitempty"`
}
