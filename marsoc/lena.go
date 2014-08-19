//
// Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
//
// Marsoc LENA API wrapper.
//
package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"margo/core"
	"net/http"
	"path"
	"time"
)

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
	Cell_line                string         `json:"cell_line"`
	Pname                    string         `json:"pname"`
	Psstate                  string         `json:"psstate"`
	Callsrc                  string         `json:"callsrc"`
}

type Lena struct {
	downloadUrl string
	authToken   string
	dbPath      string
	cache       map[string][]*Sample
	sampleBag   map[int]interface{}
	mailer      *core.Mailer
}

func NewLena(downloadUrl string, authToken string, cachePath string, mailer *core.Mailer) *Lena {
	self := &Lena{}
	self.downloadUrl = downloadUrl
	self.authToken = authToken
	self.dbPath = path.Join(cachePath, "lena.json")
	self.cache = map[string][]*Sample{}
	self.sampleBag = map[int]interface{}{}
	self.mailer = mailer
	return self
}

func (self *Lena) loadDatabase() {
	data, err := ioutil.ReadFile(self.dbPath)
	if err != nil {
		core.LogError(err, "LENAAPI", "Could not read database file %s.", self.dbPath)
		return
	}
	err = self.ingestDatabase(data)
	if err != nil {
		self.mailer.Sendmail(
			fmt.Sprintf("I swallowed a JSON bug."),
			fmt.Sprintf("Human,\n\nYou appear to have changed the Lena schema without updating my own.\n\nI will not show you any more samples until you rectify this oversight."),
		)
		core.LogError(err, "LENAAPI", "Could not parse JSON in %s.", self.dbPath)
	}
}

func (self *Lena) ingestDatabase(data []byte) error {
	// First parse the JSON as structured data into Sample.
	var samples []*Sample
	if err := json.Unmarshal(data, &samples); err != nil {
		return err
	}
	for _, sample := range samples {
		if sample.Sequencing_run == nil {
			continue
		}

		// Store them into lists indexed by flowcell id.
		fcid := sample.Sequencing_run.Name
		slist, ok := self.cache[fcid]
		if ok {
			self.cache[fcid] = append(slist, sample)
		} else {
			self.cache[fcid] = []*Sample{sample}
		}
	}
	// Now parse the JSON into unstructured interface{} bags,
	// which is only used as input into argshim.buildCallSourceForSample.
	// We need this to be schemaless to allow Lena schema changes
	// to pass through to the argshim without the need to update MARSOC.
	var bag interface{}
	if err := json.Unmarshal(data, &bag); err != nil {
		return err
	}
	bagIfaces, ok := bag.([]interface{})
	if !ok {
		return errors.New("JSON does not contain a top-level list.")
	}
	for _, iface := range bagIfaces {
		bagSample, ok := iface.(map[string]interface{})
		if !ok {
			return errors.New("JSON list includes something that was not an object.")
		}
		idIface := bagSample["id"]
		bagSampleId, ok := idIface.(float64)
		if !ok {
			return errors.New(fmt.Sprintf("JSON object contains value for id that is not a number %v.", idIface))
		}
		self.sampleBag[int(bagSampleId)] = iface
	}

	core.LogInfo("LENAAPI", "%d samples, %d bags loaded from %s.", len(samples), len(self.sampleBag), self.dbPath)
	return nil
}

// Start an infinite download loop.
func (self *Lena) goDownloadLoop() {
	go func() {
		for {
			core.LogInfo("LENAAPI", "Starting download...")
			data, err := self.lenaAPI()
			if err != nil {
				core.LogError(err, "LENAAPI", "Download error.")
			} else {
				core.LogInfo("LENAAPI", "Download complete. %d bytes.", len(data))
				err := self.ingestDatabase(data)
				if err == nil {
					// If JSON parsed properly, save it.
					ioutil.WriteFile(self.dbPath, data, 0600)
					core.LogInfo("LENAAPI", "Database ingested and saved to %s.", self.dbPath)
				} else {
					core.LogError(err, "LENAAPI", "Could not parse JSON from downloaded data.")
				}
			}

			// Wait for a bit.
			time.Sleep(time.Minute * time.Duration(10))
		}
	}()
}

func (self *Lena) lenaAPI() ([]byte, error) {
	// Configure clienttransport to skip SSL certificate verification.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	// Build request and add API authorization token header.
	req, err := http.NewRequest("GET", self.downloadUrl, nil)
	req.Header.Add("Authorization", "Token "+self.authToken)

	// Execute the request.
	res, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	}
	defer res.Body.Close()

	// Return the response body.
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return []byte{}, err
	}
	return body, nil
}

func (self *Lena) getSamplesForFlowcell(fcid string) ([]*Sample, error) {
	if samples, ok := self.cache[fcid]; ok {
		return samples, nil
	}
	return []*Sample{}, nil
}

func (self *Lena) getSampleBagWithId(sampleId int) interface{} {
	return self.sampleBag[sampleId]
}
