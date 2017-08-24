//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// Houston Amazon S3 downloader.
//

package main

import (
	"martian/util"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/dustin/go-humanize"
)

type AmazonS3Downloadable struct {
	bucket string
	object *s3.Object
	sess   *session.Session
}

func (self *AmazonS3Downloadable) Size() uint64 {
	return uint64(*self.object.Size)
}

func (self *AmazonS3Downloadable) Key() string {
	return *self.object.Key
}

func (self *AmazonS3Downloadable) Modified() time.Time {
	return *self.object.LastModified
}

func (self *AmazonS3Downloadable) Ticket() string {
	return ""
}

func (self *AmazonS3Downloadable) Download(dstPath string) {
	// Setup the local file
	fd, err := os.Create(dstPath)
	if err != nil {
		util.LogError(err, "amzons3", "    Could not create file %s for download", dstPath)
		return
	}
	defer fd.Close()

	// Download file from S3
	numBytes, err := s3manager.NewDownloader(self.sess).Download(fd,
		&s3.GetObjectInput{Bucket: &self.bucket, Key: self.object.Key})
	if err != nil {
		util.LogError(err, "amzons3", "    Download failed for bucket %s key %s",
			self.bucket, *self.object.Key)
		return
	}
	util.LogInfo("amzons3", "    Downloaded %s", humanize.Bytes(uint64(numBytes)))

}

type AmazonS3DownloadSource struct {
	bucket string
	sess   *session.Session
}

func NewAmazonS3DownloadSource(bucket string) *AmazonS3DownloadSource {
	self := &AmazonS3DownloadSource{}
	self.bucket = bucket
	self.sess = session.New()
	return self
}

func (self *AmazonS3DownloadSource) Enumerate() []Downloadable {
	// ListObjects in our bucket that start with "2" - XXX FIXME: Y3K bug
	downloadables := []Downloadable{}
	err := s3.New(self.sess).ListObjectsPages(&s3.ListObjectsInput{
		Bucket: aws.String(self.bucket),
		Prefix: aws.String("2"),
	}, func(response *s3.ListObjectsOutput, lastPage bool) bool {
		for _, object := range response.Contents {
			downloadables = append(downloadables, &AmazonS3Downloadable{
				bucket: self.bucket,
				object: object,
				sess:   self.sess,
			})
		}
		return true
	})
	if err != nil {
		util.LogError(err, "amzons3", "ListObjects failed")
		return downloadables
	}
	util.LogInfo("amzons3", "ListObjects returned %d objects", len(downloadables))
	return downloadables
}