// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package main

import (
	"martian/ligolib"
	"log"
)

func main() {
	c := ligolib.Setup()

	r, _ := c.GrabRecords(ligolib.NewEmptyWhere(), "test_reports", ligolib.ReportRecord{});

	rt := r.([]ligolib.ReportRecord)

	log.Printf("STUFF: %v", rt);


	r1 := c.JSONExtract2(ligolib.NewEmptyWhere(), []string{
		"SHA",
		"sampleid",
		"/SUMMARIZE_REPORTS_PD/universal_fract_snps_phased",
		"/SUMMARIZE_REPORTS_PD/r1_q30_bases"},
		"")


	log.Printf("R1: %v", r1);
}
