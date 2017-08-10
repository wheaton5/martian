//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// Houston Zendesk downloader.
//

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"martian/core"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/10XDev/zego/zego"
	"github.com/dustin/go-humanize"
)

type ZendeskDownloadable struct {
	key    string
	url    string
	size   uint64
	time   time.Time
	ticket string
	user   string
}

func (self *ZendeskDownloadable) Size() uint64 {
	return self.size
}

func (self *ZendeskDownloadable) Key() string {
	return self.key
}

func (self *ZendeskDownloadable) Modified() time.Time {
	return self.time
}

func (self *ZendeskDownloadable) Ticket() string {
	return self.ticket
}

func (self *ZendeskDownloadable) User() string {
	return self.user
}

func (self *ZendeskDownloadable) Download(dstPath string) {
	// Setup the local file
	fd, err := os.Create(dstPath)
	if err != nil {
		core.LogError(err, "zendesk", "    Could not create file %s for download", dstPath)
		return
	}
	defer fd.Close()

	// Download file from Zendesk
	response, err := http.Get(self.url)
	if err != nil {
		core.LogError(err, "zendesk", "    HTTP GET failed")
		return
	}
	defer response.Body.Close()

	numBytes, err := io.Copy(fd, response.Body)
	if err != nil {
		core.LogError(err, "zendesk", "    Download stream copy failed")
		return
	}
	core.LogInfo("zendesk", "    Downloaded %s", humanize.Bytes(uint64(numBytes)))
}

type ZendeskDownloadSource struct {
	domain   string
	user     string
	apitoken string
}

func NewZendeskDownloadSource(domain string, user string, apitoken string) *ZendeskDownloadSource {
	self := &ZendeskDownloadSource{}
	self.domain = domain
	self.user = user
	self.apitoken = apitoken
	return self
}

func zenIsPipestance(a *zego.Attachment) bool {
	return a.ContentType == "application/x-gzip" &&
		(strings.HasSuffix(a.FileName, "debug.tgz") ||
			strings.HasSuffix(a.FileName, "mri.tgz"))
}

func (self *ZendeskDownloadSource) Enumerate() []Downloadable {
	auth := zego.Auth{self.user + "/token", self.apitoken, self.domain}

	// Search for tickets with attachments
	resource, err := auth.Search("type:ticket+has_attachment:true+status<closed")
	if err != nil {
		core.LogError(err, "zendesk", "Search failed")
		return []Downloadable{}
	}
	results := &zego.Search_Results_Tickets{}
	err = json.Unmarshal([]byte(resource.Raw), results)
	if err != nil {
		core.LogError(err, "zendesk", "Failed to deserialize search results.")
		return []Downloadable{}
	}
	core.LogInfo("zendesk", "Search returned %d tickets", results.Count)

	// Iterate over all returned objects
	downloadables := self.getPipestances(results.Results, auth)
	for results.NextPage != "" {
		resource, err := auth.NextSearchPage(results.NextPage)
		if err != nil {
			core.LogError(err, "zendesk", "Search failed")
			return downloadables
		}
		results = &zego.Search_Results_Tickets{}
		err = json.Unmarshal([]byte(resource.Raw), results)
		if err != nil {
			core.LogError(err, "zendesk", "Failed to deserialize search results.")
			return downloadables
		}

		downloadables = append(downloadables, self.getPipestances(results.Results, auth)...)
	}

	core.LogInfo("zendesk", "Search returned %d attachments", len(downloadables))
	return downloadables
}

func (self *ZendeskDownloadSource) getPipestances(results []*zego.Ticket, auth zego.Auth) []Downloadable {
	downloadables := []Downloadable{}
	for _, t := range results {
		ticket_id := strconv.FormatUint(t.Id, 10)

		// Get user info for this ticket's requester ID
		user, err := auth.ShowUser(fmt.Sprint(t.RequesterId))
		if err != nil {
			core.LogError(err, "zendesk", "Failed to find user %d", t.RequesterId)
			continue
		} else if user == nil || user.User == nil {
			core.LogInfo("zendesk", "Failed to find user %d", t.RequesterId)
			continue
		}

		// Extract email; skip tickets initiated by us
		email := user.User.Email
		if strings.HasSuffix(email, "10xgenomics.com") {
			continue
		}

		// Parse date
		date := t.CreatedAt
		godate, err := time.Parse(time.RFC3339, date)
		if err != nil {
			core.LogError(err, "zendesk", "Failed to parse date %s", date)
			continue
		}
		godate = godate.Local()
		y := godate.Year()
		m := godate.Month()
		d := godate.Day()

		comments, err := auth.ListComments(ticket_id)
		if err != nil {
			core.LogError(err, "zendesk", "ListComments failed for ticket %s", ticket_id)
			continue
		}
		for _, comment := range comments.Comments {
			if len(comment.Attachments) < 1 {
				continue
			}
			for _, a := range comment.Attachments {
				if zenIsPipestance(&a) {
					id := strconv.Itoa(a.Id)
					id = id[len(id)-6:]
					key := fmt.Sprintf("%04d-%02d-%02d-%s-%s-%s", y, m, d, email, id, a.FileName)
					url := a.ContentURL
					size := uint64(a.Size)
					downloadables = append(downloadables, &ZendeskDownloadable{
						key:    key,
						url:    url,
						size:   size,
						time:   godate,
						ticket: ticket_id,
						user:   email,
					})
				}
			}
		}
	}
	return downloadables
}
