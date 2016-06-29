\connect postgres
drop database sere3;
create database sere3;
\connect sere3;
create table test_reports(
	id    	SERIAL PRIMARY KEY,
	SHA   	varchar(80) NOT NULL,
	Branch	varchar(80) NOT NULL,
	SampleId varchar(80) NOT NULL,
	Comments text NOT NULL,
	UserId varchar(80) NOT NULL,
	SampleDefHash varchar(80) NOT NULL,
	FinishDate timestamp NOT NULL,
	Project varchar(80) NOT NULL);

create table test_report_summaries(
	id 	SERIAL PRIMARY KEY,
	ReportRecordId integer NOT NULL,
	StageName varchar(80) NOT NULL,
	SummaryJSON jsonb NOT NULL);


	
create user x10user with password 'v3rys3cr3t';

grant all on table test_report_summaries to x10user;
grant all on sequence test_report_summaries_id_seq to x10user;
grant all on table test_reports to x10user;
grant all on sequence test_reports_id_seq to x10user;
