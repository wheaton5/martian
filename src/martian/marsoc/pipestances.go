//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Marsoc pipestance management.
//
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"martian/core"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
)

var minBytesAvailable uint64 = 1024 * 1024 * 1024 * 1024 * 1.5 // 1.5 terabytes

func makeFQName(pipeline string, psid string) string {
	// This construction must remain identical to Pipestance::GetFQName.
	return fmt.Sprintf("ID.%s.%s", psid, pipeline)
}

func makePipestancePath(pipestancesPath string, container string, pipeline string, psid string) string {
	return path.Join(pipestancesPath, container, pipeline, psid, "HEAD")
}

type PipestanceFunc func(string, string, string, os.FileInfo, *sync.WaitGroup)

type PipestanceNotification struct {
	State     string
	Container string
	Pname     string
	Psid      string
	Vdrsize   uint64
}

type AnalysisNotification struct {
	Fcid string
}

type PipestanceManager struct {
	rt             *core.Runtime
	martianVersion string
	mroVersion     string
	cachePath      string
	autoInvoke     bool
	stepms         int
	writePath      string
	scratchIndex   int
	scratchPaths   []string
	paths          []string
	pipelines      []string
	completed      map[string]bool
	failed         map[string]bool
	runList        []*core.Pipestance
	runListMutex   *sync.Mutex
	runTable       map[string]*core.Pipestance
	pendingTable   map[string]bool
	copyTable      map[string]bool
	containerTable map[string]string
	pathTable      map[string]string
	mailQueue      []*PipestanceNotification
	analysisQueue  []*AnalysisNotification
	mailer         *Mailer
}

func NewPipestanceManager(rt *core.Runtime, martianVersion string,
	mroVersion string, pipestancesPaths []string, scratchPaths []string, cachePath string,
	stepms int, autoInvoke bool, mailer *Mailer) *PipestanceManager {
	self := &PipestanceManager{}
	self.rt = rt
	self.martianVersion = martianVersion
	self.mroVersion = mroVersion
	self.paths = pipestancesPaths
	self.writePath = pipestancesPaths[len(pipestancesPaths)-1]
	self.scratchPaths = scratchPaths
	self.scratchIndex = 0
	self.cachePath = path.Join(cachePath, "pipestances")
	self.stepms = stepms
	self.autoInvoke = autoInvoke
	self.pipelines = rt.PipelineNames
	self.completed = map[string]bool{}
	self.failed = map[string]bool{}
	self.runList = []*core.Pipestance{}
	self.runListMutex = &sync.Mutex{}
	self.runTable = map[string]*core.Pipestance{}
	self.pendingTable = map[string]bool{}
	self.copyTable = map[string]bool{}
	self.containerTable = map[string]string{}
	self.pathTable = map[string]string{}
	self.mailQueue = []*PipestanceNotification{}
	self.analysisQueue = []*AnalysisNotification{}
	self.mailer = mailer
	return self
}

func (self *PipestanceManager) CopyAndClearMailQueue() []*PipestanceNotification {
	self.runListMutex.Lock()
	mailQueue := make([]*PipestanceNotification, len(self.mailQueue))
	copy(mailQueue, self.mailQueue)
	self.mailQueue = []*PipestanceNotification{}
	self.runListMutex.Unlock()
	return mailQueue
}

func (self *PipestanceManager) CopyAndClearAnalysisQueue() []*AnalysisNotification {
	self.runListMutex.Lock()
	analysisQueue := make([]*AnalysisNotification, len(self.analysisQueue))
	copy(analysisQueue, self.analysisQueue)
	self.analysisQueue = []*AnalysisNotification{}
	self.runListMutex.Unlock()
	return analysisQueue
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

func (self *PipestanceManager) getPipestancePath(container string, pipeline string, psid string) (string, error) {
	self.runListMutex.Lock()
	defer self.runListMutex.Unlock()

	fqname := makeFQName(pipeline, psid)
	if pipestancePath, ok := self.pathTable[fqname]; ok {
		return pipestancePath, nil
	}
	return "", &core.PipestanceNotExistsError{fqname}
}

func (self *PipestanceManager) traversePipestancesPaths(pipestancesPaths []string, pipestanceFunc PipestanceFunc) int {
	var wg sync.WaitGroup
	pscount := 0

	for _, pipestancesPath := range pipestancesPaths {
		containerInfos, _ := ioutil.ReadDir(pipestancesPath)
		for _, containerInfo := range containerInfos {
			container := containerInfo.Name()
			for _, pipeline := range self.pipelines {
				psidInfos, _ := ioutil.ReadDir(path.Join(pipestancesPath, container, pipeline))
				for _, psidInfo := range psidInfos {
					wg.Add(1)
					pscount += 1
					go pipestanceFunc(pipestancesPath, container, pipeline, psidInfo, &wg)
				}
			}
		}
	}
	wg.Wait()
	return pscount
}

func (self *PipestanceManager) inventoryPipestances() {
	// Look for pipestances that are not marked as completed, reattach to them
	// and put them in the runlist.
	core.LogInfo("pipeman", "Begin pipestance inventory.")

	// Concurrently step all pipestances in the runlist copy.
	scratchPsPaths := map[string]bool{}

	pscount := self.traversePipestancesPaths(self.paths,
		func(pipestancesPath string, container string, pipeline string, psidInfo os.FileInfo, wg *sync.WaitGroup) {
			psid := psidInfo.Name()
			psPath := path.Join(pipestancesPath, container, pipeline, psid)
			defer wg.Done()

			// If psid has .tmp suffix and no psid without .tmp suffix exists,
			// this pipestance was about to be renamed prior to Marsoc shutdown
			if strings.HasSuffix(psid, ".tmp") {
				permanentPsid := strings.TrimSuffix(psid, ".tmp")
				newPsPath := path.Join(pipestancesPath, container, pipeline, permanentPsid)
				if _, err := os.Stat(newPsPath); err == nil {
					return
				}
				os.Rename(psPath, newPsPath)

				psid = permanentPsid
				psPath = newPsPath
			}
			fqname := makeFQName(pipeline, psid)

			// Cache the fqname to container mapping so we know what container
			// an analysis pipestance is in for notification emails.
			self.runListMutex.Lock()
			hardPsPath, _ := filepath.EvalSymlinks(psPath)
			scratchPsPaths[hardPsPath] = true
			self.containerTable[fqname] = container
			self.pathTable[fqname] = makePipestancePath(pipestancesPath, container, pipeline, psid)
			// If we already know the state of this pipestance, move on.
			if self.completed[fqname] {
				self.copyPipestance(fqname)
				self.runListMutex.Unlock()
				return
			}
			if self.failed[fqname] {
				self.runListMutex.Unlock()
				return
			}
			self.runListMutex.Unlock()

			// If pipestance has _finalstate, consider it complete.
			if _, err := os.Stat(path.Join(makePipestancePath(pipestancesPath, container, pipeline, psid), "_finalstate")); err == nil {
				self.runListMutex.Lock()
				self.completed[fqname] = true
				self.copyPipestance(fqname)
				self.runListMutex.Unlock()
				return
			}

			pipestance, err := self.rt.ReattachToPipestance(psid, makePipestancePath(pipestancesPath, container, pipeline, psid))
			if err != nil {
				// If we could not reattach, it's because _invocation was
				// missing, or will no longer parse due to changes in MRO
				// definitions. Consider the pipestance failed.
				self.runListMutex.Lock()
				self.failed[fqname] = true
				self.runListMutex.Unlock()
				return
			}

			pipestance.LoadMetadata()

			core.LogInfo("pipeman", "%s is not cached as completed or failed, so pushing onto runList.", fqname)
			self.runListMutex.Lock()
			self.runList = append(self.runList, pipestance)
			self.runTable[fqname] = pipestance
			self.runListMutex.Unlock()
		})
	self.runListMutex.Lock()
	self.writeCache()
	self.runListMutex.Unlock()
	core.LogInfo("pipeman", "%d pipestances inventoried.", pscount)

	core.LogInfo("pipeman", "Begin scratch directory cleanup.")
	var wg sync.WaitGroup
	for _, scratchPath := range self.scratchPaths {
		scratchPsInfos, _ := ioutil.ReadDir(scratchPath)
		for _, scratchPsInfo := range scratchPsInfos {
			scratchPsPath := path.Join(scratchPath, scratchPsInfo.Name())
			if _, ok := scratchPsPaths[scratchPsPath]; !ok {
				core.LogInfo("pipeman", "Removing scratch directory %s", scratchPsPath)
				wg.Add(1)
				go func(scratchPsPath string) {
					defer wg.Done()
					os.RemoveAll(scratchPsPath)
				}(scratchPsPath)
			}
		}
	}
	wg.Wait()
	core.LogInfo("pipeman", "Finished scratch directory cleanup.")
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

func (self *PipestanceManager) copyPipestance(fqname string) {
	psPath := path.Dir(self.pathTable[fqname])
	pname, psid := parseFQName(fqname)
	if fileinfo, _ := os.Lstat(psPath); fileinfo.Mode()&os.ModeSymlink == os.ModeSymlink {
		// Check to make sure this isn't already being copied
		if _, ok := self.copyTable[fqname]; ok {
			return
		}
		self.copyTable[fqname] = true
		go func() {
			newPsPath := psPath + ".tmp"
			hardPsPath, _ := filepath.EvalSymlinks(psPath)
			os.RemoveAll(newPsPath)
			err := filepath.Walk(hardPsPath, func(oldPath string, fileinfo os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				relPath, _ := filepath.Rel(hardPsPath, oldPath)
				newPath := path.Join(newPsPath, relPath)

				if fileinfo.Mode().IsDir() {
					err = os.Mkdir(newPath, 0755)
				}

				if fileinfo.Mode().IsRegular() {
					in, _ := os.Open(oldPath)
					defer in.Close()

					out, _ := os.Create(newPath)
					defer out.Close()

					_, err = io.Copy(out, in)
				}

				if fileinfo.Mode()&os.ModeSymlink == os.ModeSymlink {
					oldPath, _ = os.Readlink(oldPath)
					err = os.Symlink(oldPath, newPath)
				}
				return err
			})
			if err == nil {
				os.Remove(psPath)
				os.Rename(newPsPath, psPath)
				os.RemoveAll(hardPsPath)
			} else {
				container := self.containerTable[fqname]
				self.mailer.Sendmail(
					[]string{},
					fmt.Sprintf("%s of %s copy failed!", pname, psid),
					fmt.Sprintf("Hey Preppie,\n\n%s of %s/%s at %s failed to copy:\n\n%s", pname, container, psid, psPath, err.Error()),
				)
			}

			self.runListMutex.Lock()
			delete(self.copyTable, fqname)
			self.runListMutex.Unlock()
			if err == nil && pname == "BCL_PROCESSOR_PD" {
				self.runListMutex.Lock()
				self.analysisQueue = append(self.analysisQueue, &AnalysisNotification{Fcid: psid})
				self.runListMutex.Unlock()
			}
		}()
	}
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
			pipestance.RefreshState()

			state := pipestance.GetState()
			fqname := pipestance.GetFQName()
			if state == "complete" {
				// If pipestance is done, remove from runTable, mark it in the
				// cache as completed, and flush the cache.
				core.LogInfo("pipeman", "Complete and removing from runList: %s.", fqname)

				// Cleanup
				pipestance.Cleanup()

				// Immortalization.
				pipestance.Immortalize()

				// VDR Kill
				core.LogInfo("pipeman", "Starting VDR kill for %s.", fqname)
				killReport := pipestance.VDRKill()
				core.LogInfo("pipeman", "VDR killed %d files, %s from %s.", killReport.Count, humanize.Bytes(killReport.Size), fqname)

				self.runListMutex.Lock()
				delete(self.runTable, fqname)
				self.completed[fqname] = true
				self.copyPipestance(fqname)
				self.runListMutex.Unlock()

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
					self.mailQueue = append(self.mailQueue, &PipestanceNotification{
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

				// Immortalization.
				pipestance.Immortalize()

				self.runListMutex.Lock()
				delete(self.runTable, fqname)
				self.failed[fqname] = true
				self.runListMutex.Unlock()

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
					self.mailQueue = append(self.mailQueue, &PipestanceNotification{
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

func (self *PipestanceManager) removePendingPipestance(fqname string, unfail bool) {
	self.runListMutex.Lock()
	delete(self.pendingTable, fqname)
	if unfail {
		self.failed[fqname] = true
	}
	self.runListMutex.Unlock()
}

func (self *PipestanceManager) getScratchPath() (string, error) {
	i := 0
	for i < len(self.scratchPaths) {
		scratchPath := self.scratchPaths[self.scratchIndex]
		self.scratchIndex = (self.scratchIndex + 1) % len(self.scratchPaths)

		var stat syscall.Statfs_t
		if err := syscall.Statfs(scratchPath, &stat); err == nil {
			bytesAvailable := stat.Bavail * uint64(stat.Bsize)
			if bytesAvailable >= minBytesAvailable {
				return scratchPath, nil
			}
		}
		i += 1
	}
	return "", &core.MartianError{fmt.Sprintf("Pipestance scratch paths %s are full.", strings.Join(self.scratchPaths, ", "))}
}

func (self *PipestanceManager) Invoke(container string, pipeline string, psid string, src string) error {
	fqname := makeFQName(pipeline, psid)

	self.runListMutex.Lock()
	// Check if pipestance has already been invoked
	if _, ok := self.getPipestanceState(container, pipeline, psid); ok {
		self.runListMutex.Unlock()
		return &core.PipestanceExistsError{psid}
	}
	scratchPath, err := self.getScratchPath()
	if err != nil {
		self.runListMutex.Unlock()
		return err
	}
	self.pendingTable[fqname] = true
	self.runListMutex.Unlock()
	core.LogInfo("pipeman", "Instantiating and pushed to pendingList: %s.", fqname)

	psDir := path.Join(self.writePath, container, pipeline, psid)
	scratchDir := path.Join(scratchPath, fmt.Sprintf("%s.%s.%s", container, pipeline, psid))
	if _, err := os.Stat(psDir); err != nil {
		os.RemoveAll(scratchDir)
		os.MkdirAll(scratchDir, 0755)
		os.MkdirAll(path.Dir(psDir), 0755)
		os.Symlink(scratchDir, psDir)
	}
	psPath := path.Join(psDir, self.mroVersion)

	pipestance, err := self.rt.InvokePipeline(src, "./argshim", psid, psPath)
	if err != nil {
		self.removePendingPipestance(fqname, false)
		return err
	}
	headPath := makePipestancePath(self.writePath, container, pipeline, psid)
	os.Remove(headPath)
	os.Symlink(psPath, headPath)

	pipestance.LoadMetadata()

	core.LogInfo("pipeman", "Finished instantiating and pushing to runList: %s.", fqname)
	self.runListMutex.Lock()
	delete(self.pendingTable, fqname)
	self.runList = append(self.runList, pipestance)
	self.runTable[fqname] = pipestance
	self.containerTable[fqname] = container
	self.pathTable[fqname] = headPath
	self.runListMutex.Unlock()

	return nil
}

func (self *PipestanceManager) ArchivePipestanceHead(container string, pipeline string, psid string) error {
	self.runListMutex.Lock()
	delete(self.completed, makeFQName(pipeline, psid))
	self.writeCache()
	self.runListMutex.Unlock()
	headPath, err := self.getPipestancePath(container, pipeline, psid)
	if err != nil {
		return err
	}
	return os.Remove(headPath)
}

func (self *PipestanceManager) UnfailPipestance(container string, pipeline string, psid string) error {
	fqname := makeFQName(pipeline, psid)

	self.runListMutex.Lock()
	state, _ := self.getPipestanceState(container, pipeline, psid)
	// Check if pipestance is being copied right now.
	if state == "copying" {
		self.runListMutex.Unlock()
		return &core.PipestanceCopyingError{psid}
	}
	// Check if pipestance is failed
	if state != "failed" {
		self.runListMutex.Unlock()
		return &core.PipestanceNotFailedError{psid}
	}
	delete(self.failed, fqname)
	self.pendingTable[fqname] = true
	self.runListMutex.Unlock()
	core.LogInfo("pipeman", "Unfailing and pushed to pendingList: %s.", fqname)

	pipestance, ok := self.GetPipestance(container, pipeline, psid)
	if !ok {
		self.removePendingPipestance(fqname, true)
		return &core.PipestanceNotExistsError{psid}
	}
	if err := pipestance.Reset(); err != nil {
		self.removePendingPipestance(fqname, true)
		return err
	}
	pipestance.Unimmortalize()

	core.LogInfo("pipeman", "Finished unfailing and pushing to runList: %s.", fqname)
	self.runListMutex.Lock()
	self.writeCache()
	delete(self.pendingTable, fqname)
	self.runList = append(self.runList, pipestance)
	self.runTable[fqname] = pipestance
	self.runListMutex.Unlock()
	return nil
}

func (self *PipestanceManager) getPipestanceState(container string, pipeline string, psid string) (string, bool) {
	fqname := makeFQName(pipeline, psid)
	if _, ok := self.copyTable[fqname]; ok {
		return "copying", true
	}
	if _, ok := self.completed[fqname]; ok {
		return "complete", true
	}
	if _, ok := self.failed[fqname]; ok {
		return "failed", true
	}
	if run, ok := self.runTable[fqname]; ok {
		return run.GetState(), true
	}
	if _, ok := self.pendingTable[fqname]; ok {
		return "waiting", true
	}
	return "", false
}

func (self *PipestanceManager) GetPipestanceState(container string, pipeline string, psid string) (string, bool) {
	self.runListMutex.Lock()
	state, ok := self.getPipestanceState(container, pipeline, psid)
	self.runListMutex.Unlock()
	return state, ok
}

func (self *PipestanceManager) GetPipestanceSerialization(container string, pipeline string, psid string) (interface{}, bool) {
	psPath, err := self.getPipestancePath(container, pipeline, psid)
	if err != nil {
		return nil, false
	}
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

	// Get pipestance path.
	psPath, err := self.getPipestancePath(container, pipeline, psid)
	if err != nil {
		return nil, false
	}

	// Reattach to the pipestance.
	pipestance, err := self.rt.ReattachToPipestance(psid, psPath)
	if err != nil {
		return nil, false
	}

	// Load its metadata and return.
	pipestance.LoadMetadata()
	return pipestance, true
}

func (self *PipestanceManager) GetPipestanceInvokeSrc(container string, pipeline string, psid string) (string, error) {
	psPath, err := self.getPipestancePath(container, pipeline, psid)
	if err != nil {
		return "", err
	}

	fname := "_invocation"
	data, err := ioutil.ReadFile(path.Join(psPath, fname))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (self *PipestanceManager) GetPipestanceOuts(container string, pipeline string, psid string, forkIndex int) map[string]interface{} {
	if psPath, err := self.getPipestancePath(container, pipeline, psid); err == nil {
		fpath := path.Join(psPath, pipeline, fmt.Sprintf("fork%d", forkIndex), "_outs")
		if data, err := ioutil.ReadFile(fpath); err == nil {
			var v map[string]interface{}
			if err := json.Unmarshal(data, &v); err == nil {
				return v
			}
		}
	}
	return map[string]interface{}{}
}
