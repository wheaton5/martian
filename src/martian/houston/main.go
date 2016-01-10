package main

import (
	"martian/core"
	"martian/manager"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/10XDev/docopt.go"
)

func main() {
	core.SetupSignalHandlers()
	doc := `Houston.

Usage:
    houston
    houston -h | --help | --version

Options:
    -h --help       Show this message.
    --version       Show version.`
	martianVersion := core.GetVersion()
	docopt.Parse(doc, nil, true, martianVersion, false)

	env := core.EnvRequire([][]string{
		{"HOUSTON_PORT", ">2000"},
		{"HOUSTON_INSTANCE_NAME", "displayed_in_ui"},
		{"HOUSTON_HOSTNAME", "fqdn_plus_port_optionally"},
		{"HOUSTON_AMAZONS3_BUCKET", "s3_bucket"},
		{"HOUSTON_ZENDESK_DOMAIN", "zendesk_domain"},
		{"HOUSTON_ZENDESK_USER", "zendesk_user"},
		{"HOUSTON_ZENDESK_APITOKEN", "zendesk_api_token"},
		{"HOUSTON_CACHE_PATH", "path/to/houston/cache"},
		{"HOUSTON_DOWNLOAD_INTERVALMIN", "integer_in_minutes"},
		{"HOUSTON_DOWNLOAD_PATH", "path/to/houston/downloads"},
		{"HOUSTON_DOWNLOAD_MAXMB", "integer_in_megabytes"},
		{"HOUSTON_PIPESTANCE_SUMMARY_PATHS", "comma_separated_paths"},
		{"HOUSTON_LOGS_PATH", "path/to/houston/logs"},
		{"HOUSTON_FILES_PATH", "path/to/houston/files"},
		{"HOUSTON_EMAIL_HOST", "smtp.server.local"},
		{"HOUSTON_EMAIL_SENDER", "email@address.com"},
		{"HOUSTON_EMAIL_RECIPIENT", "email@address.com"},
	}, true)

	core.LogTee(path.Join(env["HOUSTON_LOGS_PATH"],
		time.Now().Format("20060102150405")+".log"))

	uiport := env["HOUSTON_PORT"]
	hostname := env["HOUSTON_HOSTNAME"]
	instanceName := env["HOUSTON_INSTANCE_NAME"]
	amazonS3Bucket := env["HOUSTON_AMAZONS3_BUCKET"]
	zendeskDomain := env["HOUSTON_ZENDESK_DOMAIN"]
	zendeskUser := env["HOUSTON_ZENDESK_USER"]
	zendeskApiToken := env["HOUSTON_ZENDESK_APITOKEN"]
	cachePath := env["HOUSTON_CACHE_PATH"]
	downloadPath := env["HOUSTON_DOWNLOAD_PATH"]
	downloadIntervalMin, err := strconv.Atoi(env["HOUSTON_DOWNLOAD_INTERVALMIN"])
	if err != nil {
		core.LogError(err, "Could not parse HOUSTON_DOWNLOAD_INTERVALMIN value %s",
			env["HOUSTON_DOWNLOAD_INTERVALMIN"])
		os.Exit(1)
	}
	downloadMaxMB, err := strconv.Atoi(env["HOUSTON_DOWNLOAD_MAXMB"])
	if err != nil {
		core.LogError(err, "Could not parse HOUSTON_DOWNLOAD_MAXMB value %s",
			env["HOUSTON_DOWNLOAD_MAXMB"])
		os.Exit(1)
	}
	pipestanceSummaryPaths := strings.Split(env["HOUSTON_PIPESTANCE_SUMMARY_PATHS"], ",")
	filesPath := env["HOUSTON_FILES_PATH"]
	emailHost := env["HOUSTON_EMAIL_HOST"]
	emailSender := env["HOUSTON_EMAIL_SENDER"]
	emailRecipient := env["HOUSTON_EMAIL_RECIPIENT"]

	// Mailer
	mailer := manager.NewMailer(instanceName, emailHost, emailSender,
		emailRecipient, false)

	// Runtime
	rt := core.NewRuntime("local", "disable", "disable", martianVersion)

	// SubmissionManager
	sman := NewSubmissionManager(hostname, instanceName, filesPath, cachePath,
		pipestanceSummaryPaths, rt, mailer)

	// Downloader
	dman := NewDownloadManager(downloadPath, downloadIntervalMin,
		downloadMaxMB, filesPath, sman)
	dman.AddDownloadSource(NewZendeskDownloadSource(zendeskDomain, zendeskUser, zendeskApiToken))
	dman.AddDownloadSource(NewAmazonS3DownloadSource(amazonS3Bucket))
	dman.StartDownloadLoop()

	// Run web server.
	go runWebServer(uiport, martianVersion, sman)

	// Let daemons take over.
	done := make(chan bool)
	<-done
}
