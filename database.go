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
		Address:    address,
		Database:   dbName,
		InitialCap: 1,
		MaxOpen:    10,
	})

	if err != nil {
		log.Fatal(err)
	}
	db = &RdbUtils{session}

	// initial state
	if err := db.DBCreateAnyway(dbName); err != nil {
		panic(err)
	}
	log.Println("create tables")
	if err := db.TableCreateAnyway("devices", r.TableCreateOpts{
		PrimaryKey: "udid",
	}); err != nil {
		panic(err)
	}
	if err := db.TableCreateAnyway("products"); err != nil {
		panic(err)
	}

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
		log.Println("found db:", dbName)
		if dbName == name {
			log.Println("db exists atxserver")
			return nil
		}
	}
	err = r.DBCreate(name).Exec(db.session)
	return err
}

func (db *RdbUtils) TableCreateAnyway(name string, optArgs ...r.TableCreateOpts) error {
	err := r.TableCreate(name, optArgs...).Exec(db.session)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}

// UpdateOrInsertDevice called when device plugin
func (db *RdbUtils) UpdateOrInsertDevice(dev proto.DeviceInfo) error {
	dev.Present = newBool(true)
	dev.PresenceChangedAt = time.Now()
	// only update when create
	dev.Ready = newBool(false)
	dev.Using = newBool(false)
	dev.CreatedAt = time.Now()
	_, err := r.Table("devices").Insert(dev, r.InsertOpts{
		Conflict: func(id, oldDoc, newDoc r.Term) interface{} {
			return oldDoc.Merge(newDoc.Without("createdAt", "ready", "using")).Merge(map[string]interface{}{
				"createdAt": oldDoc.Field("createdAt").Default(time.Now()),
				"ready":     oldDoc.Field("ready").Default(false),
				"using":     oldDoc.Field("using").Default(false),
			})
		},
	}).RunWrite(db.session)
	return err
}

func (db *RdbUtils) DeviceUpdate(dev proto.DeviceInfo) error {
	if dev.Udid == "" {
		return errors.New("DeviceInfo require udid field")
	}
	_, err := r.Table("devices").Get(dev.Udid).Update(dev).RunWrite(db.session)
	return err
}

func (db *RdbUtils) DeviceList() (devices []proto.DeviceInfo) {
	res, err := r.Table("devices").
		OrderBy(r.Desc("present"), r.Desc("ready"), r.Desc("using"), r.Desc("presenceChangedAt")).
		Merge(func(p r.Term) interface{} {
			return map[string]interface{}{
				"product_id": r.Table("products").Get(p.Field("product_id").Default(0)),
			}
		}).Run(db.session)
	if err != nil {
		log.Error(err)
		return nil
	}
	defer res.Close()
	res.All(&devices)
	return
}

func (db *RdbUtils) DeviceGet(udid string) (info proto.DeviceInfo, err error) {
	res, err := r.Table("devices").Get(udid).
		Merge(func(p r.Term) interface{} {
			return map[string]interface{}{
				"product_id": r.Table("products").Get(p.Field("product_id").Default(0)),
			}
		}).Run(db.session)
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
	res, err := r.Table("devices").Filter(info).
		Merge(func(p r.Term) interface{} {
			return map[string]interface{}{
				"product_id": r.Table("products").Get(p.Field("product_id").Default(0)),
			}
		}).Run(db.session)
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
	log.Debugf("device absent: %s", udid)
	return db.DeviceUpdate(proto.DeviceInfo{
		Udid:              udid,
		Present:           newBool(false), // &present,
		PresenceChangedAt: time.Now(),
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

func (db *RdbUtils) ProductsFindAll(brand, model string) (products []proto.Product, err error) {
	res, err := r.Table("products").Filter(proto.Product{Brand: brand, Model: model}).Run(db.session)
	if err != nil {
		return
	}
	if err = res.All(&products); err != nil {
		return
	}
	if len(products) > 0 {
		return
	}
	resp, err := r.Table("products").Insert(proto.Product{Brand: brand, Model: model}).RunWrite(db.session)
	if err != nil {
		return
	}
	if len(resp.GeneratedKeys) != 1 {
		panic("generatedKeys must be one")
	}
	return db.ProductsFindAll(brand, model)
}

func (db *RdbUtils) ProductUpdate(id string, product proto.Product) error {
	product.Id = ""
	_, err := r.Table("products").Get(id).Update(product).RunWrite(db.session)
	return err
}
