// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package main

import (
	"martian/ligo/ligolib"
	"martian/ligo/ligoweb"
	"os"
)

func main() {
	c := ligolib.Setup(os.Getenv("LIGO_DB"))

	ligoweb.SetupServer(3000, c, os.Getenv("LIGO_WEBDIR"))

}
