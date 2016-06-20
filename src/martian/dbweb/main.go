// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package main

import (
	"martian/sere2lib"
	"martian/sere2web"
)

func main() {
	c := sere2lib.Setup()

	sere2web.SetupServer(3000, c, "/mnt/home/dstaff/code/mars2/web/sere2")

}
