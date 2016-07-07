// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package ligolib

/*
 * This file implements several "presenters". A presenter takes a
 * set of arguments and preoduces data that is ready to be JSONified
 * to produce some part of the display.
 */

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
)

/*
 * This is the data structure that we use for almost everything
 * that we return for the front-end to render.
 */
type Plot struct {
	/* A name for this plot (currently unused) */
	Name string

	/* A 2-d array of data in the format for google charts to
	 * injest.  The first row contains the column labels and
	 * subsequent rows contain the column data.
	 */
	ChartData [][]interface{}
}

type Datum struct {
	Path  string
	Value interface{}
}

/*
 * Grab all of the data for a given pipestance.
 * Note: This can run into problems because there is a LOT
 * of data for a pipestance!!!! In this case, the |where| argument
 * applies to the test_report_summaries table and can be used
 * to sub-select the stages for which we want to grab data.
 */
func (c *CoreConnection) AllDataForPipestance(where WhereAble, pid int) (*Plot, error) {

	my_where := NewStringWhere(fmt.Sprintf("ReportRecordId = %v", pid))
	reports_i, err := c.GrabRecords(MergeWhereClauses(where, my_where),
		"test_report_summaries",
		ReportSummaryFile{})

	if err != nil {
		return nil, err
	}

	reports := reports_i.([]ReportSummaryFile)

	result := []Datum{}
	for _, r := range reports {
		var toplevel interface{}
		err := json.Unmarshal([]byte(r.SummaryJSON), &toplevel)
		if err != nil {
			log.Printf("Can't unmarshal JSON at %v: %v", r.StageName, err)
		} else {
			FlattenJSON("/"+r.StageName, toplevel, &result)
		}
	}

	rotated := RotateStructs(result)
	log.Printf("Processed %v json entries", len(result))

	return &Plot{"", rotated}, nil
}

/*
 * Take an entire JSON structure and flatten it into a bunch of
 * {Path:.... Key:.... } objects.
 */
func FlattenJSON(base string, json_blob interface{}, result *[]Datum) {
	switch json_blob.(type) {
	case map[string]interface{}:
		/* Iterate over maps and recurse into them */
		for key, val := range json_blob.(map[string]interface{}) {
			FlattenJSON(base+"/"+key, val, result)
		}
	case []interface{}:
		/* Iterate over arrays and recurse into them */
		for idx, val := range json_blob.([]interface{}) {
			FlattenJSON(fmt.Sprintf("%v/%v", base, idx), val, result)
		}
	default:
		/* Render values as strings. */
		*result = append(*result, Datum{base, fmt.Sprintf("%v", json_blob)})
	}
}

/*
 * Product a plot that just lists all of the metrics for this
 * project.
 */
func (c *CoreConnection) ListAllMetrics(mets *Project) (*Plot, error) {

	fields := make([][]interface{}, 0, 0)
	fields = append(fields, []interface{}{"Metric Name"})
	for k := range mets.Metrics {
		fields = append(fields, []interface{}{k})
	}

	return &Plot{"Some stuff", fields}, nil
}

/*
 * Produce a plot that lists all of the metrics for a subset of the data
 * for this project.
 *
 * The output includes two "virtual" datums: test_reports.id and ok.
 * test_reports.id is simply the row in of the test_report in the DB.
 * ok is true if the row passes all specifications.
 *
 */
func (c *CoreConnection) PresentAllMetrics(where WhereAble, mets *Project) (*Plot, error) {

	/* Create an array with every field of interest */
	fields := make([]string, 0, 0)

	/* We are always interested in test_reports.id!  The UI expects
	 * it.*/
	fields = append(fields, "test_reports.id")

	/* And of course get all of the metrics defined by the project */
	for k := range mets.Metrics {
		fields = append(fields, k)
	}

	data, err := c.JSONExtract2(MergeWhereClauses(mets.WhereAble, where), fields, "-finishdate")

	if err != nil {
		return nil, err
	}

	/*
	 * Iterate over all of the data and see if it meets the required metrics.
	 */
	for i := 0; i < len(data); i++ {

		row := data[i]

		all_ok := true
		for metric_name, val := range row {
			metric := mets.Metrics[metric_name]
			if metric != nil {

				ok := CheckOK(mets.Metrics[metric_name], val)
				if !ok {
					all_ok = false
				}
			}
		}
		data[i]["OK"] = all_ok
	}

	var plot Plot
	fields = append(fields, "OK")
	gendata := RotateN(data, fields)
	plot.ChartData = gendata
	plot.Name = ""

	/* Now iterate through the column names in the first row
	 * of plot.ChartData and munge them.  Swap the key name with
	 * a human readable name when possible and truncate the length.
	 */
	for i := 0; i < len(gendata[0]); i++ {
		str := gendata[0][i].(string)
		m := mets.Metrics[str]
		if m != nil && m.HumanName != "" {
			str = m.HumanName
		} else {
			ma := strings.Split(str, "/")
			str = ma[len(ma)-1]
		}

		if len(str) > 16 {
			str = str[0:16]
		}

		gendata[0][i] = str
	}

	return &plot, nil
}

/*
 * Produce data suitable for plotting in a table or chart.
 */
func (c *CoreConnection) GenericChartPresenter(where WhereAble, mets *Project, fields []string, sortby string) (*Plot, error) {
	data, err := c.JSONExtract2(MergeWhereClauses(mets.WhereAble, where), fields, sortby)

	if err != nil {
		return nil, err
	}

	ChartData := RotateN(data, fields)
	return &Plot{"A plot", ChartData}, nil
}

/*
 * Produce data suitable for plotting in a table that compares two samples.
 */
func (c *CoreConnection) GenericComparePresenter(baseid int, newid int, mets *Project) (*Plot, error) {

	comps, err := Compare2(c, mets, baseid, newid)

	if err != nil {
		return nil, err
	}

	/*
	 * This is a hack to render numbers on the server-side for float-like data.
	 * We do this to prevent obnoxious behavior for mixed-type columns in
	 * google charts.
	 */
	for i := range comps {
		f, ok := comps[i].BaseVal.(float64)
		if ok {
			comps[i].BaseVal = fmt.Sprintf("%.5f", f)
		}

		f, ok = comps[i].NewVal.(float64)
		if ok {
			comps[i].NewVal = fmt.Sprintf("%.5f", f)
		}
	}

	data := RotateStructs(comps)

	return &Plot{"A chart", data}, nil
}

/*
 * This function converts from an array-of-maps to an array-of-arrays.
 * We use to to format data for google charts to display.
 *
 * The columns argument specifies what elements to select from the map.
 * The first element of the returned array is exactly the columns array.
 */
func RotateN(src []map[string]interface{}, columns []string) [][]interface{} {

	res := make([][]interface{}, len(src)+1)

	res[0] = make([]interface{}, len(columns))

	/* Copy column names into the first row of the output */
	for i, c := range columns {
		res[0][i] = c
	}

	/* Iterate over src and copy data into the output */
	for i, datum := range src {
		newrow := make([]interface{}, len(columns))
		for keyname, column_datum := range columns {
			d := datum[column_datum]
			newrow[keyname] = d
		}
		res[i+1] = newrow
	}
	return res
}

/*
 * Convert a single struct to an array
 */
func RotateStruct(record interface{}) []interface{} {
	val_of_r := reflect.ValueOf(record)
	outmap := make([]interface{}, val_of_r.NumField())

	for i := 0; i < val_of_r.NumField(); i++ {
		outmap[i] = val_of_r.Field(i).Interface()
	}
	return outmap
}

/*
 * This converts an array of structs to an array of arrays similarly
 * to RotateN.
 * NOTE: This does the XXX WRONG XXX thing for pointers or nested
 * structs.
 */

func RotateStructs(record_array interface{}) [][]interface{} {

	val := reflect.ValueOf(record_array)
	ma := make([][]interface{}, 0, val.Len()+1)

	keys := FieldsOfStruct(val.Index(0).Interface())

	firstrow := make([]interface{}, len(keys))

	for i, k := range keys {
		firstrow[i] = k
	}

	ma = append(ma, firstrow)

	for i := 0; i < val.Len(); i++ {
		v_here := val.Index(i).Interface()
		ma = append(ma, RotateStruct(v_here))
	}
	return ma
}
