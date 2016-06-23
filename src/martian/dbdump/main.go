// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package main

import (
	"martian/sere2lib"
	"log"
)

func main() {
	c := sere2lib.Setup()

	r, _ := c.GrabRecords(sere2lib.NewEmptyWhere(), "test_reports", sere2lib.ReportRecord{});

	rt := r.([]sere2lib.ReportRecord)

	log.Printf("STUFF: %v", rt);


	r1 := c.JSONExtract2(sere2lib.NewEmptyWhere(), []string{
		"SHA",
		"sampleid",
		"/SUMMARIZE_REPORTS_PD/universal_fract_snps_phased",
		"/SUMMARIZE_REPORTS_PD/r1_q30_bases"},
		"")


	log.Printf("R1: %v", r1);
}
