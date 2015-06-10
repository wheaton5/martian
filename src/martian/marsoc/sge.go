//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Marsoc SGE Interface
//
package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// Structs to parse XML output of qstat.
type Job_list struct {
	State              string `xml:"state,attr"`
	JB_job_number      string `xml:"JB_job_number"`
	JAT_prio           string `xml:"JAT_prio"`
	JB_name            string `xml:"JB_name"`
	JB_owner           string `xml:"JB_owner"`
	StateCode          string `xml:"state"`
	JAT_start_time     string `xml:"JAT_start_time"`
	JB_submission_time string `xml:"JB_submission_time"`
	Queue_name         string `xml:"queue_name"`
	Slots              int    `xml:"slots"`
}

type Queue_list struct {
	Name        string     `xml:"name"`
	Qtype       string     `xml:"qtype"`
	Slots_used  int        `xml:"slots_used"`
	Slots_resv  int        `xml:"slots_resv"`
	Slots_total int        `xml:"slots_total"`
	Load_avg    float64    `xml:"load_avg"`
	Arch        string     `xml:"arch"`
	Job_list    []Job_list `xml:"job_list"`
}

type Queue_info struct {
	Queue_list []Queue_list `xml:"Queue-List"`
}

type Job_info struct {
	Job_list []Job_list `xml:"job_list"`
}

type QStatOutput struct {
	Queue_info []Queue_info `xml:"queue_info"`
	Job_info   []Job_info   `xml:"job_info"`
}

// SGE declaration.
type SGE struct {
	QStatData interface{}
	SGEMutex  *sync.RWMutex
}

// SGE constructor.
func NewSGE() *SGE {
	self := &SGE{}
	self.QStatData = ""
	self.SGEMutex = &sync.RWMutex{}
	return self
}

// Start an infinite qstat loop.
func (self *SGE) goQStatLoop() {
	go func() {
		for {
			// Run qstat in xml mode, and gather stdout.
			if qstatxml, err := exec.Command("qstat", "-u", "*", "-xml", "-f").Output(); err != nil {
				self.QStatData = fmt.Sprintf("SGE Error: %v", err)
			} else {
				// Parse the XML.
				v := QStatOutput{}
				if err := xml.Unmarshal([]byte(qstatxml), &v); err != nil {
					self.SGEMutex.Lock()
					self.QStatData = fmt.Sprintf("XML Error: %v", err)
					self.SGEMutex.Unlock()
				} else {
					self.SGEMutex.Lock()
					self.QStatData = v
					self.SGEMutex.Unlock()
				}
			}

			// Wait for a bit.
			time.Sleep(time.Second * time.Duration(15))
		}
	}()
}

func (self *SGE) getJSON() string {
	jstr := ""
	self.SGEMutex.Lock()
	if bytes, err := json.Marshal(self.QStatData); err != nil {
		jstr = err.Error()
	} else {
		jstr = string(bytes)
	}
	self.SGEMutex.Unlock()
	return jstr
}
