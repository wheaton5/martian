//
// Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
//
// Marsoc LENA API wrapper.
//
package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
)

type Lena struct {
	apiUrl    string
	authToken string
	path      string
	cache     map[string][]interface{}
}

func NewLena(apiUrl string, authToken string, cachePath string) *Lena {
	self := &Lena{}
	self.apiUrl = apiUrl
	self.authToken = authToken
	self.path = path.Join(cachePath, "lena")
	self.cache = map[string][]interface{}{}
	return self
}

type Oligo struct {
	Id    int    `json:"id"`
	State string `json:"state"`
	Name  string `json:"name"`
	Seq   string `json:"seq"`
}

type Genome struct {
	Id     int     `json:"id"`
	Name   string  `json:"name"`
	A_freq float32 `json:"a_freq"`
	C_freq float32 `json:"c_freq"`
	G_freq float32 `json:"g_freq"`
	T_freq float32 `json:"t_freq"`
}

type TargetSet struct {
	Id     int     `json:"id"`
	State  int     `json:"state"`
	Name   string  `json:"name"`
	Genome int     `json:"genome"`
	A_freq float32 `json:"a_freq"`
	C_freq float32 `json:"c_freq"`
	G_freq float32 `json:"g_freq"`
	T_freq float32 `json:"t_freq"`
}

type Workflow struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type BarcodeSet struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type SequencingRun struct {
	Id                    int     `json:"id"`
	State                 string  `json:"state"`
	Name                  string  `json:"name"`
	Date                  string  `json:"date"`
	Loading_concentration float32 `json:"loading_concentration"`
	Failure_reason        string  `json:"failure_reason"`
	Samples               []int   `json:"samples"`
}

type User struct {
	Id       int    `json:"id"`
	Username string `json:"username"`
}

type Sample struct {
	Id                       int            `json:"id"`
	Description              string         `json:"description"`
	Name                     string         `json:"name"`
	State                    string         `json:"state"`
	Genome                   *Genome        `json:"genome"`
	Target_set               *TargetSet     `json:"target_set"`
	Sample_indexes           []*Oligo       `json:"sample_indexes"`
	Primers                  []*Oligo       `json:"primers"`
	Workflow                 *Workflow      `json:"workflow"`
	Sequencing_run           *SequencingRun `json:"sequencing_run"`
	Degenerate_primer_length int            `json:"degenerate_primer_length"`
	Barcode_set              *BarcodeSet    `json:"barcode_set"`
	Template_input_mass      float32        `json:"template_input_mass"`
	User                     *User          `json:"user"`
	Lane                     interface{}    `json:"lane"`
}

func (self *Lena) loadDatabase() {
	dbPath := "./nice.json"
	bytes, err := ioutil.ReadFile(dbPath)
	if err != nil {
		logError(err, "LENAAPI", "Could not read cache file %s.", dbPath)
		return
	}

	var samples []*Sample
	if err := json.Unmarshal(bytes, &samples); err != nil {
		logError(err, "LENAAPI", "Could not parse JSON in cache file %s.", dbPath)
		return
	}
	for _, sample := range samples {
		fmt.Println(sample.Id, sample.User.Username)
	}
	fmt.Println(len(samples))
}

func (self *Lena) loadCache() {
	// Iterate through files in the cache folder.
	paths, _ := filepath.Glob(path.Join(self.path, "*"))
	for _, p := range paths {
		// Each one contains the Lena API response for a single flowcell.
		// The response is a JSON list of sample objects.
		bytes, err := ioutil.ReadFile(p)
		if err != nil {
			logError(err, "LENAAPI", "Could not read cache file %s.", p)
			continue
		}
		var v interface{}
		if err := json.Unmarshal(bytes, &v); err != nil {
			logError(err, "LENAAPI", "Could not parse JSON in cache file %s.", p)
			continue
		}

		// Put the JSON object into the cache by flowcell id.
		fcid := path.Base(p)
		self.cache[fcid] = v.([]interface{})
	}
	logInfo("LENAAPI", "%d Lena records loaded from cache.", len(paths))
}

func (self *Lena) lenaAPI(query string) (string, error) {
	// Configure clienttransport to skip SSL certificate verification.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	// Build request and add API authorization token header.
	req, err := http.NewRequest("GET", self.apiUrl+query, nil)
	req.Header.Add("Authorization", "Token "+self.authToken)

	// Execute the request.
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	// Return the response body.
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (self *Lena) getSamplesForFlowcell(fcid string) ([]interface{}, error) {
	// First look for the flowcell id in the cache.
	if sample, ok := self.cache[fcid]; ok {
		return sample, nil
	}

	// If not in the cache, hit the remote API.
	body, err := self.lenaAPI("/Sample/?sequencing_run__name=" + fcid)
	fmt.Println(body)
	if err != nil {
		return nil, err
	}
	var v interface{}
	err = json.Unmarshal([]byte(body), &v)
	if err != nil {
		return nil, err
	}
	return v.([]interface{}), nil
}
