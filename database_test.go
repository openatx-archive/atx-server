package main

import (
	"testing"

	r "gopkg.in/gorethink/gorethink.v4"
)

func TestInsertOrUpdateDevice(t *testing.T) {
	mock := r.NewMock()
	// mock.On(r.Table("devices")).
	_ = mock
}

// func TestTableProduct(t *testing.T) {

// 	device, err := db.DeviceGet("6EB0217607005249-c4:86:e9:53:c2:e4-DUK-AL20")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Logf("%#v", device)
// }
