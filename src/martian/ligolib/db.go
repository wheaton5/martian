// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

/*
 * This package provides basic low-level DB stuff.
 */

package ligolib

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"log"
	"reflect"
	"strconv"
	"strings"
)

type Queryable interface {
	Query(q string, args ...interface{}) (*sql.Rows, error)
	QueryRow(q string, args ...interface{}) *sql.Row
}

/*
 * This encapsulates a database connection.
 */
type CoreConnection struct {
	Conn *sql.DB
	Tx   *sql.Tx
	Q    Queryable
}

/*
 * Build a connection to the SERE database.  TODO: This should look at
 * environment variables to figure out the database to connect to
 */
func Setup(dbspec string) *CoreConnection {
	conn := new(CoreConnection)

	/* FIXME: hard coded IP address and password :( */
	db, err := sql.Open("postgres", dbspec)
	if err != nil {
		panic(err)
	}

	conn.Conn = db
	conn.Tx = nil
	conn.Q = db

	return conn

}

func (db *CoreConnection) Begin() error {
	if db.Tx != nil {
		panic("TX in TX")
	}

	tx, err := db.Conn.Begin()
	db.Tx = tx
	db.Q = tx

	return err
}

func (db *CoreConnection) Commit() error {
	err := db.Tx.Commit()
	db.Q = db.Conn
	db.Tx = nil

	return err
}

/*
 * Insert a record into a database table.  We assume that the names of the
 * fields in |r| (which must be a struct) correspond to the fields in the
 * database table.
 */
func (c *CoreConnection) InsertRecord(table string, record interface{}) (int, error) {

	/* Keys is a list of column names, extracted from the type of r*/
	keys := make([]string, 0)

	/* Interpolator is just a big list of "$1", "$2", "$3", ..... that we
	 * use to make the query formatter do the right thing.
	 */
	interpolator := make([]string, 0)

	/* Values is an array of values to add, in the same order as keys */
	values := make([]interface{}, 0)

	val_of_r := reflect.ValueOf(record)
	type_of_r := val_of_r.Type()

	/* Iterate over every field in |record| and append its name into keys, its
	 * value in the values, and $next into interpolator.
	 */
	interpolator_index := 0
	for i := 0; i < val_of_r.NumField(); i++ {
		field := type_of_r.Field(i)
		tag := field.Tag.Get("sql")

		/* Don't set read-only fields. */
		if tag == "RO" {
			continue
		}

		keys = append(keys, field.Name)
		values = append(values, val_of_r.Field(i).Interface())
		interpolator = append(interpolator, fmt.Sprintf("$%v", interpolator_index+1))
		interpolator_index++
	}

	/* Format the query string */
	query := "INSERT INTO " + table + " (" + strings.Join(keys, ",") + ")" +
		" VALUES (" + strings.Join(interpolator, ",") + ")" +
		" RETURNING ID"

	//log.Printf("Q: %v", query)

	/* Do the query */
	result := c.Q.QueryRow(query, values...)

	/* Get the result, which will be the ID of the new row */
	var newid int
	err := result.Scan(&newid)

	if err != nil {
		log.Printf("E: %v %v", err, newid)
	}
	return newid, err
}

/*
 * This implements the awesomeness to extract JSON queries across a join.  We
 * expect a scheme like test_reports:[id, ... other fields]
 * test_report_summaries[id, testreportid, stagename, jsonsummary] with
 * test_report_summaries.testreportid associated to test_reports.id
 *
 * We interpret each key as a path expression if it starts with /. The first
 * element of the path is considered to be the value of "stagename" in the
 * test_report_summaries.  the remaining part of the path indexes into the JSON
 * bag at test_report_summaries.jsonsummary.
 *
 * For example, the key "/SUMMARIZE_REPORTS_PD/universal_fract_snps_phased"
 * with a where clause of "sample_id=12345" will return the json value of
 * "universal_fract_snps_phased" in the SUMMARIZE_REPORTS_PD/summary.json
 * directory for every test with the sample id of 12345.
 */
func (c *CoreConnection) JSONExtract2(where WhereAble, keys []string, sortkey string) []map[string]interface{} {

	/* List of all the JOIN statements we need */
	joins := []string{}

	/* List of all the select clauses we need */
	selects := []string{}

	/* mapping from the first element of a keypath to the join-name for that
	 * JSON blob*/
	tref_map := make(map[string]string)

	/* Index to make temporary unique names for joins */
	next_tref_index := 1

	/* STEP 1: Transform the keys array into a bunch of join and select
	 * statements.  For each report stage that is mentioned in a key, we add
	 * a new join clause and every key adds exactly one select expression.
	 */
	for _, key := range keys {

		if key[0] == '/' {
			/* key is a JSON path */
			keypath := strings.Split(key[1:], "/")
			var join_as_name string

			/* Do we already have a JOIN reference for the
			 * test_report_summaries_row that we're going to use?
			 */
			join_as_name, exists := tref_map[keypath[0]]
			if !exists {
				/* We don't have a join for this table, make one up */
				join_as_name = fmt.Sprintf("tmp_%v", next_tref_index)
				tref_map[keypath[0]] = join_as_name
				next_tref_index++

				/* Compute the JOIN statement for this table reference */
				join_statement :=
					fmt.Sprintf("LEFT JOIN test_report_summaries AS %v ON "+
						"test_reports.id = %v.reportrecordid and %v.stagename='%v'",
						join_as_name, join_as_name, join_as_name, keypath[0])

				joins = append(joins, join_statement)
				log.Printf("NEW: %v-->%v", keypath[0], join_as_name)
			} else {
				log.Printf("OLD: %v-->%v", keypath[0], join_as_name)
			}

			/* Compute the postgres JSON path-like expression for this
			 * key
			 */
			str := join_as_name + ".summaryjson"
			for _, p_element := range keypath[1:] {
				_, ok := strconv.Atoi(p_element)

				/* If p_element looks like an integer, write it as an
				 * integer. Otherwise quote it as a string.
				 */
				if ok == nil {
					str += "->" + p_element
				} else {
					str += "->" + "'" + p_element + "'"
				}
			}
			selects = append(selects, str)
		} else {
			/* If key doesn't start with "/", just grab out of the metadata table */
			selects = append(selects, key)
		}
	}

	query := "SELECT " + strings.Join(selects, ",") + " FROM test_reports " +
		strings.Join(joins, " ")

	query += RenderWhereClause(where)

	/*
	 * STEP 2: Parse the sort key.
	 * we'd like to be able to enable arbitrary JSON paths here but its tricky.
	 * So for now we only allow references to the test_reports table which
	 * handles most of the cases anyways.
	 */
	if sortkey != "" {
		dir := ""
		key_to_use := sortkey
		if sortkey[0] == '+' {
			dir = "ASC"
			key_to_use = sortkey[1:]
		}

		if sortkey[0] == '-' {
			dir = "DESC"
			key_to_use = sortkey[1:]
		}
		query += " ORDER BY " + key_to_use + " " + dir
	}

	log.Printf("QUERY: %v", query)

	/* STEP 3: Actually do the query */
	rows, err := c.Q.Query(query)

	if err != nil {
		panic(err)
	}

	/* STEP 4: Now collect the results. We return an array of maps. Each map
	 * associates the specific keys from the key array with some value.
	 */
	results := make([]map[string]interface{}, 0, 0)
	for rows.Next() {

		/* the Scan function wants an array of pointers to interfaces
		 * and it will unpack data into that array and set the type
		 * decently */
		ifaces := make([]interface{}, len(keys))
		iface_ptrs := make([]interface{}, len(keys))
		for i := 0; i < len(keys); i++ {
			iface_ptrs[i] = &ifaces[i]
		}

		err = rows.Scan(iface_ptrs...)
		if err != nil {
			panic(err)
		}

		rowmap := make(map[string]interface{})

		/* Copy the results into an output map */
		for i := 0; i < len(keys); i++ {
			rowmap[keys[i]] = FixType(ifaces[i])
		}

		results = append(results, rowmap)
	}

	return results
}

/*
 * Fix data type for stuff from the SQL Scan function.
 * JSON numbers are not automatically converted to float64 so we convert
 * anything that looks like a number to a float64. We also convert any
 * byte arrays to strings.
 */
func FixType(in interface{}) interface{} {
	switch in.(type) {
	case []byte:
		f, err := strconv.ParseFloat(string(in.([]byte)), 64)
		if err == nil && f == f {
			return f
		} else {
			return string(in.([]byte))
		}
	default:
		return in
	}
}

/*
 * Grab if a bunch of records from SQL, subject to a WHERE clause.
 * Table is the name of the table to extract records from.
 *
 * outtype is a template: a single instance of a struct that defiens the schema
 * for the table.  We return an array of instances of that struct A typical use
 * looks like
 *
 * out, err := GrabRecords(NewEmptyWhere(), "test_reports", ReportRecords{})
 * my_array = out.([]ReportRecords)
 *
 */

func (c *CoreConnection) GrabRecords(where WhereAble, table string, outtype interface{}) (interface{}, error) {

	/* Compute the field names that we wish to extract */
	fieldnames := FieldsOfStruct(outtype)

	/* Compute the select query */
	query := "SELECT " + strings.Join(fieldnames, ",") + " FROM " + table
	query += RenderWhereClause(where)

	log.Printf("QUERY: %v", query)
	rows, err := c.Q.Query(query)

	if err != nil {
		log.Printf("UHOHL %v", err)
		return []ReportRecord{}, err
	}

	elem_type := reflect.ValueOf(outtype).Type()
	out_array_t := reflect.SliceOf(elem_type)
	out_array := reflect.MakeSlice(out_array_t, 0, 100)

	index := 0
	for rows.Next() {
		out_array = reflect.Append(out_array, reflect.Zero(elem_type))
		val_elem := out_array.Index(index)
		err := UnpackRow(rows, elem_type, val_elem)

		if err != nil {
			log.Printf("UNOHHHHHHH -- %v", err)
			return nil, err
		}
		index++
	}
	return out_array.Interface(), nil
}

/*
 * Unpack a single row into rr. The row must have been selected using the
 * results from ComputeSelectFields on the same type as rr. rr must
 * secretly be a pointer to that type of struct.
 */
func UnpackRow(row *sql.Rows, t reflect.Type, val reflect.Value) error {

	/* Build an array of interfaces where each interface is
	 * a pointer to the correspdoning field of rr
	 */
	ifaces := make([]interface{}, t.NumField())
	for i := 0; i < val.NumField(); i++ {
		ifaces[i] = val.Field(i).Addr().Interface()
	}

	/* Scan the row and let the SQL driver perform type
	 * conversion
	 */
	err := row.Scan(ifaces...)
	if err != nil {
		log.Printf("ERROR: %v", err)
		return err
	}
	return nil
}

/*
 * Return an array of all of the field names for a struct. Note that this traverses the
 * fields in the same order as UnpackRow does and that we rely on this fact.
 */
func FieldsOfStruct(str interface{}) []string {
	output := make([]string, 0, 0)
	val := reflect.ValueOf(str)

	t := val.Type()

	for i := 0; i < val.NumField(); i++ {
		output = append(output, t.Field(i).Name)
	}
	return output
}
