//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// Houston pipestance manager.
//

package main

import (
	"encoding/json"
	"io/ioutil"
	"martian/core"
	"os"
	"path"
	"sort"
	"time"
)

type Pipestance struct {
	Domain string `json:"domain"`
	Date   string `json:"date"`
	Psid   string `json:"psid"`
	State  string `json:"state"`
	Path   string `json:"path"`
}

// Sorting support for Pipestance
type ByDate []*Pipestance

func (a ByDate) Len() int      { return len(a) }
func (a ByDate) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByDate) Less(i, j int) bool {
	if a[i].Date == a[j].Date {
		return a[i].Domain > a[j].Domain
	}
	return a[i].Date > a[j].Date
}

type PipestanceManager struct {
	psPath    string
	cachePath string
	cache     map[string]*Pipestance
	rt        *core.Runtime
}

func NewPipestanceManager(rt *core.Runtime, psPath string, cachePath string) *PipestanceManager {
	self := &PipestanceManager{}
	self.psPath = psPath
	self.cachePath = path.Join(cachePath, "pipestances")
	self.cache = map[string]*Pipestance{}
	self.rt = rt
	self.loadCache()
	return self
}

func (self *PipestanceManager) Enumerate() []*Pipestance {
	pstances := []*Pipestance{}
	for _, v := range self.cache {
		pstances = append(pstances, v)
	}
	sort.Sort(ByDate(pstances))
	return pstances
}

func (self *PipestanceManager) loadCache() error {
	bytes, err := ioutil.ReadFile(self.cachePath)
	if err != nil {
		core.LogInfo("pipeman", "Could not read cache file %s.", self.cachePath)
		return err
	}

	if err := json.Unmarshal(bytes, &self.cache); err != nil {
		core.LogError(err, "pipeman", "Could not parse JSON in cache file %s.", self.cachePath)
		return err
	}

	core.LogInfo("pipeman", "%d pipestances loaded from cache.", len(self.cache))
	return nil
}

func (self *PipestanceManager) StartInventoryLoop() {
	go func() {
		self.inventoryPipestances()

		// Wait for a bit.
		time.Sleep(time.Minute * time.Duration(1))
	}()
}

func writeJson(fpath string, object interface{}) {
	bytes, _ := json.MarshalIndent(object, "", "    ")
	if err := ioutil.WriteFile(fpath, bytes, 0644); err != nil {
		core.LogError(err, "pipeman", "Could not write JSON file %s.", fpath)
	}
}

func (self *PipestanceManager) inventoryPipestances() {
	domainInfos, _ := ioutil.ReadDir(self.psPath)
	for _, domainInfo := range domainInfos {
		domain := domainInfo.Name()
		dateInfos, _ := ioutil.ReadDir(path.Join(self.psPath, domain))
		for _, dateInfo := range dateInfos {
			date := dateInfo.Name()
			psidInfos, _ := ioutil.ReadDir(path.Join(self.psPath, domain, date))
			for _, psidInfo := range psidInfos {
				psid := psidInfo.Name()
				key := self.makePipestanceKey(domain, date, psid)

				// If this pipestance is not already cached, cache it
				if _, ok := self.cache[key]; !ok {
					state := self.GetPipestanceState(domain, date, psid)
					path := path.Join(self.psPath, domain, date, psid)
					self.cache[key] = &Pipestance{Domain: domain, Date: date, Psid: psid, State: state, Path: path}
					core.LogInfo("pipeman", "Discovered new pipestance %s.", key)
				}
			}
		}
	}
	writeJson(self.cachePath, self.cache)
}

func (self *PipestanceManager) makePipestanceKey(container string, pname string, psid string) string {
	return container + pname + psid
}

func (self *PipestanceManager) makePipestancePath(container string, pname string, psid string) string {
	return path.Join(self.psPath, container, pname, psid, "HEAD")
}

func (self *PipestanceManager) getPipestanceMetadata(container string, pname string, psid string, fname string) (string, error) {
	psPath := self.makePipestancePath(container, pname, psid)

	data, err := ioutil.ReadFile(path.Join(psPath, fname))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (self *PipestanceManager) GetPipestance(container string, pname string, psid string, readOnly bool) (*core.Pipestance, bool) {
	psPath := self.makePipestancePath(container, pname, psid)
	pipestance, _ := self.rt.ReattachToPipestanceWithMroSrc(psid, psPath, "", "", "", map[string]string{}, false, true)
	pipestance.LoadMetadata()
	return pipestance, true
}

func (self *PipestanceManager) GetPipestanceState(container string, pname string, psid string) string {
	pipestance, ok := self.GetPipestance(container, pname, psid, true)
	if !ok {
		return "waiting"
	}

	return pipestance.GetState()
}

func (self *PipestanceManager) GetPipestanceTopFile(container string, pname string, psid string, fname string) (string, error) {
	return self.getPipestanceMetadata(container, pname, psid, "_"+fname)
}

func (self *PipestanceManager) GetPipestanceMetadata(container string, pname string, psid string, metadataPath string) (string, error) {
	psPath := self.makePipestancePath(container, pname, psid)
	permanentPsPath, _ := os.Readlink(psPath)
	return self.rt.GetMetadata(permanentPsPath, metadataPath)
}

func (self *PipestanceManager) GetPipestanceCommandline(container string, pname string, psid string) (string, error) {
	return self.getPipestanceMetadata(container, pname, psid, "_cmdline")
}

func (self *PipestanceManager) GetPipestanceInvokeSrc(container string, pname string, psid string) (string, error) {
	return self.getPipestanceMetadata(container, pname, psid, "_invocation")
}

func (self *PipestanceManager) GetPipestanceTimestamp(container string, pname string, psid string) (string, error) {
	data, err := self.getPipestanceMetadata(container, pname, psid, "_timestamp")
	if err != nil {
		return "", err
	}
	return core.ParseTimestamp(data), nil
}

func (self *PipestanceManager) GetPipestanceVersions(container string, pname string, psid string) (string, string, error) {
	data, err := self.getPipestanceMetadata(container, pname, psid, "_versions")
	if err != nil {
		return "", "", err
	}
	return core.ParseVersions(data)
}

func (self *PipestanceManager) GetPipestanceJobMode(container string, pname string, psid string) (string, string, string) {
	data, err := self.getPipestanceMetadata(container, pname, psid, "_cmdline")
	if err != nil {
		return "", "", ""
	}
	return core.ParseJobMode(data)
}

func (self *PipestanceManager) GetPipestanceSerialization(container string, pname string, psid string, name string) (interface{}, bool) {
	psPath := self.makePipestancePath(container, pname, psid)
	if ser, ok := self.rt.GetSerialization(psPath, name); ok {
		return ser, true
	}

	pipestance, ok := self.GetPipestance(container, pname, psid, true)
	if !ok {
		return nil, false
	}

	// Cache serialization of pipestance
	pipestance.Immortalize()
	if ser, ok := self.rt.GetSerialization(psPath, name); ok {
		return ser, true
	}

	return pipestance.Serialize(name), true
}
