//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// Martian Redstone cloud uploader.
//
package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/docopt/docopt.go"
	"io/ioutil"
	"martian/core"
	"net/http"
	"os"
)

type ResponseMetadata struct {
	Request_id string `json:"RequestId"`
}

type Credentials struct {
	Access_key_id     string `json:"AccessKeyId"`
	Secret_access_key string `json:"SecretAccessKey"`
	Session_token     string `json:"SessionToken"`
	Expiration        string `json:"Expiration"`
}

type AWSResponse struct {
	Response_metadata *ResponseMetadata `json:"ResponseMetadata"`
	Credentials       *Credentials      `json:"Credentials"`
}

func main() {
	doc := `Martian Redstone Uploader.

Usage:
    redstone <file> 
    redstone -h | --help | --version

Options:
    -h --help     Show this message.
    --version     Show version.`
	martianVersion := core.GetVersion()
	opts, _ := docopt.Parse(doc, nil, true, martianVersion, false)

	fname := opts["<file>"].(string)

	file, err := os.Open(fname)
	if err != nil {
		fmt.Println("Failed opening file", fname, err)
		return
	}
	defer file.Close()

	bucket := "10x.uploads"
	key := "blah"

	response, err := http.Get("http://localhost:3000/sts.json")
	if err != nil {
		fmt.Println("HTTP GET failed", err)
		return
	}
	defer response.Body.Close()

	var awsresp *AWSResponse
	if b, err := ioutil.ReadAll(response.Body); err != nil {
		return
	} else {
		if err := json.Unmarshal(b, &awsresp); err != nil {
			fmt.Println("Could not parse aws response", err)
			return
		}
	}

	defaults.DefaultConfig.Region = aws.String("us-west-2")
	defaults.DefaultConfig.Credentials = credentials.NewStaticCredentials(
		awsresp.Credentials.Access_key_id,
		awsresp.Credentials.Secret_access_key,
		awsresp.Credentials.Session_token)

	uploader := s3manager.NewUploader(&s3manager.UploadOptions{
		PartSize:          5 * 1024 * 1024,
		Concurrency:       10,
		LeavePartsOnError: false,
		S3:                nil,
	})
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   file,
	})
	if err != nil {
		fmt.Println("Failed to upload", err)
		return
	}
}
