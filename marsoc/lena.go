//
// Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
//
// Marsoc LENA API wrapper.
//
package main

import (
	"crypto/tls"
	"encoding/json"
	_ "fmt"
	"io/ioutil"
	"net/http"
	"path"
)

type Lena struct {
	apiUrl    string
	authToken string
	path      string
	cache     map[string][]*Sample
}

func NewLena(apiUrl string, authToken string, cachePath string) *Lena {
	self := &Lena{}
	self.apiUrl = apiUrl
	self.authToken = authToken
	self.path = path.Join(cachePath, "lena")
	self.cache = map[string][]*Sample{}
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
	Pname                    string         `json:"pname"`
	Psstate                  string         `json:"psstate"`
	Callsrc                  string         `json:"callsrc"`
}

func (self *Lena) loadDatabase() {
	dbPath := "./nice.json"
	bytes, err := ioutil.ReadFile(dbPath)
	if err != nil {
		logError(err, "LENAAPI", "Could not read database file %s.", dbPath)
		return
	}

	var samples []*Sample
	if err := json.Unmarshal(bytes, &samples); err != nil {
		logError(err, "LENAAPI", "Could not parse JSON in database file %s.", dbPath)
		return
	}
	for _, sample := range samples {
		if sample.Sequencing_run == nil {
			continue
		}

		fcid := sample.Sequencing_run.Name
		slist, ok := self.cache[fcid]
		if ok {
			self.cache[fcid] = append(slist, sample)
		} else {
			self.cache[fcid] = []*Sample{sample}
		}
	}
	logInfo("LENAAPI", "%d Lena samples loaded from data.", len(samples))
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

func (self *Lena) getSamplesForFlowcell(fcid string) ([]*Sample, error) {
	if samples, ok := self.cache[fcid]; ok {
		return samples, nil
	}
	return []*Sample{}, nil
}
