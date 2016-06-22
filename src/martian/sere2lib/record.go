package sere2lib

import (
	"time"
)

type ReportRecord struct {
	SHA        string
	FinishDate time.Time
	Branch     string
	Comments   string
	SampleId   int
	CellLine   string
	TagsJSON   string
	Project    string
	UserId     string
}

type ReportSummaryFile struct {
	ReportRecordId int
	SummaryJSON    string
	StageName      string
}
