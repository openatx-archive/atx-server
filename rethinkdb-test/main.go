package main

import (
	"fmt"
	"log"

	r "gopkg.in/gorethink/gorethink.v3"
)

func DBCreateIfNotExists(name string, session *r.Session) error {
	res, err := r.DBList().Run(session)
	if err != nil {
		return err
	}
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
	_, err = r.DBCreate("atxserver").Run(session)
	return err
}

func main() {
	session, err := r.Connect(r.ConnectOpts{
		Address: "localhost:28015",
	})
	if err != nil {
		log.Fatal(err)
	}
	res, err := r.Expr("Hello world").Run(session)
	if err != nil {
		log.Fatal(err)
	}
	var response string
	err = res.One(&response)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(response)

	if err := DBCreateIfNotExists("atxserver", session); err != nil {
		log.Fatal(err)
	}
}
