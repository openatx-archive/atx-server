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
