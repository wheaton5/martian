//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// Kepler database manager.
//

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"martian/core"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type DatabaseManager struct {
	name   string
	url    string
	conn   *sql.DB
	roUrl  string
	roConn *sql.DB
}

type DatabaseTx struct {
	psid  int64
	table map[string]int64
}

type DatabaseError struct {
	Fqname string
}

func (self *DatabaseError) Error() string {
	return fmt.Sprintf("Fqname %s does not exist in current transaction", self.Fqname)
}

func makeForkFqname(fqname string, forki int) string {
	return fmt.Sprintf("%s.fork%d", fqname, forki)
}

func findForkId(tx *DatabaseTx, forkFqname string, forkIndex int) (int64, error) {
	fqname := makeForkFqname(forkFqname, forkIndex)
	id, ok := tx.table[fqname]
	if !ok {
		return 0, &DatabaseError{fqname}
	}
	return id, nil
}

func openConn(name string, url string) (*sql.DB, error) {
	conn, err := sql.Open(name, url)
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(); err != nil {
		return nil, err
	}
	return conn, nil
}

func NewDatabaseManager(name string, url string) *DatabaseManager {
	self := &DatabaseManager{}
	self.name = name
	self.url = url
	self.roUrl = url

	// Driver-specific initialization
	self.initDriver()

	// Open connection to database
	if err := self.open(); err != nil {
		core.LogInfo("keplerd", "Unable to establish connection with database %s: %s", self.url, err.Error())
		os.Exit(1)
	}

	// Create tables (if they don't already exist)
	self.createTables()

	return self
}

func (self *DatabaseManager) initDriver() {
	validNames := []string{"sqlite3"}

	switch self.name {
	case "sqlite3":
		self.roUrl = fmt.Sprintf("file:%s?mode=ro", self.roUrl)
	default:
		core.LogInfo("keplerd", "Invalid database driver: %s. Valid database drivers: %s", self.name, strings.Join(validNames, ", "))
		os.Exit(1)
	}
}

func NewDatabaseTx() *DatabaseTx {
	return &DatabaseTx{
		table: map[string]int64{},
	}
}

func (self *DatabaseTx) Begin() {
	core.EnterCriticalSection()
}

func (self *DatabaseTx) End() {
	core.ExitCriticalSection()
}

func (self *DatabaseManager) open() error {
	conn, err := openConn(self.name, self.url)
	if err != nil {
		return err
	}
	roConn, err := openConn(self.name, self.roUrl)
	if err != nil {
		return err
	}

	self.conn = conn
	self.roConn = roConn
	return nil
}

func (self *DatabaseManager) Close() {
	self.conn.Close()
	self.roConn.Close()
}

func (self *DatabaseManager) GetPipestances() ([]map[string]string, error) {
	res, err := self.Query("select fqname, path, pipelines_version from pipestances")
	if err != nil {
		return nil, err
	}

	pipestances := []map[string]string{}
	rows := res["rows"].([][]string)
	for _, row := range rows {
		pipestances = append(pipestances, map[string]string{
			"fqname":  row[0],
			"path":    row[1],
			"version": row[2],
		})
	}
	return pipestances, nil
}

func (self *DatabaseManager) Query(statement string) (map[string]interface{}, error) {
	rows, err := self.roConn.Query(statement)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	vals := make([][]byte, len(cols))
	dest := make([]interface{}, len(cols))
	for i, _ := range cols {
		dest[i] = &vals[i]
	}
	rowLists := [][]string{}
	for rows.Next() {
		rows.Scan(dest...)
		rowList := []string{}
		for _, val := range vals {
			rowList = append(rowList, string(val))
		}
		rowLists = append(rowLists, rowList)
	}
	return map[string]interface{}{
		"columns": cols,
		"rows":    rowLists,
	}, nil
}

func (self *DatabaseManager) createTables() {
	core.EnterCriticalSection()
	defer core.ExitCriticalSection()

	self.createTable("stats", []string{
		"id integer not null primary key",
		"core_hours float",
		"duration float",
		"walltime float",
		"systemtime float",
		"usertime float",
		"start string",
		"end string",
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
		"stats_id integer",
		"fqname string",
		"call string",
		"path string",
		"martian_version string",
		"pipelines_version string",
		"foreign key(stats_id) references stats(id)",
	})
	self.createTable("tags", []string{
		"id integer not null primary key",
		"psid integer",
		"key string",
		"value string",
		"foreign key(psid) references pipestances(id)",
	})
	self.createTable("forks", []string{
		"id integer not null primary key",
		"psid integer",
		"stats_id integer",
		"name string",
		"fqname string",
		"type string",
		"forki integer",
		"foreign key(psid) references pipestances(id)",
		"foreign key(stats_id) references stats(id)",
	})
	self.createTable("relationships", []string{
		"id integer not null primary key",
		"pipeline_id integer",
		"stage_id integer",
		"foreign key(pipeline_id) references forks(id)",
		"foreign key(stage_id) references forks(id)",
	})
	self.createTable("jobs", []string{
		"id integer not null primary key",
		"psid integer",
		"fork_id integer",
		"stats_id integer",
		"type string",
		"chunki integer",
		"foreign key(psid) references pipestances(id)",
		"foreign key(fork_id) references forks(id)",
		"foreign key(stats_id) references stats(id)",
	})
}

func (self *DatabaseManager) InsertPipestance(tx *DatabaseTx, path string, fqname string, martianVersion string,
	pipelinesVersion string, stats *core.PerfInfo, call string, args map[string]interface{},
	tags []string) error {

	statsId, err := self.insertStats(stats)
	if err != nil {
		return err
	}

	psid, err := self.insert("pipestances", map[string]interface{}{
		"stats_id":          statsId,
		"fqname":            fqname,
		"call":              call,
		"path":              path,
		"martian_version":   martianVersion,
		"pipelines_version": pipelinesVersion,
	})
	if err != nil {
		return err
	}

	for key, value := range args {
		_, err := self.insert("tags", map[string]interface{}{
			"psid":  psid,
			"key":   key,
			"value": value,
		})

		if err != nil {
			return err
		}
	}
	for _, tag := range tags {
		key, value := core.ParseTag(tag)
		_, err := self.insert("tags", map[string]interface{}{
			"psid":  psid,
			"key":   key,
			"value": value,
		})

		if err != nil {
			return err
		}
	}

	tx.psid = psid
	return nil
}

func (self *DatabaseManager) InsertChunk(tx *DatabaseTx, fqname string, forki int, stats *core.PerfInfo, chunki int) error {
	return self.insertJob(tx, fqname, forki, stats, "chunk", chunki)
}

func (self *DatabaseManager) InsertSplit(tx *DatabaseTx, fqname string, forki int, stats *core.PerfInfo) error {
	return self.insertJob(tx, fqname, forki, stats, "split", 0)
}

func (self *DatabaseManager) InsertJoin(tx *DatabaseTx, fqname string, forki int, stats *core.PerfInfo) error {
	return self.insertJob(tx, fqname, forki, stats, "join", 0)
}

func (self *DatabaseManager) insertJob(tx *DatabaseTx, fqname string, forki int, stats *core.PerfInfo,
	jobType string, jobi int) error {
	// Find fork in transaction ID table
	forkId, err := findForkId(tx, fqname, forki)
	if err != nil {
		return err
	}

	// Insert stats
	statsId, err := self.insertStats(stats)
	if err != nil {
		return err
	}

	// Insert job
	_, err = self.insert("jobs", map[string]interface{}{
		"psid":     tx.psid,
		"stats_id": statsId,
		"fork_id":  forkId,
		"type":     jobType,
		"chunki":   jobi,
	})
	return err
}

func (self *DatabaseManager) InsertRelationship(tx *DatabaseTx, pipelineFqname string, pipelineIndex int, stageFqname string, stageIndex int) error {
	// Find pipeline in transaction ID table
	pipelineId, err := findForkId(tx, pipelineFqname, pipelineIndex)
	if err != nil {
		return err
	}

	// Find stage in transaction ID table
	stageId, err := findForkId(tx, stageFqname, stageIndex)
	if err != nil {
		return err
	}

	// Insert pipeline-stage relationship
	_, err = self.insert("relationships", map[string]interface{}{
		"pipeline_id": pipelineId,
		"stage_id":    stageId,
	})
	return err
}

func (self *DatabaseManager) InsertFork(tx *DatabaseTx, name string, fqname string, nodeType string,
	forki int, stats *core.PerfInfo) error {
	// Insert stats
	statsId, err := self.insertStats(stats)
	if err != nil {
		return err
	}

	// Insert fork
	forkId, err := self.insert("forks", map[string]interface{}{
		"psid":     tx.psid,
		"stats_id": statsId,
		"name":     name,
		"fqname":   fqname,
		"type":     nodeType,
		"forki":    forki,
	})
	if err != nil {
		return err
	}

	// Insert fork in transaction ID table
	forkFqname := makeForkFqname(fqname, forki)
	tx.table[forkFqname] = forkId
	return nil
}

func (self *DatabaseManager) insertStats(stats *core.PerfInfo) (int64, error) {
	return self.insert("stats", map[string]interface{}{
		"num_jobs":          stats.NumJobs,
		"num_threads":       stats.NumThreads,
		"duration":          stats.Duration,
		"core_hours":        stats.CoreHours,
		"maxrss":            stats.MaxRss,
		"in_blocks":         stats.InBlocks,
		"out_blocks":        stats.OutBlocks,
		"total_blocks":      stats.TotalBlocks,
		"in_blocks_rate":    stats.InBlocksRate,
		"out_blocks_rate":   stats.OutBlocksRate,
		"total_blocks_rate": stats.TotalBlocksRate,
		"start":             stats.Start,
		"end":               stats.End,
		"walltime":          stats.WallTime,
		"usertime":          stats.UserTime,
		"systemtime":        stats.SystemTime,
		"total_files":       stats.TotalFiles,
		"total_bytes":       stats.TotalBytes,
		"output_files":      stats.OutputFiles,
		"output_bytes":      stats.OutputBytes,
		"vdr_files":         stats.VdrFiles,
		"vdr_bytes":         stats.VdrBytes,
	})
}

func (self *DatabaseManager) insert(tableName string, row map[string]interface{}) (int64, error) {
	keys := []string{}
	values := []string{}
	for key, value := range row {
		var toJson bool
		var newValue string

		// Convert value to JSON if it is a map or list
		// Otherwise, use its default string representation
		switch value.(type) {
		case map[string]interface{}:
			toJson = true
		case []interface{}:
			toJson = true
		default:
			toJson = false
		}

		if toJson {
			bytes, err := json.Marshal(value)
			if err != nil {
				return 0, err
			}
			newValue = string(bytes)
		} else {
			newValue = fmt.Sprintf("%v", value)
		}

		keys = append(keys, key)
		values = append(values, fmt.Sprintf("'%s'", newValue))
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
