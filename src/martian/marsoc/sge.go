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
type JobList struct {
	State          string `xml:"state,attr"`
	JB_job_number  string `xml:"JB_job_number"`
	JAT_prio       string `xml:"JAT_prio"`
	JB_name        string `xml:"JB_name"`
	JB_owner       string `xml:"JB_owner"`
	StateCode      string `xml:"state"`
	JAT_start_time string `xml:"JAT_start_time"`
	Queue_name     string `xml:"queue_name"`
	Slots          int    `xml:"slots"`
}

type QueueInfo struct {
	JobList []JobList `xml:"job_list"`
}

type QStatOutput struct {
	QueueInfo []QueueInfo `xml:"queue_info"`
	JobInfo   []QueueInfo `xml:"job_info"`
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
			if qstatxml, err := exec.Command("qstat", "-u", "\"*\"", "-xml").Output(); err != nil {
				self.QStatData = fmt.Sprintf("SGE Error: %v", err)
			} else {
				// Parse the XML.
				v := QStatOutput{}
				if err := xml.Unmarshal([]byte(qstatxml), &v); err != nil {
					self.SGEMutex.Lock()
					self.QStatData = fmt.Sprintf("XML Error: %v", err)
					self.SGEMutex.Unlock()
				} else {
					// Count slots among running jobs only.
					slots := 0
					for _, job := range v.QueueInfo[0].JobList {
						slots += job.Slots
					}
					// Count errors among pending jobs.
					error_count := 0
					for _, job := range v.JobInfo[0].JobList {
						if job.StateCode[0] == 'E' {
							error_count++
						}
					}
					self.SGEMutex.Lock()
					self.QStatData = map[string]interface{}{
						"running_count": len(v.QueueInfo[0].JobList),
						"pending_count": len(v.JobInfo[0].JobList),
						"slots":         slots,
						"jobs":          append(v.QueueInfo[0].JobList, v.JobInfo[0].JobList...),
						"error_count":   error_count,
					}
					self.SGEMutex.Unlock()
				}
			}

			// Wait for a bit.
			time.Sleep(time.Second * time.Duration(30))
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
