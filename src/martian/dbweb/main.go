// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package main

import (
	"os"
	"martian/sere2lib"
	"martian/sere2web"
)

func main() {
	c := sere2lib.Setup()

	
	sere2web.SetupServer(3000, c, os.Getenv("SERE2_WEBDIR"))

}
