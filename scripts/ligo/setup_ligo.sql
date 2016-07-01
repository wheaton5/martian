\connect postgres
DROP database ligo;
CREATE database ligo;
\connect ligo;
CREATE TABLE test_reports(
	id    	SERIAL PRIMARY KEY,
	SHA   	VARCHAR(80) NOT NULL,
	Branch	VARCHAR(80) NOT NULL,
	SampleId VARCHAR(80) NOT NULL,
	Comments TEXT NOT NULL,
	UserId CARCHAR(80) NOT NULL,
	SampleDefHash VARCHAR(80) NOT NULL,
	FinishDate TIMESTAMP NOT NULL,
	Project VARCHAR(80) NOT NULL,
	Success BOOLEAN NOT NULL);

CREATE TABLE test_report_summaries(
	id 	SERIAL PRIMARY KEY,
	ReportRecordId INTEGER NOT NULL,
	StageName VARCHAR(80) NOT NULL,
	SummaryJSON JSONB NOT NULL);


CREATE INDEX ON test_report_summaries (ReportRecordId, StageName);
CREATE INDEX ON test_reports (SampleId);
CREATE INDEX ON test_reports (FinishDate);
CREATE INDEX ON test_reports (Project);
CREATE INDEX ON test_report_summaries USING gin (SummaryJSON);
	
CREATE USER x10user WITH password 'v3rys3cr3t';

GRANT ALL ON TABLE test_report_summaries to x10user;
GRANT ALL ON SEQUENCE test_report_summaries_id_seq to x10user;
GRANT ALL ON TABLE test_reports to x10user;
GRANT ALL ON SEQUENCE test_reports_id_seq to x10user;
