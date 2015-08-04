//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
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
	"martian/core"
	"martian/manager"
	"net/http"
	"path"
	"sort"
	"strconv"
	"sync"
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

type BarcodeSet struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type SequencingRun struct {
	Id                    int      `json:"id"`
	State                 string   `json:"state"`
	Name                  string   `json:"name"`
	Date                  string   `json:"date"`
	Loading_concentration float32  `json:"loading_concentration"`
	Failure_reason        string   `json:"failure_reason"`
	Read1_length          int      `json:"read1_length"`
	Read2_length          int      `json:"read2_length"`
	Barcode_length        int      `json:"barcode_length"`
	Samples               []string `json:"samples"`
	Psstate               string   `json:"psstate"`
}

type User struct {
	Id       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type CellLine struct {
	Name string `json:"name"`
	Sex  string `json:"sex"`
}

type SampleDef struct {
	Sequencing_run   *SequencingRun `json:"sequencing_run"`
	Sequencer        string         `json:"sequencer"`
	Sample_indexes   []*Oligo       `json:"sample_indexes"`
	Sample_index_set []int          `json:"sample_index_set"`
	Lane             string         `json:"lane"`
	Gem_group        int            `json:"gem_group"`
}

type MetasamplePrereq struct {
	Fcid  string `json:"fcid"`
	State string `json:"state"`
}

type Sample struct {
	Id                       int          `json:"id"`
	Description              string       `json:"description"`
	Name                     string       `json:"name"`
	State                    string       `json:"state"`
	Genome                   *Genome      `json:"genome"`
	Target_set               *TargetSet   `json:"target_set"`
	Primers                  []*Oligo     `json:"primers"`
	Degenerate_primer_length int          `json:"degenerate_primer_length"`
	Barcode_set              *BarcodeSet  `json:"barcode_set"`
	Template_input_mass      float32      `json:"template_input_mass"`
	User                     *User        `json:"user"`
	Cell_line                *CellLine    `json:"cell_line"`
	Pipeline                 string       `json:"pipeline"`
	Product                  string       `json:"product"`
	Exclude_non_bc_reads     bool         `json:"exclude_non_bc_reads"`
	Sample_defs              []*SampleDef `json:"sample_defs"`
	Pname                    string       `json:"pname"`
	Pscontainer              string       `json:"pscontainer"`
	Psstate                  string       `json:"psstate"`
	Ready_to_invoke          bool         `json:"ready_to_invoke"`
	Callsrc                  string       `json:"callsrc"`
}

type BySampleId []*Sample

func (a BySampleId) Len() int           { return len(a) }
func (a BySampleId) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a BySampleId) Less(i, j int) bool { return a[i].Id < a[j].Id }

type Lena struct {
	downloadUrl string
	authToken   string
	dbPath      string
	fcidTable   map[string]map[int]*Sample
	spidTable   map[int]*Sample
	sbagTable   map[int]interface{}
	metasamples []*Sample
	lenaDbMutex *sync.RWMutex
	mailer      *manager.Mailer
}

func NewLena(downloadUrl string, authToken string, cachePath string, mailer *manager.Mailer) *Lena {
	self := &Lena{}
	self.downloadUrl = downloadUrl
	self.authToken = authToken
	self.dbPath = path.Join(cachePath, "lena.json")
	self.fcidTable = map[string]map[int]*Sample{}
	self.spidTable = map[int]*Sample{}
	self.sbagTable = map[int]interface{}{}
	self.metasamples = []*Sample{}
	self.lenaDbMutex = &sync.RWMutex{}
	self.mailer = mailer
	return self
}

func (self *Lena) LoadDatabase() {
	data, err := ioutil.ReadFile(self.dbPath)
	if err != nil {
		core.LogError(err, "lenaapi", "Could not read database file %s.", self.dbPath)
		return
	}
	err = self.ingestDatabase(data)
	if err != nil {
		self.mailer.Sendmail(
			[]string{},
			fmt.Sprintf("I swallowed a JSON bug."),
			fmt.Sprintf("Human,\n\nYou appear to have changed the Lena schema without updating my own.\n\nI will not show you any more samples until you rectify this oversight."),
		)
		core.LogError(err, "lenaapi", "Could not parse JSON in %s.", self.dbPath)
	}
}

func (self *Lena) ingestDatabase(data []byte) error {
	// First parse the JSON as structured data into Sample.
	var samples []*Sample
	if err := json.Unmarshal(data, &samples); err != nil {
		return err
	}

	// Create a new, empty cache.
	fcidTable := map[string]map[int]*Sample{}
	spidTable := map[int]*Sample{}
	metasamples := []*Sample{}
	for _, sample := range samples {
		// Collect list of fcids referenced in the sample_defs
		uniqueFcids := map[string]bool{}

		for _, sample_def := range sample.Sample_defs {
			if sample_def.Sequencing_run == nil {
				continue
			}

			// Store them into lists indexed by flowcell id.
			fcid := sample_def.Sequencing_run.Name
			uniqueFcids[fcid] = true
			smap, ok := fcidTable[fcid]
			if ok {
				smap[sample.Id] = sample
			} else {
				fcidTable[fcid] = map[int]*Sample{sample.Id: sample}
			}
			spidTable[sample.Id] = sample
		}

		// Sort the uniquified fcids, and build the pscontainer
		// name from it(them).
		fcids := []string{}
		for fcid, _ := range uniqueFcids {
			fcids = append(fcids, fcid)
		}
		sort.Strings(fcids)
		if len(fcids) > 1 {
			// It's a metasample, add to list, set container to sample id.
			sample.Pscontainer = strconv.Itoa(sample.Id)
			metasamples = append(metasamples, sample)
		} else if len(fcids) == 1 {
			// Single-flowcell sample.
			sample.Pscontainer = fcids[0]
		} else {
			sample.Pscontainer = "ungrouped"
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

	// Create new, empty sample bag.
	sbagTable := map[int]interface{}{}
	for _, iface := range bagIfaces {
		spbag, ok := iface.(map[string]interface{})
		if !ok {
			return errors.New("JSON list includes something that was not an object.")
		}
		idIface := spbag["id"]
		fspid, ok := idIface.(float64)
		if !ok {
			return errors.New(fmt.Sprintf("JSON object contains value for id that is not a number %v.", idIface))
		}
		sbagTable[int(fspid)] = iface
	}

	sort.Sort(sort.Reverse(BySampleId(metasamples)))

	self.lenaDbMutex.Lock()
	self.fcidTable = fcidTable
	self.spidTable = spidTable
	self.metasamples = metasamples
	self.sbagTable = sbagTable
	self.lenaDbMutex.Unlock()

	core.LogInfo("lenaapi", "%d samples, %d bags loaded from %s.", len(samples), len(sbagTable), self.dbPath)
	return nil
}

// Start an infinite download loop.
func (self *Lena) GoDownloadLoop() {
	go func() {
		for {
			//core.LogInfo("lenaapi", "Starting download...")
			data, err := self.lenaAPI()
			if err != nil {
				core.LogError(err, "lenaapi", "Download error.")
			} else {
				//core.LogInfo("lenaapi", "Download complete. %s.", humanize.Bytes(uint64(len(data))))
				err := self.ingestDatabase(data)
				if err == nil {
					// If JSON parsed properly, save it.
					ioutil.WriteFile(self.dbPath, data, 0644)
					//core.LogInfo("lenaapi", "Database ingested and saved to %s.", self.dbPath)
				} else {
					core.LogError(err, "lenaapi", "Could not parse JSON from downloaded data.")
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

func (self *Lena) GetSamplesForFlowcell(fcid string) []*Sample {
	// Get sample map for this fcid.
	self.lenaDbMutex.RLock()
	sampleMap, ok := self.fcidTable[fcid]
	if !ok {
		self.lenaDbMutex.RUnlock()
		return []*Sample{}
	}

	// Sort the samples by id and only include single-flowcell samples.
	sampleIds := []int{}
	for sampleId := range sampleMap {
		sampleIds = append(sampleIds, sampleId)
	}
	sort.Ints(sampleIds)

	sampleList := []*Sample{}
	for _, sampleId := range sampleIds {
		sample := sampleMap[sampleId]
		// Include only single-flowcell samples (no metasamples).
		if sample.Pscontainer == fcid {
			sampleList = append(sampleList, sample)
		}
	}
	self.lenaDbMutex.RUnlock()
	return sampleList
}

func (self *Lena) GetMetasamples() []*Sample {
	self.lenaDbMutex.RLock()
	metasamples := make([]*Sample, len(self.metasamples))
	copy(metasamples, self.metasamples)
	self.lenaDbMutex.RUnlock()
	return metasamples
}

func (self *Lena) GetSampleWithId(sampleId string) *Sample {
	if spid, err := strconv.Atoi(sampleId); err == nil {
		self.lenaDbMutex.RLock()
		sample, _ := self.spidTable[spid]
		self.lenaDbMutex.RUnlock()
		return sample
	}
	return nil
}

func (self *Lena) GetSampleBagWithId(sampleId string) interface{} {
	if spid, err := strconv.Atoi(sampleId); err == nil {
		self.lenaDbMutex.RLock()
		sbag := self.sbagTable[spid]
		self.lenaDbMutex.RUnlock()
		return sbag
	}
	return nil
}
