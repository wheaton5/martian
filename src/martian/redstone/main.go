//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// Martian Redstone cloud uploader.
//
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/dustin/go-humanize"
	"github.com/martian-lang/docopt.go"
)

//
// Structs for parsing AWS STS response JSON
//
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

//
// Struct for parsing Miramar's Redstone API response JSON
//
type RedstoneResponse struct {
	Error  string       `json:"error"`
	Region string       `json:"region"`
	Bucket string       `json:"bucket"`
	Key    string       `json:"key"`
	Sts    *AWSResponse `json:"sts"`
}

//
// Wrapper for os.File that tracks sequential read calls and outputs
// a wget-style progress bar to console.
//
type InstrumentedFile struct {
	file   *os.File
	bcount int64
	bsize  int64
	start  time.Time
	mutex  *sync.Mutex
	cancel bool
}

func InstrumentedOpen(name string) (*InstrumentedFile, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	// Seek to determine size of file.
	f.Seek(0, 1)
	bsize, _ := f.Seek(0, 2)
	f.Seek(0, 0)

	return &InstrumentedFile{
		file:   f,
		bcount: 0,
		bsize:  bsize,
		mutex:  &sync.Mutex{},
		start:  time.Now(),
		cancel: false,
	}, nil
}

func (self *InstrumentedFile) StartMonitor() {
	// Handle CTRL-C and kill.
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)
	signal.Notify(sigchan, syscall.SIGTERM)
	go func() {
		<-sigchan
		self.Cancel()
		self.Close()
	}()

	// Spawn goroutine to continuously update the console output.
	go func() {
		fmt.Println("Uploading file...")
		fmt.Printf("Size: %s bytes\n", humanize.Comma(self.bsize))
		self.start = time.Now()
		for {
			self.mutex.Lock()

			// Calculate progress
			elap := time.Since(self.start).Seconds()
			byps := float64(self.bcount) / elap
			mbps := float64(self.bcount*8) / elap / 1000000.0
			pctg := int64(float64(self.bcount) / float64(self.bsize) * 100.0)
			etas := int64(math.Max(0, math.Ceil(float64(self.bsize-self.bcount)/byps)))
			etam := int64(math.Max(0, float64(etas/60)))
			etas = etas % 60

			// Update progress display
			fmt.Print("\r")
			fmt.Printf("%3d%% [%-26s] %s%s %8.2fMb/s eta %dm %2ds  ",
				pctg, // percentage
				strings.Repeat("=", int(pctg/4))+">",                                                  // progress bar
				strings.Repeat(" ", len(humanize.Comma(self.bsize))-len(humanize.Comma(self.bcount))), // bytes sent left padding
				humanize.Comma(self.bcount),                                                           // bytes sent
				mbps, // bitrate
				etam, // eta min
				etas, // eta sec
			)

			self.mutex.Unlock()

			// Messages indicating completion or interruption.
			if pctg == 100 {
				fmt.Println("\r\nUpload complete! Cleaning up...")
				break
			}
			if self.cancel {
				fmt.Println("\r\nUpload interrupted. Cleaning up...")
				break
			}
			time.Sleep(time.Second * time.Duration(1))
		}
	}()
}

func (self *InstrumentedFile) Seek(offset int64, whence int) (int64, error) {
	self.mutex.Lock()
	n, err := self.file.Seek(offset, whence)
	self.mutex.Unlock()
	return n, err
}

func (self *InstrumentedFile) Read(p []byte) (int, error) {
	self.mutex.Lock()
	n, err := self.file.Read(p)
	self.bcount += int64(n)
	self.mutex.Unlock()
	return n, err
}

func (self *InstrumentedFile) Cancel() {
	self.mutex.Lock()
	self.cancel = true
	self.mutex.Unlock()
}

func (self *InstrumentedFile) Close() error {
	self.mutex.Lock()
	err := self.file.Close()
	self.mutex.Unlock()
	return err
}

//
// Encryption routines for Miramar's Redstone API
//
func decrypt(encodedpayload []byte) []byte {
	// AES-128 (128 bit/16 byte block size and IV length)
	// IV is assumed to precede payload
	// Ciphertext is assumed to be PKCS padded and base64 encoded
	key := []byte{0xfe, 0xfa, 0xf0, 0xfc, 0xef, 0xaf, 0x0f, 0xcf, 0xfe, 0xfa, 0xf0, 0xfc, 0xef, 0xaf, 0x0f, 0xcf}
	algo, _ := aes.NewCipher(key)

	payload, _ := base64.StdEncoding.DecodeString(string(encodedpayload))
	iv := payload[:algo.BlockSize()]
	ciphertext := payload[algo.BlockSize():]

	cipher.NewCBCDecrypter(algo, iv).CryptBlocks(ciphertext, ciphertext)

	return pkcsUnpad(ciphertext, algo.BlockSize())
}

func pkcsUnpad(data []byte, blocklen int) []byte {
	padlen := int(data[len(data)-1])
	if padlen > blocklen || padlen == 0 {
		return nil
	}
	pad := data[len(data)-padlen:]
	for i := 0; i < padlen; i++ {
		if pad[i] != byte(padlen) {
			return nil
		}
	}
	return data[:len(data)-padlen]
}

//
// Main routine
//
var GET_UPLOAD_INFO_FAIL_MSG = "Could not contact http://software.10xgenomics.com (%s)\n  Please make sure you have Internet connectivity and try again,\n  or contact software@10xgenomics.com for help.\n"

func main() {
	doc := `Martian Redstone cloud uploader.

Usage:
    redstone --from=EMAIL --to=EMAIL <file> [options]
    redstone -h | --help | --version

Options:
    --concurrency=<num>     Number of concurrent upload streams.
                              Defaults to 10. 

    -h --help               Show this message.
    --version               Show version.`
	version := "2.1.0"
	opts, _ := docopt.Parse(doc, nil, true, version, false)
	fpath := opts["<file>"].(string)
	concurrency := 0
	if value := opts["--concurrency"]; value != nil {
		if value, err := strconv.Atoi(value.(string)); err == nil {
			if value > 0 {
				concurrency = value
			}
		}
	}
	frEmail := opts["--from"].(string)
	toEmail := opts["--to"].(string)

	// Prep runtime values to pass to Miramar.
	parameters := url.Values{}
	parameters.Add("frEmail", frEmail)
	parameters.Add("toEmail", toEmail)
	parameters.Add("fname", path.Base(fpath))
	parameters.Add("version", version)
	parameters.Encode()

	// Call Miramar Redstone API.
	response, err := http.Get("http://software.10xgenomics.com/redstone.json?" + parameters.Encode())
	defer response.Body.Close()
	if err != nil {
		fmt.Printf(GET_UPLOAD_INFO_FAIL_MSG, err)
		return
	}
	if response.StatusCode != 200 {
		fmt.Printf(GET_UPLOAD_INFO_FAIL_MSG, response.Status)
		return
	}

	// Decrypt and parse response.
	var rr *RedstoneResponse
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Printf(GET_UPLOAD_INFO_FAIL_MSG, err)
		return
	}
	if err := json.Unmarshal(decrypt(body), &rr); err != nil {
		fmt.Printf(GET_UPLOAD_INFO_FAIL_MSG, err)
		return
	}
	if len(rr.Error) > 0 {
		fmt.Println("Error: " + rr.Error)
		return
	}

	// Set S3 credentials based on Miramar response.
	sess := session.New(&aws.Config{
		Region: aws.String(rr.Region),
		Credentials: credentials.NewStaticCredentials(
			rr.Sts.Credentials.Access_key_id,
			rr.Sts.Credentials.Secret_access_key,
			rr.Sts.Credentials.Session_token),
	})

	// Create multi-stream uploader with default options.
	uploader := s3manager.NewUploader(sess, func(u *s3manager.Uploader) {
		u.PartSize = 0
		u.Concurrency = concurrency
		u.LeavePartsOnError = false
	})

	// Open the wrapped file.
	f, err := InstrumentedOpen(fpath)
	if err != nil {
		fmt.Println("Error opening file:\n", err)
		return
	}
	defer f.Close()

	// Start the progress monitoring.
	f.StartMonitor()

	// Initiate the multi-stream upload.
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: &rr.Bucket,
		Key:    &rr.Key,
		Body:   f,
		Metadata: map[string]*string{
			"from-email": &frEmail,
			"to-email":   &toEmail,
		},
	})

	// If the upload died (not due to CTRL-C or kill), then report the error.
	if err != nil && f.cancel == false {
		fmt.Println("\n\nUpload failed with error:\n", err)
	}

	// Give time for the monitor goroutine to report final status.
	time.Sleep(time.Second * time.Duration(3))
}
