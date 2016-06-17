// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package main

import (
	"martian/sere2lib"
)

func main() {
	c := sere2lib.Setup()

	c.GrabRecords("");
}
