// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package main

import (
	"martian/sere2lib"
	"log"
)

func main() {
	c := sere2lib.Setup()

	c.GrabRecords("");

	r1 := c.JSONExtract("test_reports", "", []string{"summaryjson/effective_diversity_reads", "sha", "summaryjson/fraction_on_target"})

	log.Printf("R1: %v", r1);
}
