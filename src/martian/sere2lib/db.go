// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package sere2lib

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"log"
	"reflect"
	"strings"
)

type CoreConnection struct {
	Conn *sql.DB
}

func Setup() *CoreConnection {
	conn := new(CoreConnection)

	db, err := sql.Open("postgres", "postgres://x10user:v3rys3cr3t@52.39.198.116/sere2?sslmode=disable")
	if err != nil {
		panic(err)
	}

	conn.Conn = db
	return conn

}

func (c *CoreConnection) InsertRecord(table string, r interface{}) error {

	keys := make([]string, 0)
	interpolator := make([]string, 0)
	values := make([]interface{}, 0)

	val := reflect.ValueOf(r)
	t := val.Type()
	for i := 0; i < val.NumField(); i++ {
		sf := t.Field(i)
		keys = append(keys, sf.Name)
		values = append(values, val.Field(i).Interface())
		interpolator = append(interpolator, fmt.Sprintf("$%v", i+1))
	}

	query := "INSERT INTO " + table + " (" + strings.Join(keys, ",") + ") VALUES (" + strings.Join(interpolator, ",") + ")"

	log.Printf("Q: %v", query)
	log.Print("V: %v", values)

	_, err := c.Conn.Query(query, values...)

	log.Printf("E: %v", err)
	return err
}

func (c *CoreConnection) Dump() {

	res, err := c.Conn.Query("select * from test_reports;")

	log.Printf("UHOH: %v %v", err, res)

}
