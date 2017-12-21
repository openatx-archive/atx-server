package main

import (
	"log"
	"strings"

	"github.com/openatx/atx-server/proto"

	r "gopkg.in/gorethink/gorethink.v3"
)

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

func (db *RdbUtils) UpdateOrInsertDevice(dev proto.DeviceInfo) error {
	return r.Table("devices").Insert(dev, r.InsertOpts{
		Conflict: func(id, oldDoc, newDoc r.Term) interface{} {
			return oldDoc.Merge(newDoc)
		},
	}).Exec(db.session)
}

func (db *RdbUtils) DeviceList() (devices []proto.DeviceInfo) {
	res, err := r.Table("devices").Run(db.session)
	if err != nil {
		return nil
	}
	defer res.Close()
	res.All(&devices)
	return
}

var db *RdbUtils

func init() {
	r.SetTags("gorethink", "json")
	r.SetVerbose(true)
	session, err := r.Connect(r.ConnectOpts{
		Address:    "localhost:28015",
		Database:   "atxserver",
		InitialCap: 10,
		MaxOpen:    10,
	})

	if err != nil {
		log.Fatal(err)
	}
	db = &RdbUtils{session}
}

func main() {
	log.Println("main")
	if err := db.DBCreateAnyway("atxserver"); err != nil {
		log.Fatal(err)
	}
	if err := db.TableCreateAnyway("devices"); err != nil {
		log.Fatal(err)
	}
	log.Println("table created")
	db.UpdateOrInsertDevice(proto.DeviceInfo{
		Udid:   "aaaabbbbccccdddd1234",
		Serial: "abcd123456",
		// Brand: "Huawei",
	})

	log.Println(db.DeviceList())

	feeds, err := r.Table("devices").Changes().Run(db.session)
	if err != nil {
		log.Fatal(err)
	}
	defer feeds.Close()
	var change r.ChangeResponse
	for feeds.Next(&change) {
		// var devInfo proto.DeviceInfo
		// log.Println(devInfo)
		// log.Println(change.State)
		log.Println(change.NewValue)
		log.Println(change.OldValue)
	}
}
