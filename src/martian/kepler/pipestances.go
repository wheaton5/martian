package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"martian/core"
	"os"
	"path"
	"time"
)

type PipestanceManager struct {
	psPaths       []string
	exploredPaths map[string]bool
	cache         map[string]bool
	rt            *core.Runtime
	db            *DatabaseManager
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

func NewPipestanceManager(psPaths []string, db *DatabaseManager, rt *core.Runtime) *PipestanceManager {
	self := &PipestanceManager{}
	self.psPaths = psPaths
	self.rt = rt
	self.db = db
	self.loadCache()
	return self
}

func (self *PipestanceManager) loadCache() {
	self.exploredPaths = map[string]bool{}
	self.cache = map[string]bool{}

	pipestances, err := self.db.GetPipestances()
	if err != nil {
		core.LogError(err, "keplerd", "Failed to load pipestance caches: %s", err.Error())
		os.Exit(1)
	}

	for _, ps := range pipestances {
		path := ps["path"]
		fqname := ps["fqname"]
		self.exploredPaths[path] = true
		self.cache[fqname] = true
	}
}

func (self *PipestanceManager) recursePath(root string) []string {
	newPsPaths := []string{}
	if _, ok := self.exploredPaths[root]; ok {
		// This directory has already been explored
		return newPsPaths
	}

	invocationPath := makeInvocationPath(root)
	finalStatePath := makeFinalStatePath(root)
	if _, err := os.Stat(invocationPath); err == nil {
		// This directory is a pipestance
		if _, err := os.Stat(finalStatePath); err == nil {
			core.LogInfo("keplerd", "Found pipestance %s", root)
			newPsPaths = append(newPsPaths, root)
			self.exploredPaths[root] = true
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

func (self *PipestanceManager) parseVersions(psPath string) (string, string) {
	versionsPath := makeVersionsPath(psPath)

	var v map[string]string
	if err := json.Unmarshal([]byte(read(versionsPath)), &v); err == nil {
		return v["martian"], v["pipelines"]
	}
	return "", ""
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

	if _, err := os.Stat(perfPath); err != nil {
		// If _perf file does not exist, generate it.
		_, psid := core.ParseFQName(fqname)

		pipestance, err := self.rt.ReattachToPipestanceWithMroSrc(psid, psPath, "", false, true)
		if err != nil {
			return nil, err
		}

		pipestance.LoadMetadata()

		// Sanity check that pipestance is complete.
		state := pipestance.GetState()
		if state != "complete" {
			return nil, &core.MartianError{fmt.Sprintf("Pipestance %s is not complete", psPath)}
		}
		pipestance.Immortalize()
	}

	var nodes []*core.NodePerfInfo
	err := json.Unmarshal([]byte(read(perfPath)), &nodes)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

func (self *PipestanceManager) InsertPipestance(psPath string) error {
	martianVersion, pipelinesVersion := self.parseVersions(psPath)
	tags := self.parseTags(psPath)

	call, fqname, args, err := self.parseFinalState(psPath)
	if err != nil {
		return err
	}

	// Check cache
	if _, ok := self.cache[fqname]; ok {
		return nil
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
	pipestanceStats := core.ComputeStats(forkStats, []string{}, nil)

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
	self.cache[fqname] = true

	return nil
}

func (self *PipestanceManager) InsertPipestances(newPsPaths []string) {
	for _, newPsPath := range newPsPaths {
		core.LogInfo("keplerd", "Adding pipestance %s", newPsPath)
		if err := self.InsertPipestance(newPsPath); err != nil {
			core.LogError(err, "keplerd", "Failed to add pipestance %s: %s",
				newPsPath, err.Error())
			delete(self.exploredPaths, newPsPath)
		}
	}
}

func (self *PipestanceManager) Start() {
	go func() {
		for {
			for _, psPath := range self.psPaths {
				newPsPaths := self.recursePath(psPath)
				self.InsertPipestances(newPsPaths)
			}
			time.Sleep(time.Minute * time.Duration(5))
		}
	}()
}
