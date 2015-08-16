package main

import (
	"io/ioutil"
	"martian/core"
	"os"
	"path"
)

type Pipestance struct {
	Domain string `json:"domain"`
	Date   string `json:"date"`
	Psid   string `json:"psid"`
	State  string `json:"state"`
	Path   string `json:"path"`
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
				state := self.GetPipestanceState(domain, date, psid)
				path := path.Join(self.psPath, domain, date, psid)
				ps := &Pipestance{Domain: domain, Date: date, Psid: psid, State: state, Path: path}
				pstances = append(pstances, ps)
			}
		}
	}
	return pstances
}

func (self *PipestanceManager) GetPipestanceState(container string, pname string, psid string) string {
	pipestance, ok := self.GetPipestance(container, pname, psid, true)
	if !ok {
		return "waiting"
	}

	return pipestance.GetState()
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
func (self *PipestanceManager) GetPipestance(container string, pname string, psid string, readOnly bool) (*core.Pipestance, bool) {
	psPath := self.makePipestancePath(container, pname, psid)
	pipestance, _ := self.rt.ReattachToPipestanceWithMroSrc(psid, psPath, "", "", "", map[string]string{}, false, true)
	pipestance.LoadMetadata()
	return pipestance, true
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
