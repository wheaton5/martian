//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// Houston Amazon S3 downloader.
//

package main

import (
	"martian/core"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/dustin/go-humanize"
)

type AmazonS3Downloadable struct {
	bucket string
	object *s3.Object
}

func (self *AmazonS3Downloadable) Size() uint64 {
	return uint64(*self.object.Size)
}

func (self *AmazonS3Downloadable) Key() string {
	return *self.object.Key
}

func (self *AmazonS3Downloadable) Download(dstPath string) {
	// Setup the local file
	fd, err := os.Create(dstPath)
	if err != nil {
		core.LogError(err, "amzons3", "    Could not create file for download")
		return
	}
	defer fd.Close()

	// Download file from S3
	numBytes, err := s3manager.NewDownloader(nil).Download(fd,
		&s3.GetObjectInput{Bucket: &self.bucket, Key: self.object.Key})
	if err != nil {
		core.LogError(err, "amzons3", "    Download failed")
		return
	}
	core.LogInfo("amzons3", "    Downloaded %s", humanize.Bytes(uint64(numBytes)))

}

type AmazonS3DownloadSource struct {
	bucket string
}

func NewAmazonS3DownloadSource(bucket string) *AmazonS3DownloadSource {
	self := &AmazonS3DownloadSource{}
	self.bucket = bucket
	return self
}

func (self *AmazonS3DownloadSource) Enumerate() []Downloadable {
	// ListObjects in our bucket that start with "2" - XXX FIXME: Y3K bug
	response, err := s3.New(nil).ListObjects(&s3.ListObjectsInput{
		Bucket: aws.String(self.bucket),
		Prefix: aws.String("2"),
	})
	if err != nil {
		core.LogError(err, "amzons3", "ListObjects failed")
		return []Downloadable{}
	}
	core.LogInfo("amzons3", "ListObjects returned %d objects", len(response.Contents))

	// Iterate over all returned objects
	downloadables := []Downloadable{}
	for _, object := range response.Contents {
		downloadables = append(downloadables, &AmazonS3Downloadable{bucket: self.bucket, object: object})
	}
	return downloadables
}
