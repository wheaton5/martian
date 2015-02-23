//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// Kepler database manager.
//

package main

import (
	"database/sql"
	"fmt"
	"martian/core"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type DatabaseManager struct {
	name string
	url  string
	conn *sql.DB
}

type DatabaseTx struct {
	tx      *sql.Tx
	psid    int64
	forkIds map[string]int64
}

func NewDatabaseManager(name string, url string) *DatabaseManager {
	self := &DatabaseManager{}
	self.name = name
	self.url = url

	verifyDatabaseDriver(name)
	return self
}

func verifyDatabaseDriver(name string) {
	validNames := []string{"sqlite3"}
	for _, validName := range validNames {
		if name == validName {
			return
		}
	}
	core.LogInfo("keplerd", "Invalid database driver: %s. Valid database drivers: %s", name, strings.Join(validNames, ", "))
	os.Exit(1)
}

func (self *DatabaseManager) NewTransaction() (*DatabaseTx, error) {
	tx, err := self.conn.Begin()
	if err != nil {
		return nil, err
	}
	return &DatabaseTx{
		tx:      tx,
		forkIds: map[string]int64{},
	}, nil
}

func (self *DatabaseManager) Open() error {
	conn, err := sql.Open(self.name, self.url)
	if err != nil {
		return err
	}
	self.conn = conn
	return self.conn.Ping()
}

func (self *DatabaseManager) Close() error {
	return self.conn.Close()
}

func (self *DatabaseManager) Query(statement string) (map[string]interface{}, error) {
	rows, err := self.conn.Query(statement)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	vals := make([]interface{}, len(cols))
	for i, _ := range cols {
		vals[i] = new(sql.RawBytes)
	}

	rowLists := [][]string{}
	for rows.Next() {
		rows.Scan(vals...)
		rowList := []string{}
		for _, val := range vals {
			rowList = append(rowList, fmt.Sprintf("%v", val))
		}
		rowLists = append(rowLists, rowList)
	}
	return map[string]interface{}{
		"columns": cols,
		"rows":    rowLists,
	}, nil
}

func (self *DatabaseManager) CreateTables() {
	self.createTable("stats", []string{
		"id integer not null primary key",
		"core_hours float",
		"duration float",
		"walltime float",
		"systemtime float",
		"usertime float",
		"start time",
		"end time",
		"num_jobs int",
		"num_threads int",
		"maxrss int",
		"in_blocks int",
		"out_blocks int",
		"total_blocks int",
		"in_blocks_rate float",
		"out_blocks_rate float",
		"total_blocks_rate float",
		"total_files int",
		"total_bytes int",
		"vdr_bytes int",
		"vdr_files int",
		"output_bytes int",
		"output_files int",
	})
	self.createTable("pipestances", []string{
		"id integer not null primary key",
		"fqname string",
		"call string",
		"path string",
		"martian_version string",
		"pipelines_version string",
	})
	self.createTable("arguments", []string{
		"id integer not null primary key",
		"psid integer",
		"key string",
		"value string",
		"foreign key(psid) references pipestances(id)",
	})
	self.createTable("forks", []string{
		"id integer not null primary key",
		"psid integer",
		"sid integer",
		"fqname string",
		"type string",
		"foreign key(psid) references pipestances(id)",
		"foreign key(sid) references stats(id)",
	})
	self.createTable("pipeline_stages", []string{
		"id integer not null primary key",
		"pid integer",
		"sid integer",
		"foreign key(pid) references forks(id)",
		"foreign key(sid) references forks(id)",
	})
	self.createTable("chunks", []string{
		"id integer not null primary key",
		"parent integer",
		"psid integer",
		"sid integer",
		"fqname string",
		"foreign key(parent) references forks(id)",
		"foreign key(psid) references pipestances(id)",
		"foreign key(sid) references stats(id)",
	})
}

func (self *DatabaseManager) InsertPipestance(tx *DatabaseTx, path string, fqname string, martianVersion string,
	pipelinesVersion string, call string, args map[string]interface{}) {
	tx.psid, _ = self.insert("pipestances", map[string]interface{}{
		"fqname":            fqname,
		"call":              call,
		"path":              path,
		"martian_version":   martianVersion,
		"pipelines_version": pipelinesVersion,
	})
	for key, value := range args {
		self.insert("arguments", map[string]interface{}{
			"psid":  tx.psid,
			"key":   key,
			"value": value,
		})
	}
}

func (self *DatabaseManager) insert(tableName string, row map[string]interface{}) (int64, error) {
	keys := []string{}
	values := []string{}
	for key, value := range row {
		keys = append(keys, key)
		values = append(values, fmt.Sprintf("%v", value))
	}
	cmd := fmt.Sprintf("insert into %s(%s) values(%s)", tableName, strings.Join(keys, ", "), strings.Join(values, ", "))
	res, err := self.conn.Exec(cmd)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (self *DatabaseManager) createTable(name string, columns []string) error {
	cmd := fmt.Sprintf("create table %s (%s)", name, strings.Join(columns, ", "))
	_, err := self.conn.Exec(cmd)
	return err
}
