package ligolib

import (
	"time"
)

type ReportRecord struct {
	ID            int `sql:"RO"`
	SHA           string
	FinishDate    time.Time
	Branch        string
	Comments      string
	SampleId      string
	SampleDefHash string
	Project       string
	UserId        string
}

type ReportSummaryFile struct {
	ID             int `sql:"RO"`
	ReportRecordId int
	SummaryJSON    string
	StageName      string
}
