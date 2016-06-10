// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package main

import (
	"core"
	"flag"
	"io/ioutil"
)

var flag_json_summary_path = flag.String("summary", "", "path to summary.json file")
var flag_git_sha = flag.String("sha", "", "git sha")
var flag_git_branch = flag.String("branch", "", "git branch")
var flag_comment_path = flag.String("comment", "", "path to comments file")
var flag_cell_line = flag.String("line", "", "cell line id")
var flag_sample_id = flag.Int("sample", 0, "sample id")
var flag_project = flag.String("project", "", "project name")
var flag_user = flag.String("user", "", "user name")

func main() {
	c := core.Setup()
	c.Dump()

	var rr core.ReportRecord

	flag.Parse()

	if *flag_json_summary_path == "" ||
		*flag_git_sha == "" ||
		*flag_git_branch == "" ||
		*flag_sample_id == 0 ||
		*flag_user == "" ||
		*flag_project == "" ||
		*flag_cell_line == "" {
		panic("bad pargs")
	}

	rr.SHA = *flag_git_sha
	rr.Branch = *flag_git_branch
	rr.SampleId = *flag_sample_id
	rr.CellLine = *flag_cell_line
	rr.Project = *flag_project
	rr.UserId = *flag_user

	jsondata, err := ioutil.ReadFile(*flag_json_summary_path)
	if err != nil {
		panic(err)
	}

	rr.SummaryJSON = string(jsondata)
	rr.Comments = "{}"
	rr.InterpretedJSON = "{}"
	c.InsertRecord("test_reports", rr)
}
