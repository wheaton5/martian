//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Marsoc pipestance management.
//
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mario/core"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
)

func makeFQName(pipeline string, psid string) string {
	// This construction must remain identical to Pipestance::GetFQName.
	return fmt.Sprintf("ID.%s.%s", psid, pipeline)
}

type PipestanceNotification struct {
	State     string
	Container string
	Pname     string
	Psid      string
	Vdrsize   uint64
}

type PipestanceManager struct {
	rt             *core.Runtime
	marioVersion   string
	mroVersion     string
	path           string
	cachePath      string
	stepms         int
	pipelines      []string
	completed      map[string]bool
	failed         map[string]bool
	runList        []*core.Pipestance
	runListMutex   *sync.Mutex
	runTable       map[string]*core.Pipestance
	pendingTable   map[string]bool
	containerTable map[string]string
	notifyQueue    []*PipestanceNotification
	mailer         *Mailer
}

func NewPipestanceManager(rt *core.Runtime, marioVersion string,
	mroVersion string, pipestancesPath string, cachePath string, stepms int,
	mailer *Mailer) *PipestanceManager {
	self := &PipestanceManager{}
	self.rt = rt
	self.marioVersion = marioVersion
	self.mroVersion = mroVersion
	self.path = pipestancesPath
	self.cachePath = path.Join(cachePath, "pipestances")
	self.stepms = stepms
	self.pipelines = rt.PipelineNames
	self.completed = map[string]bool{}
	self.failed = map[string]bool{}
	self.runList = []*core.Pipestance{}
	self.runListMutex = &sync.Mutex{}
	self.runTable = map[string]*core.Pipestance{}
	self.pendingTable = map[string]bool{}
	self.containerTable = map[string]string{}
	self.notifyQueue = []*PipestanceNotification{}
	self.mailer = mailer
	return self
}

func (self *PipestanceManager) CopyAndClearNotifyQueue() []*PipestanceNotification {
	self.runListMutex.Lock()
	notifyQueue := make([]*PipestanceNotification, len(self.notifyQueue))
	copy(notifyQueue, self.notifyQueue)
	self.notifyQueue = []*PipestanceNotification{}
	self.runListMutex.Unlock()
	return notifyQueue
}

func (self *PipestanceManager) loadCache() {
	bytes, err := ioutil.ReadFile(self.cachePath)
	if err != nil {
		core.LogInfo("pipeman", "Could not read cache file %s.", self.cachePath)
		return
	}

	var cache map[string]map[string]bool
	if err := json.Unmarshal(bytes, &cache); err != nil {
		core.LogError(err, "pipeman", "Could not parse JSON in cache file %s.", self.cachePath)
		return
	}

	if completed, ok := cache["completed"]; ok {
		self.completed = completed
	}
	if failed, ok := cache["failed"]; ok {
		self.failed = failed
	}
	core.LogInfo("pipeman", "%d completed pipestance flags loaded from cache.", len(self.completed))
	core.LogInfo("pipeman", "%d failed pipestance flags loaded from cache.", len(self.failed))
}

func (self *PipestanceManager) writeCache() {
	cache := map[string]map[string]bool{
		"completed": self.completed,
		"failed":    self.failed,
	}
	bytes, _ := json.MarshalIndent(cache, "", "    ")
	if err := ioutil.WriteFile(self.cachePath, bytes, 0644); err != nil {
		core.LogError(err, "pipeman", "Could not write cache file %s.", self.cachePath)
	}
}

func (self *PipestanceManager) getPipestancePath(container string, pipeline string, psid string) string {
	return path.Join(self.path, container, pipeline, psid, "HEAD")
}

func (self *PipestanceManager) inventoryPipestances() {
	// Look for pipestances that are not marked as completed, reattach to them
	// and put them in the runlist.
	core.LogInfo("pipeman", "Begin pipestance inventory.")
	pscount := 0

	// Concurrently step all pipestances in the runlist copy.
	var wg sync.WaitGroup

	// Iterate over top level containers (flowcells).
	containerInfos, _ := ioutil.ReadDir(self.path)
	for _, containerInfo := range containerInfos {
		container := containerInfo.Name()

		// Iterate over all known pipelines.
		for _, pipeline := range self.pipelines {
			psidInfos, _ := ioutil.ReadDir(path.Join(self.path, container, pipeline))

			// Iterate over psids under this pipeline.
			for _, psidInfo := range psidInfos {
				wg.Add(1)
				pscount += 1
				go func(psidInfo os.FileInfo, pipeline string, container string) {
					psid := psidInfo.Name()
					fqname := makeFQName(pipeline, psid)
					defer wg.Done()

					// Cache the fqname to container mapping so we know what container
					// an analysis pipestance is in for notification emails.
					self.runListMutex.Lock()
					self.containerTable[fqname] = container
					if self.completed[fqname] || self.failed[fqname] {
						// If we already know the state of this pipestance, move on.
						self.runListMutex.Unlock()
						return
					}
					self.runListMutex.Unlock()

					// If pipestance has _finalstate, consider it complete.
					if _, err := os.Stat(path.Join(self.getPipestancePath(container, pipeline, psid), "_finalstate")); err == nil {
						self.runListMutex.Lock()
						self.completed[fqname] = true
						self.runListMutex.Unlock()
						return
					}

					pipestance, err := self.rt.ReattachToPipestance(psid, self.getPipestancePath(container, pipeline, psid))
					if err != nil {
						// If we could not reattach, it's because _invocation was
						// missing, or will no longer parse due to changes in MRO
						// definitions. Consider the pipestance failed.
						self.runListMutex.Lock()
						self.failed[fqname] = true
						self.runListMutex.Unlock()
						return
					}

					core.LogInfo("pipeman", "%s is not cached as completed or failed, so pushing onto runList.", fqname)
					self.runListMutex.Lock()
					self.runList = append(self.runList, pipestance)
					self.runTable[fqname] = pipestance
					self.runListMutex.Unlock()
				}(psidInfo, pipeline, container)
			}
		}
	}
	wg.Wait()
	self.runListMutex.Lock()
	self.writeCache()
	self.runListMutex.Unlock()
	core.LogInfo("pipeman", "%d pipestances inventoried.", pscount)
}

// Start an infinite process loop.
func (self *PipestanceManager) goRunListLoop() {
	go func() {
		// Sleep for 5 seconds to let the webserver fail on port rebind.
		time.Sleep(time.Second * time.Duration(5))
		for {
			self.processRunList()

			// Wait for a bit.
			time.Sleep(time.Second * time.Duration(self.stepms))
		}
	}()
}

func parseFQName(fqname string) (string, string) {
	parts := strings.Split(fqname, ".")
	return parts[2], parts[1]
}

func (self *PipestanceManager) processRunList() {
	// Copy the current runlist, then clear it.
	self.runListMutex.Lock()
	runListCopy := self.runList
	self.runList = []*core.Pipestance{}
	self.runListMutex.Unlock()

	// Concurrently step all pipestances in the runlist copy.
	var wg sync.WaitGroup
	wg.Add(len(runListCopy))

	for _, pipestance := range runListCopy {
		go func(pipestance *core.Pipestance, wg *sync.WaitGroup) {
			pipestance.RefreshMetadata()

			state := pipestance.GetState()
			fqname := pipestance.GetFQName()
			if state == "complete" {
				// If pipestance is done, remove from runTable, mark it in the
				// cache as completed, and flush the cache.
				core.LogInfo("pipeman", "Complete and removing from runList: %s.", fqname)
				self.runListMutex.Lock()
				delete(self.runTable, fqname)
				self.completed[fqname] = true
				self.runListMutex.Unlock()

				// Immortalization.
				pipestance.Immortalize()

				// VDR Kill
				core.LogInfo("pipeman", "Starting VDR kill for %s.", fqname)
				killReport := pipestance.VDRKill()
				core.LogInfo("pipeman", "VDR killed %d files, %s from %s.", killReport.Count, humanize.Bytes(killReport.Size), fqname)

				// Email notification.
				pname, psid := parseFQName(fqname)
				if pname == "BCL_PROCESSOR_PD" {
					// For BCL_PROCESSOR_PD, just email the admins.
					self.mailer.Sendmail(
						[]string{},
						fmt.Sprintf("%s of %s has succeeded!", pname, psid),
						fmt.Sprintf("Hey Preppie,\n\n%s of %s is done.\n\nCheck out my rad moves at http://%s/pipestance/%s/%s/%s.\n\nBtw I also saved you %s with VDR. Show me love!", pname, psid, self.mailer.InstanceName, psid, pname, psid, humanize.Bytes(killReport.Size)),
					)
				} else {
					// For ANALYZER_PD, queue up notification for batch email of users.
					self.runListMutex.Lock()
					self.notifyQueue = append(self.notifyQueue, &PipestanceNotification{
						State:     "complete",
						Container: self.containerTable[fqname],
						Pname:     pname,
						Psid:      psid,
						Vdrsize:   killReport.Size,
					})
					self.runListMutex.Unlock()
				}
			} else if state == "failed" {
				// If pipestance is failed, remove from runTable, mark it in the
				// cache as failed, and flush the cache.
				core.LogInfo("pipeman", "Failed and removing from runList: %s.", fqname)
				self.runListMutex.Lock()
				delete(self.runTable, fqname)
				self.failed[fqname] = true
				self.runListMutex.Unlock()

				// Immortalization.
				pipestance.Immortalize()

				// Email notification.
				pname, psid := parseFQName(fqname)
				if pname == "BCL_PROCESSOR_PD" {
					// For BCL_PROCESSOR_PD, just email the admins.
					self.mailer.Sendmail(
						[]string{},
						fmt.Sprintf("%s of %s has failed!", pname, psid),
						fmt.Sprintf("Hey Preppie,\n\n%s of %s failed.\n\nDon't feel bad, but check out what you messed up at http://%s/pipestance/%s/%s/%s.", pname, psid, self.mailer.InstanceName, psid, pname, psid),
					)
				} else {
					// For ANALYZER_PD, queue up notification for batch email of users.
					self.runListMutex.Lock()
					self.notifyQueue = append(self.notifyQueue, &PipestanceNotification{
						State:     "failed",
						Container: self.containerTable[fqname],
						Pname:     pname,
						Psid:      psid,
						Vdrsize:   0,
					})
					self.runListMutex.Unlock()
				}
			} else {
				// If it is not done, put it back on the run list and step the nodes.
				self.runListMutex.Lock()
				self.runList = append(self.runList, pipestance)
				self.runListMutex.Unlock()
				pipestance.StepNodes()
			}
			wg.Done()
		}(pipestance, &wg)
	}

	// Wait for this round of processing to complete, then write
	// out any changes to the complete/fail cache.
	wg.Wait()

	self.runListMutex.Lock()
	self.writeCache()
	self.runListMutex.Unlock()
}

func (self *PipestanceManager) addPendingPipestance(fqname string, unfail bool) {
	self.runListMutex.Lock()
	self.pendingTable[fqname] = true
	if unfail {
		delete(self.failed, fqname)
	}
	self.runListMutex.Unlock()
}

func (self *PipestanceManager) removePendingPipestance(fqname string, unfail bool) {
	self.runListMutex.Lock()
	delete(self.pendingTable, fqname)
	if unfail {
		self.failed[fqname] = true
	}
	self.runListMutex.Unlock()
}

func (self *PipestanceManager) Invoke(container string, pipeline string, psid string, src string) error {
	fqname := makeFQName(pipeline, psid)

	core.LogInfo("pipeman", "Instantiating and pushing to pendingList: %s.", fqname)
	self.addPendingPipestance(fqname, false)

	psPath := path.Join(self.path, container, pipeline, psid, self.mroVersion)
	pipestance, err := self.rt.InvokePipeline(src, "./argshim", psid, psPath)
	if err != nil {
		self.removePendingPipestance(fqname, false)
		return err
	}
	headPath := self.getPipestancePath(container, pipeline, psid)
	os.Remove(headPath)
	os.Symlink(self.mroVersion, headPath)

	core.LogInfo("pipeman", "Finished instantiating and pushing to runList: %s.", fqname)
	self.runListMutex.Lock()
	self.runList = append(self.runList, pipestance)
	self.runTable[fqname] = pipestance
	self.containerTable[fqname] = container
	delete(self.pendingTable, fqname)
	self.runListMutex.Unlock()

	return nil
}

func (self *PipestanceManager) ArchivePipestanceHead(container string, pipeline string, psid string) error {
	self.runListMutex.Lock()
	delete(self.completed, makeFQName(pipeline, psid))
	self.writeCache()
	self.runListMutex.Unlock()
	headPath := self.getPipestancePath(container, pipeline, psid)
	return os.Remove(headPath)
}

func (self *PipestanceManager) unfailPipestance(container string, pipeline string, psid string, nodeFQname string, unfailAll bool) error {
	fqname := makeFQName(pipeline, psid)

	core.LogInfo("pipeman", "Unfailing and pushing to pendingList: %s.", fqname)
	self.addPendingPipestance(fqname, true)

	pipestance, ok := self.GetPipestance(container, pipeline, psid)
	if !ok {
		self.removePendingPipestance(fqname, true)
		return &core.PipestanceNotExistsError{psid}
	}
	var err error
	if unfailAll {
		err = pipestance.Reset()
	} else {
		err = pipestance.ResetNode(nodeFQname)
	}
	if err != nil {
		self.removePendingPipestance(fqname, true)
		return err
	}
	pipestance.Unimmortalize()

	core.LogInfo("pipeman", "Finished unfailing and pushing to runList: %s.", fqname)
	self.runListMutex.Lock()
	self.writeCache()
	self.runList = append(self.runList, pipestance)
	self.runTable[fqname] = pipestance
	delete(self.pendingTable, fqname)
	self.runListMutex.Unlock()
	return nil
}

func (self *PipestanceManager) UnfailPipestanceNode(container string, pipeline string, psid string, nodeFQname string) error {
	return self.unfailPipestance(container, pipeline, psid, nodeFQname, false)
}

func (self *PipestanceManager) UnfailPipestance(container string, pipeline string, psid string) error {
	return self.unfailPipestance(container, pipeline, psid, "", true)
}

func (self *PipestanceManager) GetPipestanceState(container string, pipeline string, psid string) (string, bool) {
	fqname := makeFQName(pipeline, psid)
	self.runListMutex.Lock()
	if _, ok := self.completed[fqname]; ok {
		self.runListMutex.Unlock()
		return "complete", true
	}
	if _, ok := self.failed[fqname]; ok {
		self.runListMutex.Unlock()
		return "failed", true
	}
	if run, ok := self.runTable[fqname]; ok {
		self.runListMutex.Unlock()
		return run.GetState(), true
	}
	if _, ok := self.pendingTable[fqname]; ok {
		self.runListMutex.Unlock()
		return "waiting", true
	}
	self.runListMutex.Unlock()
	return "", false
}

func (self *PipestanceManager) GetPipestanceSerialization(container string, pipeline string, psid string) (interface{}, bool) {
	psPath := self.getPipestancePath(container, pipeline, psid)
	if ser, ok := self.rt.GetSerialization(psPath); ok {
		return ser, true
	}
	pipestance, ok := self.GetPipestance(container, pipeline, psid)
	if !ok {
		return nil, false
	}
	return pipestance.Serialize(), true
}

func (self *PipestanceManager) GetPipestance(container string, pipeline string, psid string) (*core.Pipestance, bool) {
	fqname := makeFQName(pipeline, psid)

	// Check if requested pipestance actually exists.
	if _, ok := self.GetPipestanceState(container, pipeline, psid); !ok {
		return nil, false
	}

	// Check the runTable.
	self.runListMutex.Lock()
	if pipestance, ok := self.runTable[fqname]; ok {
		self.runListMutex.Unlock()
		return pipestance, true
	}
	self.runListMutex.Unlock()

	// Reattach to the pipestance.
	pipestance, err := self.rt.ReattachToPipestance(psid, self.getPipestancePath(container, pipeline, psid))
	if err != nil {
		return nil, false
	}

	// Refresh its metadata state and return.
	pipestance.RefreshMetadata()
	return pipestance, true
}

func (self *PipestanceManager) GetPipestanceInvokeSrc(container string, pipeline string, psid string) (string, error) {
	psPath := self.getPipestancePath(container, pipeline, psid)
	fname := "_invocation"

	data, err := ioutil.ReadFile(path.Join(psPath, fname))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
