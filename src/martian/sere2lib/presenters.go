// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package sere2lib

/*
 * This file implements several "presenters". A presenter takes a
 * set of arguments and preoduces data that is ready to be JSONified
 * to produce some part of the display.
 */

import (
	"fmt"
	"log"
	"strconv"
)

type Plot struct {
	Name      string
	ChartData [][]interface{}
}

/*
 * Generate a simple x/y plot of two columns (or JSON columns) subject to a
 * where clause.
 */
func (c *CoreConnection) XYPresenter(where string, x string, y string) *Plot {

	/* Get the data*/
	data := c.JSONExtract2(where, []string{x, y})

	/* Convert it to the format that google charts wants */
	ChartData := Rotate2(data, x, y)

	return &Plot{
		fmt.Sprintf("PLOT %v versus %v (%v)", x, y, where), ChartData}

}

func RotateOld(src []map[string]interface{}, x string, y string) ([]interface{}, []interface{}) {

	xa := make([]interface{}, len(src))
	ya := make([]interface{}, len(src))

	for i, datum := range src {
		xa[i] = datum[x]
		ya[i] = datum[y]
	}

	return xa, ya
}

/*
 * This converts the results from the DB fetcher to make simple x-y plots with
 * google charts.
 */
func Rotate2(src []map[string]interface{}, x string, y string) [][]interface{} {

	res := make([][]interface{}, len(src)+1)
	res[0] = []interface{}{x, y}

	for i, datum := range src {
		n := make([]interface{}, 2)
		n[0] = datum[x]

		dy := datum[y].(string)

		dy_as_int, err := strconv.ParseFloat(dy, 64)
		if err == nil {
			n[1] = dy_as_int
		} else {
			log.Printf("UHOH %v %v", dy, err)
			n[1] = datum[y]
		}

		res[i+1] = n
	}

	return res
}
