package core

type ReportRecord struct {
	SummaryJSON     string
	SHA             string
	Branch          string
	Comments        string
	SampleId        int
	CellLine        string
	InterpretedJSON string
	Project         string
	UserId          string
}
