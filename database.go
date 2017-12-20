package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/openatx/atx-server/proto"
	log "github.com/sirupsen/logrus"
	r "gopkg.in/gorethink/gorethink.v4"
)

var (
	db *RdbUtils
)

func initDB(address, dbName string) {
	r.SetTags("gorethink", "json")
	r.SetVerbose(true)
	session, err := r.Connect(r.ConnectOpts{
		Address:  address,
		Database: dbName,
		// InitialCap: 10,
		// MaxOpen:    10,
	})

	if err != nil {
		log.Fatal(err)
	}
	db = &RdbUtils{session}

	// initial state
	db.DBCreateAnyway(dbName)
	db.TableCreateAnyway("devices")
	r.Table("devices").Update(map[string]bool{
		"present": false,
	}).Exec(session)
}

type RdbUtils struct {
	session *r.Session
}

func (db *RdbUtils) DBCreateAnyway(name string) error {
	res, err := r.DBList().Run(db.session)
	if err != nil {
		return err
	}
	defer res.Close()
	var dbNames []string
	if err := res.All(&dbNames); err != nil {
		return err
	}
	for _, dbName := range dbNames {
		log.Println(dbName)
		if dbName == name {
			log.Println("db exists atxserver")
			return nil
		}
	}
	err = r.DBCreate("atxserver").Exec(db.session)
	return err
}

func (db *RdbUtils) TableCreateAnyway(name string) error {
	err := r.TableCreate(name, r.TableCreateOpts{
		PrimaryKey: "udid",
	}).Exec(db.session)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}

// UpdateOrInsertDevice called when device plugin
func (db *RdbUtils) UpdateOrInsertDevice(dev proto.DeviceInfo) error {
	dev.Present = newBool(true)
	dev.CreatedAt = time.Now()
	dev.PresenceChangedAt = time.Now()
	return r.Table("devices").Insert(dev, r.InsertOpts{
		Conflict: func(id, oldDoc, newDoc r.Term) interface{} {
			return oldDoc.Merge(newDoc.Without("createdAt"))
		},
	}).Exec(db.session)
}

func (db *RdbUtils) DeviceUpdate(dev proto.DeviceInfo) error {
	if dev.Udid == "" {
		return errors.New("DeviceInfo require udid field")
	}
	return r.Table("devices").Get(dev.Udid).Update(dev).Exec(db.session)
}

func (db *RdbUtils) DeviceList() (devices []proto.DeviceInfo) {
	res, err := r.Table("devices").OrderBy(r.Desc("present"), r.Desc("presenceChangedAt"), r.Desc("ready"), r.Desc("using")).Run(db.session)
	if err != nil {
		log.Error(err)
		return nil
	}
	defer res.Close()
	res.All(&devices)
	return
}

func (db *RdbUtils) DeviceGet(udid string) (info proto.DeviceInfo, err error) {
	res, err := r.Table("devices").Get(udid).Run(db.session)
	if err != nil {
		return
	}
	defer res.Close()
	err = res.One(&info)
	return
}

func (db *RdbUtils) DeviceFindAll(info proto.DeviceInfo) (infos []proto.DeviceInfo) {
	infojson, _ := json.Marshal(info)
	log.Debugf("query %s", string(infojson))
	res, err := r.Table("devices").Filter(info).Run(db.session)
	if err != nil {
		log.Error(err)
		return nil
	}
	defer res.Close()
	if err := res.All(&infos); err != nil {
		log.Error(err)
	}
	return
}

// SetDevicePresent change present status
func (db *RdbUtils) SetDeviceAbsent(udid string) error {
	return db.UpdateOrInsertDevice(proto.DeviceInfo{
		Udid:              udid,
		Present:           newBool(false), // &present,
		PresenceChangedAt: time.Now(),
		Ready:             newBool(false),
	})
}

func (db *RdbUtils) WatchDeviceChanges() (feeds chan r.ChangeResponse, cancel func(), err error) {
	ctx, cancel := context.WithCancel(context.Background())
	res, err := r.Table("devices").Changes().Run(db.session, r.RunOpts{
		Context: ctx,
	})
	if err != nil {
		return
	}
	feeds = make(chan r.ChangeResponse)
	var change r.ChangeResponse
	go func() {
		for res.Next(&change) {
			feeds <- change
		}
		close(feeds)
	}()
	return
}
