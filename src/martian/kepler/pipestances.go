package main

import (
	"encoding/json"
	"io/ioutil"
	"martian/core"
	"path"
	"sync"
	"time"
)

type PipestanceManager struct {
	psPaths []string
	cache   map[string]bool
	rt      *core.Runtime
	db      *DatabaseManager
}

func read(path string) string {
	bytes, _ := ioutil.ReadFile(path)
	return string(bytes)
}

func NewPipestanceManager(psPaths []string, db *DatabaseManager, rt *core.Runtime) *PipestanceManager {
	self := &PipestanceManager{}
	self.psPaths = psPaths
	self.rt = rt
	self.db = db
	self.loadCache()
	return self
}

func (self *PipestanceManager) loadCache() {
	// TODO: Load already seen pipestances from database
}

func (self *PipestanceManager) recursePath(root string) []string {
	newPsPaths := []string{}
	infos, _ := ioutil.ReadDir(root)
	for _, info := range infos {
		if info.IsDir() {
			newPsPaths = append(newPsPaths, self.recursePath(path.Join(root, info.Name()))...)
		} else if info.Name() == "_perf" {
			if _, ok := self.cache[root]; !ok {
				newPsPaths = append(newPsPaths, root)
				self.cache[root] = true
			}
		}
	}
	return newPsPaths
}

func (self *PipestanceManager) parseVersions(path string) (string, string) {
	var v map[string]string
	json.Unmarshal([]byte(read(path)), &v)
	return v["martian"], v["pipelines"]
}

func (self *PipestanceManager) parseInvocation(path string) (string, map[string]interface{}) {
	v, _ := self.rt.BuildCallJSON(read(path), path)
	return v["call"].(string), v["args"].(map[string]interface{})
}

func (self *PipestanceManager) parseFqname(path string) string {
	// TODO: Implement this!
	return ""
}

func (self *PipestanceManager) Start() {
	go func() {
		var wg sync.WaitGroup
		for _, psPath := range self.psPaths {
			wg.Add(1)
			go func(psPath string) {
				defer wg.Done()
				newPsPaths := self.recursePath(psPath)
				for _, newPsPath := range newPsPaths {
					perfPath := path.Join(newPsPath, "_perf")
					versionsPath := path.Join(newPsPath, "_versions")
					invocationPath := path.Join(newPsPath, "_invocation")

					tx, _ := self.db.NewTransaction()
					fqname := self.parseFqname(perfPath)
					martianVersion, pipelinesVersion := self.parseVersions(versionsPath)
					call, args := self.parseInvocation(invocationPath)
					self.db.InsertPipestance(tx, newPsPath, fqname, martianVersion,
						pipelinesVersion, call, args)
				}
			}(psPath)
		}
		wg.Wait()
		time.Sleep(time.Minute * time.Duration(5))
	}()
}
