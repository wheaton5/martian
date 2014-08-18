//
// Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
//
// Marsoc utilities.
//
package main

import (
	"fmt"
	"margo/core"
)

func logInfo(component string, format string, v ...interface{}) {
	fmt.Printf("[%s] %s %s\n", component, core.Timestamp(), fmt.Sprintf(format, v...))
}

func logInfoNoTime(component string, format string, v ...interface{}) {
	fmt.Printf("[%s] %s\n", component, fmt.Sprintf(format, v...))
}

func logError(err error, component string, format string, v ...interface{}) {
	fmt.Printf("[%s] %s %s\n          %s\n", component, core.Timestamp(), fmt.Sprintf(format, v...), err.Error())
}
