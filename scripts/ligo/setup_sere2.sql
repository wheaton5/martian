\connect postgres
drop database ligo;
create database ligo;
\connect ligo;
create table test_reports(
	id    	SERIAL PRIMARY KEY,
	SHA   	varchar(80) NOT NULL,
	Branch	varchar(80) NOT NULL,
	SampleId varchar(80) NOT NULL,
	Comments text NOT NULL,
	UserId varchar(80) NOT NULL,
	SampleDefHash varchar(80) NOT NULL,
	FinishDate timestamp NOT NULL,
	Project varchar(80) NOT NULL,
	Success boolean NOT NULL);

create table test_report_summaries(
	id 	SERIAL PRIMARY KEY,
	ReportRecordId integer NOT NULL,
	StageName varchar(80) NOT NULL,
	SummaryJSON jsonb NOT NULL);


create index on test_report_summaries (ReportRecordId, StageName);
create index on test_reports (SampleId);
create index on test_reports (FinishDate);
create index on test_reports (Project);
create index on test_report_summaries USING gin (SummaryJSON);
	
create user x10user with password 'v3rys3cr3t';

grant all on table test_report_summaries to x10user;
grant all on sequence test_report_summaries_id_seq to x10user;
grant all on table test_reports to x10user;
grant all on sequence test_reports_id_seq to x10user;
