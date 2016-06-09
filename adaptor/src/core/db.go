// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package core

import (
	"database/sql"
	_ "github.com/lib/pq"
	"log"
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

func RecordToInsert(r interface{}) string {

	return ""
}

func (c *CoreConnection) CommitRecord(r *ReportRecord) {

}

func (c *CoreConnection) Dump() {

	res, err := c.Conn.Query("select * from test_reports;")

	log.Printf("UHOH: %v %v", err, res)

}
