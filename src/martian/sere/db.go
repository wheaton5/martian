//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"martian/core"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DatabaseManager struct {
	name string
	url  string
	conn *sql.DB
}

type DatabaseTx struct {
	conn *sql.DB
	tx   *sql.Tx
}

type DatabaseResult struct {
	Rows    [][]string `json:"rows"`
	Columns []string   `json:"columns"`
}

type Program struct {
	Name    string   `json:"name"`
	Battery *Battery `json:"battery"`
	Cycles  []*Cycle `json:"cycles"`
}

type Battery struct {
	Name  string  `json:"name"`
	Tests []*Test `json:"tests"`
}

type Cycle struct {
	Id        int      `json:"id"`
	Name      string   `json:"name"`
	StartDate string   `json:"start_date"`
	EndDate   string   `json:"end_date"`
	Rounds    []*Round `json:"rounds"`
}

type Round struct {
	Id             int     `json:"id"`
	PackageName    string  `json:"package_name"`
	PackageTarget  string  `json:"package_target"`
	PackageVersion string  `json:"package_version"`
	StartDate      string  `json:"start_date"`
	EndDate        string  `json:"end_date"`
	Tests          []*Test `json:"tests"`
}

type Test struct {
	Name      string             `json:"name"`
	Category  string             `json:"category"`
	Id        string             `json:"id"`
	Container string             `json:"container"`
	Pipeline  string             `json:"pipeline"`
	Psid      string             `json:"psid"`
	State     core.MetadataState `json:"state"`
}

const timeFormat = "2006-01-02 15:04"

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
		break
	default:
		core.LogInfo("keplerd", "Invalid database driver: %s. Valid database drivers: %s", self.name, strings.Join(validNames, ", "))
		os.Exit(1)
	}
}

func (self *DatabaseManager) NewDatabaseTx() *DatabaseTx {
	return &DatabaseTx{
		conn: self.conn,
	}
}

func (self *DatabaseTx) Begin() error {
	core.EnterCriticalSection()

	var err error
	self.tx, err = self.conn.Begin()
	return err
}

func (self *DatabaseTx) Rollback() error {
	err := self.tx.Rollback()
	core.ExitCriticalSection()
	return err
}

func (self *DatabaseTx) Commit() error {
	err := self.tx.Commit()
	core.ExitCriticalSection()
	return err
}

func (self *DatabaseManager) open() error {
	conn, err := openConn(self.name, self.url)
	if err != nil {
		return err
	}

	self.conn = conn
	return nil
}

func (self *DatabaseManager) Close() {
	self.conn.Close()
}

func (self *DatabaseManager) query(statement string) (*DatabaseResult, error) {
	rows, err := self.conn.Query(statement)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	vals := make([][]byte, len(cols))
	dest := make([]interface{}, len(cols))
	for i := range cols {
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
	return &DatabaseResult{
		Columns: cols,
		Rows:    rowLists,
	}, nil
}

func (self *DatabaseManager) createTables() {
	core.EnterCriticalSection()
	defer core.ExitCriticalSection()

	self.createTable("battery", []string{
		"battery_name text not null primary key",
	})
	self.createTable("program", []string{
		"program_name text not null primary key",
		"battery_name text",
		"foreign key(battery_name) references battery(battery_name)",
	})
	self.createTable("test", []string{
		"test_name text not null primary key",
		"test_category text",
		"test_id text",
	})
	self.createTable("battery_test", []string{
		"battery_name text",
		"test_name text",
		"primary key (battery_name, test_name)",
		"foreign key(battery_name) references battery(battery_name)",
		"foreign key(test_name) references test(test_name)",
	})
	self.createTable("cycle", []string{
		"program_name text",
		"cycle_id integer",
		"cycle_name text",
		"start_date string",
		"end_date string",
		"primary key (program_name, cycle_id)",
		"foreign key(program_name) references program(program_name)",
	})
	self.createTable("round", []string{
		"program_name text",
		"cycle_id integer",
		"round_id integer",
		"package_name text",
		"package_target text",
		"package_version text",
		"start_date string",
		"end_date string",
		"primary key (program_name, cycle_id, round_id)",
		"foreign key(program_name) references program(program_name)",
		"foreign key(cycle_id) references cycle(cycle_id)",
	})
}

func (self *DatabaseManager) ManageBatteries() ([]*Battery, error) {
	res, err := self.query("select battery_name, test_name from battery_test order by battery_name")
	if err != nil {
		return nil, err
	}

	batteries := []*Battery{}
	for _, row := range res.Rows {
		name := row[0]
		length := len(batteries)

		var battery *Battery
		if length == 0 || batteries[length-1].Name != name {
			battery = &Battery{
				Name:  name,
				Tests: []*Test{},
			}
			batteries = append(batteries, battery)
		} else {
			battery = batteries[length-1]
		}

		test := &Test{
			Name: row[1],
		}
		battery.Tests = append(battery.Tests, test)
	}
	return batteries, nil
}

func (self *DatabaseManager) ManageTests() ([]*Test, error) {
	res, err := self.query("select test_name, test_category, test_id from test")
	if err != nil {
		return nil, err
	}

	tests := []*Test{}
	for _, row := range res.Rows {
		test := &Test{
			Name:     row[0],
			Category: row[1],
			Id:       row[2],
		}
		tests = append(tests, test)
	}
	return tests, nil
}

func (self *DatabaseManager) ManagePrograms() ([]*Program, error) {
	res, err := self.query("select program_name, battery_name from program")
	if err != nil {
		return nil, err
	}

	programs := []*Program{}
	for _, row := range res.Rows {
		battery := &Battery{
			Name: row[1],
		}
		program := &Program{
			Name:    row[0],
			Battery: battery,
		}
		programs = append(programs, program)
	}
	return programs, nil
}

func (self *DatabaseManager) InsertProgram(programName string, batteryName string) error {
	tx := self.NewDatabaseTx()
	if err := tx.Begin(); err != nil {
		return err
	}

	_, err := tx.insert("program", map[string]interface{}{
		"program_name": programName,
		"battery_name": batteryName,
	})
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (self *DatabaseManager) updateBattery(tx *DatabaseTx, batteryName string, testNames []string) error {
	for _, testName := range testNames {
		_, err := tx.insert("battery_test", map[string]interface{}{
			"battery_name": batteryName,
			"test_name":    testName,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (self *DatabaseManager) InsertBattery(batteryName string, testNames []string) error {
	tx := self.NewDatabaseTx()
	if err := tx.Begin(); err != nil {
		return err
	}

	_, err := tx.insert("battery", map[string]interface{}{
		"battery_name": batteryName,
	})
	if err != nil {
		tx.Rollback()
		return err
	}

	if err := self.updateBattery(tx, batteryName, testNames); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (self *DatabaseManager) UpdateBattery(batteryName string, testNames []string) error {
	tx := self.NewDatabaseTx()
	if err := tx.Begin(); err != nil {
		return err
	}

	if err := self.updateBattery(tx, batteryName, testNames); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (self *DatabaseManager) InsertTest(testName string, testCategory string, testId string) error {
	tx := self.NewDatabaseTx()
	if err := tx.Begin(); err != nil {
		return err
	}

	_, err := tx.insert("test", map[string]interface{}{
		"test_name":     testName,
		"test_category": testCategory,
		"test_id":       testId,
	})
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (self *DatabaseManager) InsertCycle(programName string, cycleId int, cycleName string) error {
	tx := self.NewDatabaseTx()
	if err := tx.Begin(); err != nil {
		return err
	}

	_, err := tx.insert("cycle", map[string]interface{}{
		"program_name": programName,
		"cycle_id":     cycleId,
		"cycle_name":   cycleName,
		"start_date":   time.Now().Format(timeFormat),
	})
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (self *DatabaseManager) InsertRound(programName string, cycleId int, roundId int, packageName string,
	packageTarget string, packageVersion string) error {
	tx := self.NewDatabaseTx()
	if err := tx.Begin(); err != nil {
		return err
	}

	_, err := tx.insert("round", map[string]interface{}{
		"program_name":    programName,
		"cycle_id":        cycleId,
		"round_id":        roundId,
		"package_name":    packageName,
		"package_target":  packageTarget,
		"package_version": packageVersion,
		"start_date":      time.Now().Format(timeFormat),
	})
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (self *DatabaseManager) UpdateCycle(programName string, cycleId int) error {
	tx := self.NewDatabaseTx()
	if err := tx.Begin(); err != nil {
		return err
	}

	err := tx.update("cycle", map[string]interface{}{
		"end_date": time.Now().Format(timeFormat),
	}, map[string]interface{}{
		"program_name": programName,
		"cycle_id":     cycleId,
	})
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (self *DatabaseManager) UpdateRound(programName string, cycleId int, roundId int) error {
	tx := self.NewDatabaseTx()
	if err := tx.Begin(); err != nil {
		return err
	}

	err := tx.update("round", map[string]interface{}{
		"end_date": time.Now().Format(timeFormat),
	}, map[string]interface{}{
		"program_name": programName,
		"cycle_id":     cycleId,
		"round_id":     roundId,
	})
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (self *DatabaseManager) GetPrograms() ([]*Program, error) {
	res, err := self.query("select program.program_name, cycle_id, cycle_name, start_date, end_date from program left outer join cycle on program.program_name = cycle.program_name order by program.program_name, cycle.cycle_id")
	if err != nil {
		return nil, err
	}

	programs := []*Program{}
	for _, row := range res.Rows {
		name := row[0]
		length := len(programs)

		var program *Program
		if length == 0 || programs[length-1].Name != name {
			program = &Program{
				Name:   name,
				Cycles: []*Cycle{},
			}
			programs = append(programs, program)
		} else {
			program = programs[length-1]
		}

		if id, err := strconv.Atoi(row[1]); err == nil {
			cycle := &Cycle{
				Id:        id,
				Name:      row[2],
				StartDate: row[3],
				EndDate:   row[4],
			}
			program.Cycles = append(program.Cycles, cycle)
		}
	}
	return programs, nil
}

func (self *DatabaseManager) GetProgram(programName string, cycleId int) (*Program, error) {
	query := fmt.Sprintf("select battery_test.battery_name, test.test_name, test.test_category, test.test_id from program join battery_test on program.battery_name = battery_test.battery_name join test on battery_test.test_name = test.test_name where program_name = '%s'", programName)
	res, err := self.query(query)
	if err != nil {
		return nil, err
	}

	if len(res.Rows) == 0 {
		return nil, &core.MartianError{fmt.Sprintf("Failed to find battery for program %s", programName)}
	}

	program := &Program{
		Name:   programName,
		Cycles: []*Cycle{},
	}
	for _, row := range res.Rows {
		if program.Battery == nil {
			program.Battery = &Battery{
				Name:  row[0],
				Tests: []*Test{},
			}
		}
		program.Battery.Tests = append(program.Battery.Tests, &Test{
			Name:     row[1],
			Category: row[2],
			Id:       row[3],
		})
	}

	query = fmt.Sprintf("select cycle_name, start_date, end_date from cycle where program_name = '%s' and cycle_id = %d",
		programName, cycleId)
	res, err = self.query(query)
	if err != nil {
		return nil, err
	}

	if len(res.Rows) == 0 {
		return nil, &core.MartianError{fmt.Sprintf("Failed to find cycle %d for program %s", cycleId, programName)}
	}

	row := res.Rows[0]
	cycle := &Cycle{
		Id:        cycleId,
		Name:      row[0],
		StartDate: row[1],
		EndDate:   row[2],
		Rounds:    []*Round{},
	}
	program.Cycles = append(program.Cycles, cycle)

	query = fmt.Sprintf("select round_id, package_name, package_target, package_version, start_date, end_date from round where program_name = '%s' and cycle_id = %d", programName, cycleId)
	res, err = self.query(query)
	if err != nil {
		return nil, err
	}

	for _, row := range res.Rows {
		id, _ := strconv.Atoi(row[0])
		cycle.Rounds = append(cycle.Rounds, &Round{
			Id:             id,
			PackageName:    row[1],
			PackageTarget:  row[2],
			PackageVersion: row[3],
			StartDate:      row[4],
			EndDate:        row[5],
			Tests:          []*Test{},
		})
	}

	return program, nil
}

func (self *DatabaseManager) GetRound(programName string, cycleId int, roundId int) (*Round, error) {
	query := fmt.Sprintf("select package_name, package_target, package_version, start_date, end_date from round where program_name = '%s' and cycle_id = %d and round_id = %d", programName, cycleId, roundId)
	res, err := self.query(query)
	if err != nil {
		return nil, err
	}

	if len(res.Rows) == 0 {
		return nil, &core.MartianError{fmt.Sprintf("Failed to find round %d with program %s, cycle %d",
			roundId, programName, cycleId)}
	}

	row := res.Rows[0]
	round := &Round{
		Id:             roundId,
		PackageName:    row[0],
		PackageTarget:  row[1],
		PackageVersion: row[2],
		StartDate:      row[3],
		EndDate:        row[4],
	}
	return round, nil
}

func (self *DatabaseManager) GetTest(testName string) (*Test, error) {
	query := fmt.Sprintf("select test_category, test_id from test where test_name = '%s'", testName)
	res, err := self.query(query)
	if err != nil {
		return nil, err
	}

	if len(res.Rows) == 0 {
		return nil, &core.MartianError{fmt.Sprintf("Failed to find test %s", testName)}
	}

	row := res.Rows[0]
	test := &Test{
		Name:     testName,
		Category: row[0],
		Id:       row[1],
	}
	return test, nil
}

func formatRow(row map[string]interface{}) ([]string, []string, error) {
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
				return nil, nil, err
			}
			newValue = string(bytes)
		} else {
			newValue = fmt.Sprintf("%v", value)
		}

		keys = append(keys, key)
		values = append(values, fmt.Sprintf("'%s'", newValue))
	}
	return keys, values, nil
}

func formatColumns(row map[string]interface{}) ([]string, error) {
	keys, values, err := formatRow(row)
	if err != nil {
		return nil, err
	}

	cols := []string{}
	for i := range keys {
		key := keys[i]
		value := values[i]

		cols = append(cols, fmt.Sprintf("%s=%s", key, value))
	}
	return cols, nil
}

func (self *DatabaseTx) update(tableName string, row map[string]interface{}, where map[string]interface{}) error {
	rowCols, err := formatColumns(row)
	if err != nil {
		return err
	}

	whereCols, err := formatColumns(where)
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf("update %s set %s where %s", tableName, strings.Join(rowCols, ", "), strings.Join(whereCols, " and "))
	_, err = self.tx.Exec(cmd)
	return err
}

func (self *DatabaseTx) insert(tableName string, row map[string]interface{}) (int64, error) {
	keys, values, err := formatRow(row)
	if err != nil {
		return 0, err
	}

	cmd := fmt.Sprintf("insert into %s(%s) values(%s)", tableName, strings.Join(keys, ", "), strings.Join(values, ", "))
	res, err := self.tx.Exec(cmd)
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
