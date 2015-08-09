//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// Houston AWS S3 downloader.
//

package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func runs3(dlPath string, stPath string) {
	svc := s3.New(nil)
	result, err := svc.ListObjects(&s3.ListObjectsInput{
		Bucket: aws.String("10x.uploads"),
		Prefix: aws.String("2"),
	})
	if err != nil {
		fmt.Println("Failed to list objects", err)
		return
	}

	re := regexp.MustCompile("^(\\d{4}-\\d{2}-\\d{2})-(.*@.*)-([A-Z0-9]{5,6})-(.*)$")
	//re_debug := regexp.MustCompile("\\.debug\\.tgz$")

	for _, object := range result.Contents {
		res := re.FindStringSubmatch(*object.Key)
		if len(res) != 5 {
			continue
		}

		dparts := strings.Split(res[1], "-")
		if len(dparts) != 3 {
			continue
		}
		year := dparts[0]
		month := dparts[1]
		day := dparts[2]

		eparts := strings.Split(res[2], "@")
		if len(eparts) != 2 {
			continue
		}
		uname := eparts[0]
		domain := eparts[1]

		uid := res[3]

		fname := res[4]
		fdir := fmt.Sprintf("%s-%s", uid, fname)

		key := *object.Key
		size := *object.Size

		// Permanent storage path
		permPath := path.Join(stPath, year, month, day, domain, uname, fdir)

		// Skip further processing if already exists
		if _, err := os.Stat(permPath); err == nil {
			fmt.Printf("Already exists: %s\n", permPath)
			continue
		}

		// Skip further processing if error making directory
		if err := os.MkdirAll(permPath, 0755); err != nil {
			fmt.Printf("Error making folder: %s\n", permPath)
			continue
		}

		//fmt.Printf("%s %d\n", *object.Key, *object.Size)
		//isdebug := re_debug.MatchString(*object.Key)
		if size < 1000000 {
			fmt.Printf("Created: %s\n", permPath)
			downloadToFile(dlPath, key, permPath)
		}
		//fmt.Printf("%s %s %v %s\n", path.Ext(fname), domain, res[1:], *object.ETag)
	}
}

func downloadToFile(dlPath string, key string, permPath string) {
	file := path.Join(dlPath, key)

	// Setup the local file
	fd, err := os.Create(file)
	if err != nil {
		panic(err)
	}

	downloadBucket := "10x.uploads"

	fmt.Printf("Downloading...\n")
	downloader := s3manager.NewDownloader(nil)
	_, err = downloader.Download(fd,
		&s3.GetObjectInput{
			Bucket: &downloadBucket,
			Key:    &key,
		})

	fd.Seek(0, 0)
	var magic []byte
	magic = make([]byte, 512)
	nbytes, err := fd.Read(magic)
	fd.Close()
	if nbytes != 512 || err != nil {
		fmt.Printf("Could not read downloaded file.\n")
	} else {
		mimeType := http.DetectContentType(magic)
		if strings.HasPrefix(mimeType, "application/x-gzip") {
			fmt.Printf("Type is tarball, untar'ing to permPath\n")
			_, err := exec.Command("tar", "xf", file, "-C", permPath).Output()
			if err != nil {
				fmt.Printf("%v\n", err)
			}
		} else if strings.HasPrefix(mimeType, "text/plain") {
			fmt.Printf("Type is text, copying to permPath\n")
			_, err := exec.Command("cp", file, permPath).Output()
			if err != nil {
				fmt.Printf("%v\n", err)
			}
		} else {
			fmt.Printf("UNKNOWN filetype\n")
		}
	}
}
