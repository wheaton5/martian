package main

import (
	_ "encoding/json"
	"io/ioutil"
	_ "martian/core"
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
}

func NewPipestanceManager(psPath string) *PipestanceManager {
	self := &PipestanceManager{}
	self.psPath = psPath
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
