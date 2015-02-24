package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"martian/core"
	"os"
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
	self.cache = map[string]bool{}

	paths, err := self.db.GetPipestances()
	if err != nil {
		core.LogError(err, "keplerd", "Failed to load pipestance cache: %s", err.Error())
		os.Exit(1)
	}

	for _, path := range paths {
		self.cache[path] = true
	}
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

func (self *PipestanceManager) InsertPipestance(psPath string) error {
	perfPath := path.Join(psPath, "_perf")
	versionsPath := path.Join(psPath, "_versions")
	invocationPath := path.Join(psPath, "_invocation")

	martianVersion, pipelinesVersion := self.parseVersions(versionsPath)
	call, args := self.parseInvocation(invocationPath)

	var nodes []*core.NodePerfInfo
	err := json.Unmarshal([]byte(read(perfPath)), &nodes)
	if err != nil {
		return err
	}

	if len(nodes) == 0 {
		return &core.MartianError{fmt.Sprintf("Pipestance %s has empty _perf file", psPath)}
	}

	// Wrap database insertions in transaction
	tx := NewDatabaseTx()
	tx.Begin()
	defer tx.End()

	// Insert pipestance with its metadata
	fqname := nodes[0].Fqname
	err = self.db.InsertPipestance(tx, psPath, fqname, martianVersion,
		pipelinesVersion, call, args)
	if err != nil {
		return err
	}

	// First pass: Insert all forks, chunks, splits, joins
	for _, node := range nodes {
		for _, fork := range node.Forks {
			self.db.InsertFork(tx, node.Name, node.Fqname, node.Type, fork.Index, fork.ForkStats)
			if fork.SplitStats != nil {
				err := self.db.InsertSplit(tx, node.Fqname, fork.Index, fork.SplitStats)
				if err != nil {
					return err
				}
			}
			if fork.JoinStats != nil {
				err := self.db.InsertJoin(tx, node.Fqname, fork.Index, fork.JoinStats)
				if err != nil {
					return err
				}
			}
			for _, chunk := range fork.Chunks {
				err := self.db.InsertChunk(tx, node.Fqname, fork.Index, chunk.ChunkStats, chunk.Index)
				if err != nil {
					return err
				}
			}
		}
	}

	// Second pass: Insert relationships between pipelines and stages
	for _, node := range nodes {
		for _, fork := range node.Forks {
			for _, stage := range fork.Stages {
				err := self.db.InsertRelationship(tx, node.Fqname, fork.Index, stage.Fqname, stage.Forki)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
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
					core.LogInfo("keplerd", "Adding pipestance %s", newPsPath)
					if err := self.InsertPipestance(newPsPath); err != nil {
						core.LogError(err, "keplerd", "Failed to add pipestance %s: %s",
							newPsPath, err.Error())
						delete(self.cache, newPsPath)
					}
				}
			}(psPath)
		}
		wg.Wait()
		time.Sleep(time.Minute * time.Duration(5))
	}()
}
