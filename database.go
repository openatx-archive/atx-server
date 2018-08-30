package main

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/openatx/atx-server/proto"
	"github.com/qiniu/log"
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
	db.TableMustCreate("devices", r.TableCreateOpts{
		PrimaryKey: "udid",
	})
	db.TableMustCreate("products")
	db.TableMustCreate("providers")

	r.Table("devices").Update(map[string]interface{}{
		"present":     false,
		"using":       false,
		"provider_id": 0,
	}).Exec(session)

	if err := r.Table("devices").IndexCreate("provider_id", r.IndexCreateOpts{}).Exec(session); err != nil {
		log.Println("create index", err)
	}

	r.Table("providers").Update(proto.Provider{
		Present: newBool(false),
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

func (db *RdbUtils) TableMustCreate(name string, optArgs ...r.TableCreateOpts) {
	if err := db.TableCreateAnyway(name, optArgs...); err != nil {
		panic(err)
	}
}

func (db *RdbUtils) TableCreateAnyway(name string, optArgs ...r.TableCreateOpts) error {
	err := r.TableCreate(name, optArgs...).Exec(db.session)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}

// DeviceUpdateOrInsert called when device plugin
func (db *RdbUtils) DeviceUpdateOrInsert(dev proto.DeviceInfo) error {
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

func (db *RdbUtils) DeviceUpdate(udid string, arg interface{}) error {
	_, err := r.Table("devices").Get(udid).Update(arg).RunWrite(db.session)
	return err
}

func (db *RdbUtils) DeviceList() (devices []proto.DeviceInfo, err error) {
	res, err := r.Table("devices").
		OrderBy(r.Desc("present"), r.Desc("ready"), r.Desc("using"), r.Desc("presenceChangedAt")).
		Merge(func(p r.Term) interface{} {
			return map[string]interface{}{
				"product_id":  r.Table("products").Get(p.Field("product_id").Default(0)),
				"provider_id": r.Table("providers").Get(p.Field("provider_id").Default(0)),
			}
		}).Run(db.session)
	if err != nil {
		log.Error(err)
		return
	}
	defer res.Close()
	err = res.All(&devices)
	return
}

func (db *RdbUtils) DeviceGet(udid string) (info proto.DeviceInfo, err error) {
	res, err := r.Table("devices").Get(udid).
		Merge(func(p r.Term) interface{} {
			return map[string]interface{}{
				"product_id":  r.Table("products").Get(p.Field("product_id").Default(0)),
				"provider_id": r.Table("providers").Get(p.Field("provider_id").Default(0)),
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
				"product_id":  r.Table("products").Get(p.Field("product_id").Default(0)),
				"provider_id": r.Table("providers").Get(p.Field("provider_id").Default(0)),
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

// ProviderFindAll get all providers
func (db *RdbUtils) ProvidersAll() (providers []proto.Provider, err error) {
	res, err := r.Table("providers").OrderBy(r.Desc("present"), "id").
		Merge(func(p r.Term) interface{} {
			return map[string]interface{}{
				"devices": r.Table("devices").
					GetAllByIndex("provider_id", p.Field("id")).
					Without("product_id", "provider_id", "battery").CoerceTo("array"),
			}
		}).Run(db.session)
	if err != nil {
		return nil, err
	}
	defer res.Close()
	err = res.All(&providers)
	return
}

// SetDevicePresent change present status
func (db *RdbUtils) SetDeviceAbsent(udid string) error {
	log.Debugf("device absent: %s", udid)
	return db.DeviceUpdate(udid, proto.DeviceInfo{
		Present:           newBool(false),
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

// ProviderUpdateOrInsert will create a record if not exists
func (db *RdbUtils) ProviderUpdateOrInsert(machineId string, ip string, port int) error {
	p := proto.Provider{
		Id:                machineId,
		IP:                ip,
		Port:              port,
		Present:           newBool(true),
		CreatedAt:         time.Now(),
		PresenceChangedAt: time.Now(),
	}
	_, err := r.Table("providers").Insert(p, r.InsertOpts{
		Conflict: func(id, oldDoc, newDoc r.Term) interface{} {
			return oldDoc.Merge(newDoc.Without("createdAt")).Merge(map[string]interface{}{
				"createdAt": oldDoc.Field("createdAt").Default(time.Now()),
			})
		},
	}).RunWrite(db.session)
	return err
}

func (db *RdbUtils) ProviderUpdate(id string, provider proto.Provider) error {
	provider.Id = id
	_, err := r.Table("providers").Get(id).Update(provider).RunWrite(db.session)
	return err
}

func (db *RdbUtils) ProviderOffline(id string) error {
	_, err := r.Table("providers").Get(id).Update(proto.Provider{
		Present: newBool(false),
	}).RunWrite(db.session)
	if err != nil {
		return err
	}
	_, err = r.Table("devices").Filter(r.Row.Field("provider_id").Eq(id)).Update(map[string]interface{}{
		"provider_id": 0,
	}).RunWrite(db.session)
	return err
}

func (db *RdbUtils) ProviderGet(id string) (provider proto.Provider, err error) {
	res, err := r.Table("providers").Get(id).Run(db.session)
	if err != nil {
		return
	}
	defer res.Close()
	err = res.One(&provider)
	return
}
