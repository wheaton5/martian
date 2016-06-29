// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package main

import (
	"os"
	"martian/ligolib"
	"martian/ligoweb"
)

func main() {
	c := ligolib.Setup()

	
	ligoweb.SetupServer(3000, c, os.Getenv("LIGO_WEBDIR"))

}
