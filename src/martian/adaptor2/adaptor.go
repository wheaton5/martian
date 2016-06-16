// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package main

import (
	"flag"
	"io/ioutil"
	"martian/core"
	"martian/sere2lib"
	"os"
	"strconv"
)

var flag_pipestance_path = flag.String("path", "", "path to pipestance")

func LookupCallInfo(basepath string) int {

	_, _, ast, err := core.Compile(basepath+"_mrosource", []string{}, false)
	if err != nil {
		panic(err)
	}

	s := core.SearchPipestanceParams(ast, "sample_id")
	if s == nil {
		panic("WTF2")
	}
	res, err := strconv.Atoi(s.(string))
	if err != nil {
		panic(err)
	}
	return res
}

func main() {
	c := sere2lib.Setup()
	c.Dump()

	var rr sere2lib.ReportRecord

	flag.Parse()

	if *flag_pipestance_path == "" {
		panic("bad args")
	}

	version, err := sere2lib.GetPipestanceVersion(*flag_pipestance_path)

	if err != nil {
		panic(err)
	}
	project := sere2lib.GuessProject(*flag_pipestance_path)
	if project == nil {
		panic("can't figure out what kind of project this is!")
	}

	rr.SHA = version
	rr.Branch = version
	rr.SampleId = LookupCallInfo(*flag_pipestance_path)
	rr.CellLine = "blah"
	rr.Project = project.Name
	rr.UserId = os.Getenv("USER")

	jsondata, err := ioutil.ReadFile(*flag_pipestance_path + "/" + project.SummaryJSONPath)
	if err != nil {
		panic(err)
	}

	rr.SummaryJSON = string(jsondata)
	rr.Comments = "{}"
	rr.InterpretedJSON = "{}"
	c.InsertRecord("test_reports", rr)
}
