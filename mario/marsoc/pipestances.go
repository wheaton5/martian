//
// Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
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
	self.containerTable = map[string]string{}
	self.notifyQueue = []*PipestanceNotification{}
	self.mailer = mailer
	return self
}

func (self *PipestanceManager) CopyAndClearNotifyQueue() []*PipestanceNotification {
	notifyQueue := make([]*PipestanceNotification, len(self.notifyQueue))
	copy(notifyQueue, self.notifyQueue)
	self.notifyQueue = []*PipestanceNotification{}
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
	if err := ioutil.WriteFile(self.cachePath, bytes, 0600); err != nil {
		core.LogError(err, "pipeman", "Could not write cache file %s.", self.cachePath)
	}
}

func (self *PipestanceManager) inventoryPipestances() {
	// Look for pipestances that are not marked as completed, reattach to them
	// and put them in the runlist.
	core.LogInfo("pipeman", "Begin pipestance inventory.")
	pscount := 0

	// Iterate over top level containers (flowcells).
	containerInfos, _ := ioutil.ReadDir(self.path)
	for _, containerInfo := range containerInfos {
		container := containerInfo.Name()

		// Iterate over all known pipelines.
		for _, pipeline := range self.pipelines {
			psidInfos, _ := ioutil.ReadDir(path.Join(self.path, container, pipeline))

			// Iterate over psids under this pipeline.
			for _, psidInfo := range psidInfos {
				pscount += 1
				psid := psidInfo.Name()

				fqname := makeFQName(pipeline, psid)

				// Cache the fqname to container mapping so we know what container
				// an analysis pipestance is in for notification emails.
				self.containerTable[fqname] = container

				if self.completed[fqname] || self.failed[fqname] {
					continue
				}
				pipestance, err := self.rt.ReattachToPipestance(psid, path.Join(self.path, container, pipeline, psid, "HEAD"))
				if err != nil {
					core.LogError(err, "pipeman", "%s was previously cached but no longer exists.", fqname)
					self.writeCache()
					continue
				}
				core.LogInfo("pipeman", "%s is not cached as completed or failed, so pushing onto runList.", fqname)
				self.runListMutex.Lock()
				self.runList = append(self.runList, pipestance)
				self.runTable[fqname] = pipestance
				self.runListMutex.Unlock()
			}
		}
	}
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

func (self *PipestanceManager) Invoke(container string, pipeline string, psid string, src string) error {
	psPath := path.Join(self.path, container, pipeline, psid, self.mroVersion)
	pipestance, err := self.rt.InvokePipeline(src, "./argshim", psid, psPath)
	if err != nil {
		return err
	}
	fqname := pipestance.GetFQName()
	headPath := path.Join(self.path, container, pipeline, psid, "HEAD")
	os.Remove(headPath)
	os.Symlink(self.mroVersion, headPath)

	core.LogInfo("pipeman", "Instantiating and pushing to runList: %s.", fqname)
	self.runListMutex.Lock()
	self.runList = append(self.runList, pipestance)
	self.runTable[fqname] = pipestance
	self.containerTable[fqname] = container
	self.runListMutex.Unlock()

	return nil
}

func (self *PipestanceManager) ArchivePipestanceHead(container string, pipeline string, psid string) error {
	delete(self.completed, makeFQName(pipeline, psid))
	self.writeCache()
	headPath := path.Join(self.path, container, pipeline, psid, "HEAD")
	return os.Remove(headPath)
}

func (self *PipestanceManager) UnfailPipestance(container string, pipeline string, psid string, nodeFQname string) {
	pipestance, ok := self.GetPipestance(container, pipeline, psid)
	if !ok {
		return
	}
	pipestance.ResetNode(nodeFQname)
	pipestance.Unimmortalize()
	delete(self.failed, pipestance.GetFQName())
	self.writeCache()
	self.runListMutex.Lock()
	self.runList = append(self.runList, pipestance)
	self.runTable[pipestance.GetFQName()] = pipestance
	self.runListMutex.Unlock()
}

func (self *PipestanceManager) GetPipestanceState(container string, pipeline string, psid string) (string, bool) {
	fqname := makeFQName(pipeline, psid)
	if _, ok := self.completed[fqname]; ok {
		return "complete", true
	}
	if _, ok := self.failed[fqname]; ok {
		return "failed", true
	}
	if run, ok := self.runTable[fqname]; ok {
		return run.GetState(), true
	}
	return "", false
}

func (self *PipestanceManager) GetPipestanceSerialization(container string, pipeline string, psid string) (interface{}, bool) {
	psPath := path.Join(self.path, container, pipeline, psid, "HEAD")
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
	if pipestance, ok := self.runTable[fqname]; ok {
		return pipestance, true
	}

	// Reattach to the pipestance.
	pipestance, err := self.rt.ReattachToPipestance(psid, path.Join(self.path, container, pipeline, psid, "HEAD"))
	if err != nil {
		return nil, false
	}

	// Refresh its metadata state and return.
	pipestance.RefreshMetadata()
	return pipestance, true
}
