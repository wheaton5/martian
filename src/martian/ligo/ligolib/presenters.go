// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package ligolib

/*
 * This file implements several "presenters". A presenter takes a
 * set of arguments and preoduces data that is ready to be JSONified
 * to produce some part of the display.
 */

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strconv"
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

	/*
	 * This annotations matrix specifies attributes (that convert to CSS) for
	 * each element of chartdata.
	 *
	 * It is a bitmask of the following operations
	 * 1 Datum failed to meet it's target
	 * 2 Datum is significantly different from its comparant
	 * 4 Datum wasn't successfully extracted from the summary/db info.
	 */
	Annotations [][]int
}

func MakeAnnotation(rows int, cols int) [][]int {
	r := make([][]int, rows)
	for i := 0; i < rows; i++ {
		r[i] = make([]int, cols)
	}
	return r
}

/*
 * Grab all of the data for a given pipestance.
 * Note: This can run into problems because there is a LOT
 * of data for a pipestance!!!! In this case, the |where| argument
 * applies to the test_report_summaries table and can be used
 * to sub-select the stages for which we want to grab data.
 */
func (c *CoreConnection) AllDataForPipestance(where WhereAble, pid int) (*Plot, error) {
	result, err := c.GrabAllMetricsRaw(where, pid)
	if err != nil {
		return nil, err
	}

	rotated := RotateStructs(result)
	log.Printf("Processed %v json entries", len(result))

	return &Plot{"", rotated, nil}, nil
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

	return &Plot{"Some stuff", fields, nil}, nil
}

/*
 * Compare software version numbers in the format X.Y.Z.
 * Returns true if a is less than b.
 */
func CompareSHA(a string, b string) bool {
	aa := strings.Split(a, ".")
	ba := strings.Split(b, ".")

	/* Iterate over each component of the version */
	for i := 0; i < Min(len(aa), len(ba)); i++ {
		ai, aerr := strconv.Atoi(aa[i])
		bi, berr := strconv.Atoi(ba[i])
		if aerr == nil && berr == nil {
			/* Do they look like integers? compare as such! */
			if ai != bi {
				return ai < bi
			}
		} else {
			/* Do they look like strings? Fall back to
			 * lexographic comparison.
			 */
			if aa[i] != ba[i] {
				return aa[i] < ba[i]
			}
		}
	}

	/* If strings are otherwise identical, the shortest one is "smaller" */

	return len(ba) < len(ba)
}

/*
 * This function takes an array of JSONExtra2 results that contain SHA and sampleid.
 * for each sampleID, it selects only the latest SHA (according to comparesha, above)
 * and retains that in the output.  not-latest-data will not be in the output. The
 * input array is not modified.
 */
func LatestOnly(data []map[string]interface{}) []map[string]interface{} {

	/* Where we store output */
	newguy := make([]map[string]interface{}, 0, len(data))

	/* Keep a map of sha--> latest version */
	versiontable := make(map[string]string)

	/* Iterate over each datum and build up a table of
	 * sampleid --> latest sha present for this sample id
	 */
	for i := range data {
		sid := data[i]["sampleid"].(string)
		sha := data[i]["SHA"].(string)

		oldsha, hasold := versiontable[sid]
		if !hasold {
			versiontable[sid] = sha
		} else if !CompareSHA(sha, oldsha) {
			versiontable[sid] = sha
		}
	}

	/* Now iterate over each datum and filter based on versiontable.
	 * only accept results whose sha matches the best sha found for that
	 * sample.  This means that if the same sample appears twice with the
	 * same sha, it will end up in the output twice.
	 */
	for i := range data {
		sid := data[i]["sampleid"].(string)
		sha := data[i]["SHA"].(string)

		if versiontable[sid] == sha {
			newguy = append(newguy, data[i])
		}
	}

	return newguy
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
func (c *CoreConnection) PresentAllMetrics(where WhereAble, mets *Project, limit *int, offset *int, latest bool) (*Plot, error) {

	/* Create an array with every field of interest */
	fields := make([]string, 0, 0)

	/* We are always interested in test_reports.id!  The UI expects
	 * it.*/
	fields = append(fields, "test_reports.id")

	/* And of course get all of the metrics defined by the project */
	for k := range mets.Metrics {
		fields = append(fields, k)
	}

	sort.Strings(fields[1:len(fields)])

	fields = AugmentMetrics(fields, "sampleid")
	fields = AugmentMetrics(fields, "SHA")

	data, err := c.JSONExtract2(MergeWhereClauses(mets.WhereAble, where), fields, "-finishdate", limit, offset)

	if err != nil {
		return nil, err
	}

	/* Trim duplicate records */
	if latest {
		/* The LatestOnly filter only works if we actually have all of the data
		 * for this query. If offset or limit has truncated the data, bail out
		 * now.
		 */
		if limit != nil && ((offset != nil && *offset > 0) || (len(data) >= *limit)) {
			return nil, errors.New("Too much data for latest filter to work correctly.")
		}
		data = LatestOnly(data)
	}

	var plot Plot
	fields = append(fields, "OK")
	gendata := RotateN(data, fields)
	plot.ChartData = gendata
	plot.Name = ""
	plot.Annotations = MakeAnnotation(len(gendata), len(gendata[0]))

	/* This is a horrible bloody mess! Note that subtle dependence on the exact what that
	 * RotateN orginizaes the values that it returns!
	 *
	 * This loops through the output column by column . For each cell, (except the 0th row
	 * which is the column labels) it runs ResolveAndCheck to see the Annotations bitmap for
	 * that cell correctly. This turns cells that fail their targets red.
	 *
	 * While we're at it, we change the label of each column to try to be more human friendly.
	 */
	for which_metric_idx := 0; which_metric_idx < len(gendata[0]); which_metric_idx++ {
		metric_name := gendata[0][which_metric_idx].(string)
		for row_idx := 1; row_idx < len(gendata); row_idx++ {

			/* Off by 1 here because RotateN added a header row with column names */
			sampleid := data[row_idx-1]["sampleid"].(string)
			ok := ResolveAndCheckOK(mets, metric_name, sampleid, gendata[row_idx][which_metric_idx])
			if !ok {
				plot.Annotations[row_idx][which_metric_idx] = 1
			}
		}

		/* Adjust the metric name */
		m := mets.Metrics[metric_name]
		if m != nil && m.HumanName != "" {
			metric_name = m.HumanName
		} else {
			ma := strings.Split(metric_name, "/")
			metric_name = ma[len(ma)-1]
		}

		if len(metric_name) > 16 {
			metric_name = metric_name[0:16]
		}

		gendata[0][which_metric_idx] = metric_name
	}

	return &plot, nil
}

/*
 * Produce data suitable for plotting in a table or chart.
 */
func (c *CoreConnection) GenericChartPresenter(where WhereAble, mets *Project, fields []string, sortby string, limit *int, offset *int, latest bool) (*Plot, error) {
	data, err := c.JSONExtract2(MergeWhereClauses(mets.WhereAble, where), fields, sortby, limit, offset)

	if err != nil {
		return nil, err
	}

	if latest {
		/* The LatestOnly filter only works if we actually have all of the data
		 * for this query. If offset or limit has truncated the data, bail out
		 * now.
		 */
		if limit != nil && ((offset != nil && *offset > 0) || (len(data) >= *limit)) {
			return nil, errors.New("Too much data for latest filter to work correctly.")
		}
		data = LatestOnly(data)
	}

	ChartData := RotateN(data, fields)
	return &Plot{"A plot", ChartData, nil}, nil
}

func (c *CoreConnection) SuperCompare(baseid int, newid int, mets *Project, where WhereAble) (*Plot, error) {
	comps, err := CompareAll(mets, c, baseid, newid, where)
	if err != nil {
		return nil, err
	}

	return c.FixCompareResults(comps), nil
}

/*
 * Produce data suitable for plotting in a table that compares two samples.
 */
func (c *CoreConnection) GenericComparePresenter(baseid int, newid int, mets *Project) (*Plot, error) {

	comps, err := Compare2(c, mets, baseid, newid)

	if err != nil {
		return nil, err
	}

	return c.FixCompareResults(comps), nil

}

func (c *CoreConnection) FixCompareResults(comps []MetricResult) *Plot {
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

	return &Plot{"A chart", data, nil}
}

/*
 * Grab the values of every *DEFINED* metric for a test ID and return
 * them. Also return an "OK" field that defines if a metric hit spec.
 */
func (c *CoreConnection) DetailsPresenter(id int, mets *Project) (*Plot, error) {

	/* Flatten the list of metrics */
	list_of_metrics := make([]string, 0, len(mets.Metrics))
	for metric_name, _ := range mets.Metrics {
		list_of_metrics = append(list_of_metrics, metric_name)
	}

	sort.Strings(list_of_metrics)

	list_of_metrics = AugmentMetrics(list_of_metrics, "sampleid")

	/* Grab the data for the metrics that we care about */
	data, err := c.JSONExtract2(NewStringWhere(fmt.Sprintf("test_reports.id = %v", id)),
		list_of_metrics,
		"",
		nil,
		nil)

	if err != nil {
		return nil, err
	}

	if len(data) < 1 {
		return nil, errors.New("No Data for your ID!")
	}

	/*
	 * Database will return exactly one row that maps metric names to values.
	 * Convert that to a plot like format that looks like.
	 * [
	 *  [metric_1_name, metric_1_value, OK],
	 *  [metric_2_name, metric_2_value, OK],
	 *  ...
	 * ]
	 */
	d1 := data[0]

	details := make([][]interface{}, 0, 0)
	details = append(details, []interface{}{"Metric", "Value", "OK", "Low", "High"})
	// XXX DANGER!
	sampleid := d1["sampleid"].(string)
	annotations := MakeAnnotation(len(d1)+1, len(details[0]))

	for _, metric_name := range list_of_metrics {
		metric_value := d1[metric_name]
		met_ok := ResolveAndCheckOK(mets, metric_name, sampleid, metric_value)
		metric_def, _ := mets.LookupMetricDef(metric_name, sampleid)
		row := []interface{}{metric_name, fmt.Sprintf("%v", metric_value), met_ok, metric_def.Low, metric_def.High}

		/* Red-out the entire row if we didn't pass the metric target */
		if !met_ok {
			for j := 0; j < len(details[0]); j++ {
				log.Printf("%v %v %v %v", len(d1), len(details[0]), len(details), j)
				annotations[len(details)][j] = 1
			}
		}
		details = append(details, row)
	}

	return &Plot{"", details, annotations}, nil
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
