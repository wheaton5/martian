package sere2lib

type ReportRecord struct {
	SHA             string
	Branch          string
	Comments        string
	SampleId        int
	CellLine        string
	TagsJSON	string
	Project         string
	UserId          string
}

type ReportSummaryFile struct {
	ReportRecordId int
	SummaryJSON string
	StageName string
}
