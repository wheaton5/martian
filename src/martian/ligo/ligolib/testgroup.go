// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package ligolib

import (
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"
	"time"
)

type MultiResultSet struct {
	Metrics []string
	Data    [][]MetricResult
}

func FormatHalf(data interface{}) string {

	t, ok := data.(time.Time)
	if ok {
		fmt.Sprintf("%v", t.Unix())
	}

	f, ok := data.(float64)

	if ok {
		return fmt.Sprintf("%.4g", f)
	}

	return fmt.Sprintf("%v", data)

}

/*
 * Render the text inside a comparison cell.  Cells are always rendered as
 * OLD/NEW but if old or new looks like a float, we try to render it nicely
 */
func FormatCell(oldv interface{}, newv interface{}) string {

	so := FormatHalf(oldv)
	sn := FormatHalf(newv)
	return fmt.Sprintf("%v / %v", so, sn)
}

func FormatMRSAsPlot(project *Project, mrs *MultiResultSet) *Plot {

	var p Plot

	/* Step 1: Build the plot matrix.*/

	/* We need to add an extra row for the column headers */
	p.ChartData = make([][]interface{}, len(mrs.Data)+1)
	p.Annotations = make([][]int, len(mrs.Data)+1)

	columns := len(mrs.Metrics)
	for i := 0; i < len(mrs.Data)+1; i++ {
		p.ChartData[i] = make([]interface{}, columns)
		p.Annotations[i] = make([]int, columns)
	}

	/* Step 2: Copy data from the matrix of metric comparisons
	 * into the plot. Making the right UI transforms along the way.
	 */
	for sample_i := 0; sample_i < len(mrs.Data); sample_i++ {
		for metric_i := 0; metric_i < columns; metric_i++ {
			mdata := mrs.Data[sample_i][metric_i]

			/* Render the cell contents */

			p.ChartData[sample_i+1][metric_i] = FormatCell(mdata.BaseVal, mdata.NewVal)

			/* Compute the cell styling */
			if !mdata.Diff {
				p.Annotations[sample_i+1][metric_i] |= 2
			}

			if !mdata.NewOK {
				p.Annotations[sample_i+1][metric_i] |= 1
			}
		}
	}

	/* Now go and setup the column labels */

	p.ChartData[0] = make([]interface{}, columns)

	for i, m_name := range mrs.Metrics {
		p.ChartData[0][i] = HumanizeMetricName(project, m_name)
	}

	return &p
}

/* This manages the "testgroup" fields in a really crazy way.
 * If the string looks like an ordinary word, we treat it as a filter on the testgroup field
 * the DB.  Otherwise, we assume that the string is secretely SQL and treat it as an ordinary
 * SQL expression.
 */
func InterpretTestGroup(test string) WhereAble {
	sql_test_chars := "'\" ="

	if strings.ContainsAny(test, sql_test_chars) {
		return NewStringWhere(test)
	} else {
		return NewStringWhere(fmt.Sprintf("test_reports.testgroup = '%v'", test))
	}
}

/* Compare two groups */
func CompareTestGroups(db *CoreConnection, m *Project, oldgroup string, newgroup string) (*MultiResultSet, error) {

	/* Step 1: Grab all of the data that we need */
	list_of_metrics := make([]string, 0, len(m.Metrics))

	for metric_name := range m.Metrics {
		list_of_metrics = append(list_of_metrics, metric_name)
	}

	sort.Strings(list_of_metrics)

	list_of_metrics = AugmentMetrics(list_of_metrics, "sampleid")
	list_of_metrics = AugmentMetrics(list_of_metrics, "testgroup")

	/* Step 2: Grab data from the database. Get every defined metric from every
	 * sample in the new and old test groups.
	 */
	basedata, err := db.JSONExtract2(InterpretTestGroup(oldgroup),
		list_of_metrics,
		"",
		nil,
		nil)

	if err != nil {
		return nil, err
	}

	newdata, err := db.JSONExtract2(InterpretTestGroup(newgroup),
		list_of_metrics,
		"",
		nil,
		nil)

	if err != nil {
		return nil, err
	}

	/* Step 3: Make a map for newdata that associates sampleID to its metrics. */
	newdata_map := make(map[string]map[string]interface{})

	for _, sampledata := range newdata {
		s := sampledata["sampleid"].(string)
		newdata_map[s] = sampledata
	}

	/* Step 4: Do the comparisons. For every sample in the old testgroup,
	 * find its corresponding sample in the new test group and compare
	 * the corresponding metric with 'CompareValues'
	 *
	 */
	res := new(MultiResultSet)

	res.Metrics = list_of_metrics
	res.Data = make([][]MetricResult, 0, 0)

	/* Iterate over every sample ID in basedata.  If something shows up
	 * in basedata but not newdata, it will have a row of broken comparisons
	 * (which is the expected result).
	 */
	for i := 0; i < len(basedata); i++ {
		mra := make([]MetricResult, 0, 0)
		sampleid := basedata[i]["sampleid"].(string)
		/* Iterate over every key in this sample */
		for _, key := range list_of_metrics {

			/* Extract values and do the comparison */
			oldval := basedata[i][key]

			var newval interface{}
			newsample := newdata_map[sampleid]
			if newsample != nil {
				newval = newsample[key]
			}

			comparison := CompareValues(m, key, sampleid, oldval, newval)

			mra = append(mra, comparison)
		}
		res.Data = append(res.Data, mra)
		//res.Data[sampleid] = mra
	}

	return res, nil

}

func CompareValues(p *Project, key string, sample_id string, oldval interface{}, newval interface{}) MetricResult {

	var mr MetricResult

	metric, _ := p.LookupMetricDef(key, sample_id)

	if metric == nil {
		log.Printf("Huh? metric %v is missing", key)
		return mr
	}

	mr.HumanName = metric.HumanName
	mr.JSONPath = metric.JSONPath
	mr.BaseVal = oldval
	mr.NewVal = newval

	newfloat, ok1 := newval.(float64)
	basefloat, ok2 := oldval.(float64)

	if ok1 && ok2 {
		mr.DiffPerc = PercentDifference(newfloat, basefloat)
		mr.Diff = !CheckDiff(metric, basefloat, newfloat, mr.DiffPerc)
		mr.OK = true

	} else {
		mr.Diff = reflect.DeepEqual(newval, oldval)
		mr.OK = false
	}

	mr.NewOK = CheckOK(metric, newval)
	mr.OldOK = CheckOK(metric, oldval)

	return mr
}
