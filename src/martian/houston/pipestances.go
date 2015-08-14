package main

import (
	_ "encoding/json"
	"io/ioutil"
	"martian/core"
	_ "os"
	"path"
	_ "sync"
	_ "time"
)

type Pipestance struct {
	Domain string `json:"domain"`
	Date   string `json:"date"`
	Psid   string `json:"psid"`
}

type PipestanceManager struct {
	psPath string
	rt     *core.Runtime
}

func NewPipestanceManager(psPath string, rt *core.Runtime) *PipestanceManager {
	self := &PipestanceManager{}
	self.psPath = psPath
	self.rt = rt
	return self
}

func (self *PipestanceManager) Enumerate() []*Pipestance {
	pstances := []*Pipestance{}
	domainInfos, _ := ioutil.ReadDir(self.psPath)
	for _, domainInfo := range domainInfos {
		domain := domainInfo.Name()
		dateInfos, _ := ioutil.ReadDir(path.Join(self.psPath, domain))
		for _, dateInfo := range dateInfos {
			date := dateInfo.Name()
			psidInfos, _ := ioutil.ReadDir(path.Join(self.psPath, domain, date))
			for _, psidInfo := range psidInfos {
				psid := psidInfo.Name()
				ps := &Pipestance{Domain: domain, Date: date, Psid: psid}
				pstances = append(pstances, ps)
			}
		}
	}
	return pstances
}

func (self *PipestanceManager) makePipestancePath(container string, pipeline string, psid string) string {
	return path.Join(self.psPath, container, pipeline, psid, "HEAD")
}

func (self *PipestanceManager) GetPipestance(container string, pipeline string, psid string, readOnly bool) (*core.Pipestance, bool) {
	psPath := self.makePipestancePath(container, pipeline, psid)
	pipestance, _ := self.rt.ReattachToPipestanceWithMroSrc(psid, psPath, "", "",
		"99", map[string]string{}, false, true)
	pipestance.LoadMetadata()
	return pipestance, true
}

func (self *PipestanceManager) GetPipestanceSerialization(container string, pipeline string, psid string, name string) (interface{}, bool) {
	psPath := self.makePipestancePath(container, pipeline, psid)
	if ser, ok := self.rt.GetSerialization(psPath, name); ok {
		return ser, true
	}

	readOnly := true
	pipestance, ok := self.GetPipestance(container, pipeline, psid, readOnly)
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
