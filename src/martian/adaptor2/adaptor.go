// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package main

import (
	"flag"
	"martian/core"
	"martian/sere2lib"
	"os"
	"strconv"
)

var flag_pipestance_path = flag.String("path", "", "path to pipestance")

func LookupCallInfo(basepath string) (int, string) {

	_, _, ast, err := core.Compile(basepath+"/_mrosource", []string{}, false)
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

	d := core.SearchPipestanceParams(ast, "sample_desc")
	if d == nil {
		panic("WTF3");
	}

	return res, d.(string)
}

func main() {
	c := sere2lib.Setup()
	//c.Dump()

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
	rr.SampleId, rr.Comments = LookupCallInfo(*flag_pipestance_path)
	rr.CellLine = "blah"
	rr.Project = project.Name
	rr.UserId = os.Getenv("USER")
	rr.FinishDate = sere2lib.GetPipestanceDate(*flag_pipestance_path);

	/*
	jsondata, err := ioutil.ReadFile(*flag_pipestance_path + "/" + project.SummaryJSONPath)
	if err != nil {
		panic(err)
	}
	*/

	//rr.SummaryJSON = string(jsondata)
	rr.TagsJSON= "{}"
	id, err :=c.InsertRecord("test_reports", rr)
	if (err != nil) {
		panic(err);
	}

	sere2lib.CheckinSummaries(c, id, *flag_pipestance_path);
	sere2lib.CheckinOne(c, id, *flag_pipestance_path + "/_perf", "_perf");
}
