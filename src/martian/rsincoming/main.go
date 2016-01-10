//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// Martian Redstone cloud uploader.
//
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/10XDev/docopt.go"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dustin/go-humanize"
)

func main() {
	doc := `Martian Redstone Incoming Monitor

Usage:
    rsincoming [options]
    rsincoming -h | --help | --version

Options:
    --stale    Report stale transfers.

    -h --help  Show this message.
    --version  Show version.`
	version := "1.0.0"
	opts, _ := docopt.Parse(doc, nil, true, version, false)
	stale := opts["--stale"].(bool)

	bucket := "10x.redstone"
	region := "us-west-2"
	staleHoursThreshold := 24 * 14 // transfer considered stale after two weeks

	svc := s3.New(session.New(&aws.Config{Region: aws.String(region)}))

	// Iterate through all multipart uploads
	resp, err := svc.ListMultipartUploads(&s3.ListMultipartUploadsInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	i := 1
	for _, u := range resp.Uploads {
		// Parse initiation time
		age := humanize.Time(*u.Initiated)
		hoursAge := int(time.Since(*u.Initiated) / time.Hour)

		// Only show stale if --stale, otherwise only show non-stale
		if (!stale && hoursAge >= staleHoursThreshold) || (stale && hoursAge < staleHoursThreshold) {
			continue
		}

		// Parse the key
		key := *u.Key
		keyParts := strings.Split(key, "-")
		year := keyParts[0]
		month := keyParts[1]
		day := keyParts[2]
		email := keyParts[3]
		file := keyParts[5]

		uploadId := *u.UploadId

		// Iterate through paginated part data
		marker := int64(0)
		partcount := 0
		totalsize := int64(0)
		for {
			params := &s3.ListPartsInput{
				Bucket:           aws.String(bucket),
				Key:              aws.String(key),
				UploadId:         aws.String(uploadId),
				PartNumberMarker: aws.Int64(marker),
			}
			resp, err := svc.ListParts(params)
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			// Iterate through individual parts
			partcount += len(resp.Parts)
			for _, p := range resp.Parts {
				totalsize += *p.Size
			}
			// Advance to the next page
			marker = *resp.NextPartNumberMarker

			// Stop if this is the last page
			if !*resp.IsTruncated {
				break
			}
		}

		// Calculate total and parts sizes
		size := humanize.Bytes(uint64(totalsize))
		partsize := int64(0)
		if partcount > 0 {
			partsize = totalsize / int64(partcount)
		}
		partCount := humanize.Comma(int64(partcount))
		partSize := humanize.Bytes(uint64(partsize))

		// Calculate throughput
		MBps := totalsize / int64(time.Since(*u.Initiated)/time.Second)
		sMBps := humanize.Bytes(uint64(MBps))
		Mbps := int64(MBps * 8 / (1000 * 1000))

		// Output upload info
		fmt.Printf("Index:    %d\nSender:   %s\nFile:     %s\nAge:      %s (%s-%s-%s)\nReceived: %s (%s parts @ %s)\nThruput:  %dMbps (%s/s)\n", i, email, file, age, year, month, day, size, partCount, partSize, Mbps, sMBps)
		if stale {
			// Only output CLI options if stale (for use with aws s3api abort-multipart-upload)
			fmt.Printf("CLI:      --bucket %s --key %s --upload-id %s\n", bucket, key, uploadId)
		}
		fmt.Println("")

		i += 1
	}
}
