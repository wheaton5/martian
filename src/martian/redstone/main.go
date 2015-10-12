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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/docopt/docopt.go"
	"github.com/dustin/go-humanize"
	"io/ioutil"
	"martian/core"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"
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

type RedstoneResponse struct {
	Error  string       `json:"error"`
	Region string       `json:"region"`
	Bucket string       `json:"bucket"`
	Key    string       `json:"key"`
	Sts    *AWSResponse `json:"sts"`
}

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
				fmt.Println("\r\nUpload complete!")
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

func decrypt(encodedpayload []byte) []byte {
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

func main() {
	doc := `Martian Redstone Uploader.

Usage:
    redstone <your_email> <file> 
    redstone -h | --help | --version

Options:
    -h --help     Show this message.
    --version     Show version.`
	martianVersion := core.GetVersion()
	opts, _ := docopt.Parse(doc, nil, true, martianVersion, false)

	email := opts["<your_email>"].(string)
	fpath := opts["<file>"].(string)

	parameters := url.Values{}
	parameters.Add("email", email)
	parameters.Add("fname", path.Base(fpath))
	parameters.Encode()

	response, err := http.Get("http://localhost:3000/redstone.json?" + parameters.Encode())
	if err != nil {
		fmt.Println("HTTP GET failed", err)
		return
	}
	defer response.Body.Close()

	var rr *RedstoneResponse
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}
	if err := json.Unmarshal(decrypt(body), &rr); err != nil {
		fmt.Println("Could not parse aws response", err)
		return
	}
	if len(rr.Error) > 0 {
		fmt.Println(rr.Error)
		return
	}

	defaults.DefaultConfig.Region = aws.String(rr.Region)
	defaults.DefaultConfig.Credentials = credentials.NewStaticCredentials(
		rr.Sts.Credentials.Access_key_id,
		rr.Sts.Credentials.Secret_access_key,
		rr.Sts.Credentials.Session_token)

	uploader := s3manager.NewUploader(&s3manager.UploadOptions{
		PartSize:          0,
		Concurrency:       0,
		LeavePartsOnError: false,
		S3:                nil,
	})

	f, err := InstrumentedOpen(fpath)
	if err != nil {
		fmt.Println("Error opening file:\n", err)
		return
	}
	defer f.Close()

	f.StartMonitor()
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: &rr.Bucket,
		Key:    &rr.Key,
		Body:   f,
	})
	if err != nil && f.cancel == false {
		fmt.Println("\n\nUpload failed with error:\n", err)
	}
	time.Sleep(time.Second * time.Duration(3))
}
