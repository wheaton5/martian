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
	psPaths         []string
	mroPath         string
	mroVersion      string
	exploredPaths   map[string]bool
	cache           map[string][]string
	rt              *core.Runtime
	db              *DatabaseManager
	mutex           *sync.Mutex
	perfChan        chan string
	numPerfHandlers int
}

func makeInvocationPath(root string) string {
	return path.Join(root, "_invocation")
}

func makeFinalStatePath(root string) string {
	return path.Join(root, "_finalstate")
}

func makePerfPath(root string) string {
	return path.Join(root, "_perf")
}

func makeVersionsPath(root string) string {
	return path.Join(root, "_versions")
}

func makeTagsPath(root string) string {
	return path.Join(root, "_tags")
}

func read(path string) string {
	bytes, _ := ioutil.ReadFile(path)
	return string(bytes)
}

func NewPipestanceManager(psPaths []string, mroPath string, mroVersion string, db *DatabaseManager,
	rt *core.Runtime) *PipestanceManager {
	self := &PipestanceManager{}
	self.psPaths = psPaths
	self.mroPath = mroPath
	self.mroVersion = mroVersion
	self.rt = rt
	self.db = db
	self.mutex = &sync.Mutex{}
	self.perfChan = make(chan string)
	self.numPerfHandlers = 8
	self.loadCache()
	return self
}

func (self *PipestanceManager) loadCache() {
	self.exploredPaths = map[string]bool{}
	self.cache = map[string][]string{}

	pipestances, err := self.db.GetPipestances()
	if err != nil {
		core.LogError(err, "keplerd", "Failed to load pipestance caches: %s", err.Error())
		os.Exit(1)
	}

	for _, ps := range pipestances {
		path := ps["path"]
		fqname := ps["fqname"]
		version := ps["version"]
		self.addPath(path)
		self.insertCache(fqname, version)
	}
}

func (self *PipestanceManager) pushQueue(psPath string) {
	go func() {
		self.perfChan <- psPath
	}()
}

func (self *PipestanceManager) popQueue() string {
	return <-self.perfChan
}

func (self *PipestanceManager) deletePath(psPath string) {
	self.mutex.Lock()
	delete(self.exploredPaths, psPath)
	self.mutex.Unlock()
}

func (self *PipestanceManager) addPath(psPath string) {
	self.mutex.Lock()
	self.exploredPaths[psPath] = true
	self.mutex.Unlock()
}

func (self *PipestanceManager) getPath(psPath string) (bool, bool) {
	self.mutex.Lock()
	value, ok := self.exploredPaths[psPath]
	self.mutex.Unlock()
	return value, ok
}

func (self *PipestanceManager) recursePath(root string) []string {
	newPsPaths := []string{}
	if _, ok := self.getPath(root); ok {
		// This directory has already been explored
		return newPsPaths
	}

	invocationPath := makeInvocationPath(root)
	finalStatePath := makeFinalStatePath(root)
	perfPath := makePerfPath(root)
	if _, err := os.Stat(invocationPath); err == nil {
		// This directory is a pipestance
		if _, err := os.Stat(finalStatePath); err == nil {
			core.LogInfo("keplerd", "Found pipestance %s", root)
			if _, err := os.Stat(perfPath); err == nil {
				// Insert pipestance into database
				newPsPaths = append(newPsPaths, root)
			} else {
				// Generate pipestance performance info
				self.pushQueue(root)
			}
			self.addPath(root)
		}
		return newPsPaths
	}

	// Otherwise recurse until we find a pipestance directory
	infos, _ := ioutil.ReadDir(root)
	for _, info := range infos {
		if info.IsDir() {
			newPsPaths = append(newPsPaths, self.recursePath(
				path.Join(root, info.Name()))...)
		}
	}
	return newPsPaths
}

func (self *PipestanceManager) parseVersions(psPath string) (string, string, error) {
	versionsPath := makeVersionsPath(psPath)
	data := read(versionsPath)
	return core.ParseVersions(data)
}

func (self *PipestanceManager) parseFinalState(psPath string) (string, string, map[string]interface{}, error) {
	finalStatePath := makeFinalStatePath(psPath)
	args := map[string]interface{}{}

	var v []*core.NodeInfo
	err := json.Unmarshal([]byte(read(finalStatePath)), &v)
	if err != nil {
		return "", "", args, err
	}
	if len(v) == 0 {
		return "", "", args, &core.MartianError{fmt.Sprintf("Pipestance %s has empty _finalstate file", psPath)}
	}

	topNode := v[0]
	for _, fork := range topNode.Forks {
		for _, binding := range fork.Bindings.Argument {
			if !binding.Sweep {
				args[binding.Id] = binding.Value
			}
		}
	}
	for _, binding := range topNode.SweepBindings {
		args[binding.Id] = binding.Value
	}
	return topNode.Name, topNode.Fqname, args, nil
}

func (self *PipestanceManager) parseTags(psPath string) []string {
	tagsPath := makeTagsPath(psPath)

	var v []string
	if err := json.Unmarshal([]byte(read(tagsPath)), &v); err == nil {
		return v
	}
	return []string{}
}

func (self *PipestanceManager) parsePerf(psPath string, fqname string) ([]*core.NodePerfInfo, error) {
	perfPath := makePerfPath(psPath)

	var nodes []*core.NodePerfInfo
	err := json.Unmarshal([]byte(read(perfPath)), &nodes)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

func (self *PipestanceManager) writePerf(psPath string) error {
	perfPath := makePerfPath(psPath)
	if _, err := os.Stat(perfPath); err != nil {
		_, fqname, _, err := self.parseFinalState(psPath)
		if err != nil {
			return err
		}

		_, psid := core.ParseFQName(fqname)

		envs := map[string]string{}
		pipestance, err := self.rt.ReattachToPipestanceWithMroSrc(psid, psPath, "", self.mroPath,
			self.mroVersion, envs, false, true)
		if err != nil {
			return err
		}

		pipestance.LoadMetadata()

		// Sanity check that pipestance is complete.
		state := pipestance.GetState()
		if state != "complete" {
			return &core.MartianError{fmt.Sprintf("Pipestance %s is not complete", psPath)}
		}

		core.EnterCriticalSection()
		pipestance.Immortalize()
		core.ExitCriticalSection()
	}
	return nil
}

func (self *PipestanceManager) insertCache(fqname string, pipelinesVersion string) {
	if _, ok := self.cache[fqname]; ok {
		self.cache[fqname] = append(self.cache[fqname], pipelinesVersion)
	} else {
		self.cache[fqname] = []string{pipelinesVersion}
	}
}

func (self *PipestanceManager) searchCache(fqname string, pipelinesVersion string) bool {
	found := false
	if versions, ok := self.cache[fqname]; ok {
		for _, version := range versions {
			if version == pipelinesVersion {
				found = true
				break
			}
		}
	}
	return found
}

func (self *PipestanceManager) InsertPipestance(psPath string) error {
	martianVersion, pipelinesVersion, _ := self.parseVersions(psPath)
	tags := self.parseTags(psPath)

	call, fqname, args, err := self.parseFinalState(psPath)
	if err != nil {
		return err
	}

	// Check cache
	if self.searchCache(fqname, pipelinesVersion) {
		return &core.MartianError{fmt.Sprintf("Pipestance %s has duplicate fqname %s and version %s", psPath, fqname, pipelinesVersion)}
	}

	nodes, err := self.parsePerf(psPath, fqname)
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

	// Aggregate pipestance stats
	topNode := nodes[0]
	forkStats := []*core.PerfInfo{}
	for _, fork := range topNode.Forks {
		forkStats = append(forkStats, fork.ForkStats)
	}
	pipestanceStats := core.ComputeStats(forkStats, []string{}, nil, false)

	// Insert pipestance with its metadata
	err = self.db.InsertPipestance(tx, psPath, fqname, martianVersion,
		pipelinesVersion, pipestanceStats, call, args, tags)
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

	// Insert pipestance into cache
	self.insertCache(fqname, pipelinesVersion)

	return nil
}

func (self *PipestanceManager) InsertPipestances(newPsPaths []string) {
	for _, newPsPath := range newPsPaths {
		core.LogInfo("keplerd", "Adding pipestance %s", newPsPath)
		if err := self.InsertPipestance(newPsPath); err != nil {
			core.LogError(err, "keplerd", "Failed to add pipestance %s: %s",
				newPsPath, err.Error())
			self.deletePath(newPsPath)
		}
	}
}

func (self *PipestanceManager) startPerfHandlers() {
	for i := 0; i < self.numPerfHandlers; i++ {
		go func() {
			for {
				psPath := self.popQueue()
				core.LogInfo("keplerd", "Writing _perf for pipestance %s", psPath)
				if err := self.writePerf(psPath); err != nil {
					core.LogError(err, "keplerd", "Failed to write _perf for pipestance %s: %s",
						psPath, err.Error())
				}
				self.deletePath(psPath)
			}
		}()
	}
}

func (self *PipestanceManager) startInsertHandler() {
	go func() {
		for {
			newPsPaths := []string{}
			for _, psPath := range self.psPaths {
				newPsPaths = append(newPsPaths, self.recursePath(psPath)...)
			}
			self.InsertPipestances(newPsPaths)
			time.Sleep(time.Minute * time.Duration(5))
		}
	}()
}

func (self *PipestanceManager) Start() {
	self.startPerfHandlers()
	self.startInsertHandler()
}
