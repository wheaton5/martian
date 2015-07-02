//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

type MarsocManager struct {
	downloadUrl string
}

type Sample struct {
	SampleBag     interface{}       `json:"sample_bag"`
	FastqPaths    map[string]string `json:"fastq_paths"`
	ReadyToInvoke bool              `json:"ready_to_invoke"`
}

func NewMarsocManager(downloadUrl string) *MarsocManager {
	self := &MarsocManager{}
	self.downloadUrl = strings.TrimRight(downloadUrl, "/")
	return self
}

func (self *MarsocManager) GetSample(id int) (*Sample, error) {
	downloadUrl := self.downloadUrl + "/" + strconv.Itoa(id)
	res, err := http.Get(downloadUrl)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var sample *Sample
	if err := json.Unmarshal(body, &sample); err != nil {
		return nil, err
	}

	return sample, nil
}
