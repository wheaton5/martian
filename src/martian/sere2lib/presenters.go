// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package sere2lib

/*
 * This file implements several "presenters". A presenter takes a
 * set of arguments and preoduces data that is ready to be JSONified
 * to produce some part of the display.
 */

import ()

type Plot struct {
	Name      string
	ChartData [][]interface{}
}

func (c *CoreConnection) GenericPresentor(where string, fields []string) *Plot {

	data := c.JSONExtract2(where, fields)

	ChartData := RotateN(data, fields)

	return &Plot{"A plot", ChartData}
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

	/* Copy columns into the first row of the output */
	for i, c := range columns {
		res[0][i] = c
	}

	/* Iterate over src and copy data into the output */
	for i, datum := range src {
		newrow := make([]interface{}, len(columns))
		for k, c := range columns {
			newrow[k] = datum[c]
		}
		res[i+1] = newrow
	}
	return res
}
