//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Marsoc pipestance management.
//
package manager

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"martian/core"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
)

const minBytesAvailable = 1024 * 1024 * 1024 * 1024 * 1.5 // 1.5 terabytes
const scratchTimeout = 24 * 7 * 2                         // 2 weeks

type PipestanceFunc func(string, string, string) error
type PipestanceInventoryFunc func(string, string, string, string, string, *sync.WaitGroup)

type PipestanceNotification struct {
	Name      string
	State     string
	Container string
	Pname     string
	Psid      string
	Stage     string
	Summary   string
	Vdrsize   uint64
}

type AnalysisNotification struct {
	Fcid string
}

type PipestanceManager struct {
	rt            *core.Runtime
	cachePath     string
	autoInvoke    bool
	runLoop       bool
	stepms        int
	aggregatePath string
	writePath     string
	failCoopPath  string
	scratchIndex  int
	scratchPaths  []string
	paths         []string
	pipelines     []string
	completed     map[string]bool
	failed        map[string]bool
	mutex         *sync.Mutex
	running       map[string]*core.Pipestance
	pending       map[string]bool
	copying       map[string]bool
	mailQueue     []*PipestanceNotification
	analysisQueue []*AnalysisNotification
	mailer        *Mailer
	packages      PackageManager
}

func makePipestanceKey(container string, pipeline string, psid string) string {
	return fmt.Sprintf("%s.%s.%s", container, pipeline, psid)
}

func parsePipestanceKey(pkey string) (string, string, string) {
	parts := strings.Split(pkey, ".")
	return parts[0], parts[1], parts[2]
}

func writeJson(fpath string, object interface{}) {
	bytes, _ := json.MarshalIndent(object, "", "    ")
	if err := ioutil.WriteFile(fpath, bytes, 0644); err != nil {
		core.LogError(err, "pipeman", "Could not write JSON file %s.", fpath)
	}
}

func copyFile(oldPath string, newPath string) error {
	in, _ := os.Open(oldPath)
	defer in.Close()

	out, _ := os.Create(newPath)
	defer out.Close()

	_, err := io.Copy(out, in)
	return err
}

func deleteJobs(fqname string) ([]byte, error) {
	cmd := exec.Command("qdel", fmt.Sprintf("%s*", fqname))
	return cmd.CombinedOutput()
}

func getFilenameWithSuffix(dir string, fname string) string {
	suffix := 0
	infos, _ := ioutil.ReadDir(dir)
	re := regexp.MustCompile(fmt.Sprintf("^%s-(\\d+)$", fname))
	for _, info := range infos {
		if m := re.FindStringSubmatch(info.Name()); m != nil {
			infoSuffix, _ := strconv.Atoi(m[1])
			if suffix <= infoSuffix {
				suffix = infoSuffix + 1
			}
		}
	}
	return fmt.Sprintf("%s-%d", fname, suffix)
}

func NewPipestanceManager(rt *core.Runtime, pipestancesPaths []string, scratchPaths []string,
	cachePath string, failCoopPath string, stepms int, autoInvoke bool, mailer *Mailer,
	packages PackageManager) *PipestanceManager {
	self := &PipestanceManager{}
	self.rt = rt
	self.paths = pipestancesPaths
	self.aggregatePath = pipestancesPaths[0]
	self.writePath = pipestancesPaths[len(pipestancesPaths)-1]
	self.scratchPaths = scratchPaths
	self.scratchIndex = 0
	self.cachePath = path.Join(cachePath, "pipestances")
	self.failCoopPath = failCoopPath
	self.stepms = stepms
	self.autoInvoke = autoInvoke
	self.runLoop = true
	self.pipelines = rt.MroCache.GetPipelines()
	self.completed = map[string]bool{}
	self.failed = map[string]bool{}
	self.mutex = &sync.Mutex{}
	self.running = map[string]*core.Pipestance{}
	self.pending = map[string]bool{}
	self.copying = map[string]bool{}
	self.mailQueue = []*PipestanceNotification{}
	self.analysisQueue = []*AnalysisNotification{}
	self.mailer = mailer
	self.packages = packages
	return self
}

func (self *PipestanceManager) GetAutoInvoke() bool {
	return self.autoInvoke
}

func (self *PipestanceManager) SetAutoInvoke(autoInvoke bool) {
	core.LogInfo("pipeman", "Setting autoinvoke = %v", autoInvoke)
	self.autoInvoke = autoInvoke
}

func (self *PipestanceManager) EnableRunLoop() {
	core.LogInfo("pipeman", "Enabling run loop.")
	self.runLoop = true
}

func (self *PipestanceManager) DisableRunLoop() {
	core.LogInfo("pipeman", "Disabling run loop.")
	self.runLoop = false
}

func (self *PipestanceManager) CountRunningPipestances() int {
	self.mutex.Lock()
	count := len(self.running)
	self.mutex.Unlock()
	return count
}

func (self *PipestanceManager) CopyAndClearMailQueue() []*PipestanceNotification {
	self.mutex.Lock()
	mailQueue := make([]*PipestanceNotification, len(self.mailQueue))
	copy(mailQueue, self.mailQueue)
	self.mailQueue = []*PipestanceNotification{}
	self.mutex.Unlock()
	return mailQueue
}

func (self *PipestanceManager) CopyAndClearAnalysisQueue() []*AnalysisNotification {
	self.mutex.Lock()
	analysisQueue := make([]*AnalysisNotification, len(self.analysisQueue))
	copy(analysisQueue, self.analysisQueue)
	self.analysisQueue = []*AnalysisNotification{}
	self.mutex.Unlock()
	return analysisQueue
}

func (self *PipestanceManager) LoadPipestances() {
	if err := self.loadCache(); err != nil {
		self.inventoryPipestances()
	}
}

func (self *PipestanceManager) loadCache() error {
	bytes, err := ioutil.ReadFile(self.cachePath)
	if err != nil {
		core.LogInfo("pipeman", "Could not read cache file %s.", self.cachePath)
		return err
	}

	var cache map[string]map[string]bool
	if err := json.Unmarshal(bytes, &cache); err != nil {
		core.LogError(err, "pipeman", "Could not parse JSON in cache file %s.", self.cachePath)
		return err
	}

	if completed, ok := cache["completed"]; ok {
		self.completed = completed
	}
	if failed, ok := cache["failed"]; ok {
		self.failed = failed
	}
	if copying, ok := cache["copying"]; ok {
		for pkey, _ := range copying {
			self.copyPipestance(pkey)
		}
	}
	if running, ok := cache["running"]; ok {
		var wg sync.WaitGroup

		// Load running pipestance in parallel.
		wg.Add(len(running))
		for pkey, _ := range running {
			go func(pkey string) {
				defer wg.Done()
				self.loadPipestance(pkey)
			}(pkey)
		}
		wg.Wait()
	}
	core.LogInfo("pipeman", "%d completed pipestances loaded from cache.", len(self.completed))
	core.LogInfo("pipeman", "%d failed pipestances loaded from cache.", len(self.failed))
	core.LogInfo("pipeman", "%d copying pipestances loaded from cache.", len(self.copying))
	core.LogInfo("pipeman", "%d running pipestances loaded from cache.", len(self.running))

	return nil
}

func (self *PipestanceManager) loadPipestance(pkey string) {
	container, pipeline, psid := parsePipestanceKey(pkey)
	psPath := self.makePipestancePath(container, pipeline, psid)

	// If pipestance has _finalstate, consider it complete.
	if _, err := os.Stat(path.Join(psPath, "_finalstate")); err == nil {
		self.mutex.Lock()
		self.completed[pkey] = true
		self.copyPipestance(pkey)
		self.mutex.Unlock()
		return
	}

	readOnly := false
	pipestance, err := self.ReattachToPipestance(container, pipeline, psid, psPath, readOnly)
	if err != nil {
		// If we could not reattach, it's because _invocation was
		// missing, or will no longer parse due to changes in MRO
		// definitions. Consider the pipestance failed.
		self.mutex.Lock()
		self.failed[pkey] = true
		self.mutex.Unlock()
		return
	}

	pipestance.LoadMetadata()

	core.LogInfo("pipeman", "%s is not cached as completed or failed, so pushing onto run list.", pkey)
	self.mutex.Lock()
	self.running[pkey] = pipestance
	self.mutex.Unlock()
}

func (self *PipestanceManager) writeCache() {
	running := map[string]bool{}

	for pkey, _ := range self.running {
		running[pkey] = true
	}

	cache := map[string]map[string]bool{
		"completed": self.completed,
		"failed":    self.failed,
		"copying":   self.copying,
		"running":   running,
	}
	writeJson(self.cachePath, cache)
}

func (self *PipestanceManager) traversePipestancesPaths(pipestancesPaths []string, pipestanceInventoryFunc PipestanceInventoryFunc) int {
	var wg sync.WaitGroup
	pscount := 0

	for _, pipestancesPath := range pipestancesPaths {
		containerInfos, _ := ioutil.ReadDir(pipestancesPath)
		for _, containerInfo := range containerInfos {
			container := containerInfo.Name()
			for _, pipeline := range self.pipelines {
				psidInfos, _ := ioutil.ReadDir(path.Join(pipestancesPath, container, pipeline))
				for _, psidInfo := range psidInfos {
					psid := psidInfo.Name()
					pscount += 1
					mroVersionInfos, _ := ioutil.ReadDir(path.Join(pipestancesPath, container, pipeline, psid))
					for _, mroVersionInfo := range mroVersionInfos {
						wg.Add(1)
						mroVersion := mroVersionInfo.Name()
						go pipestanceInventoryFunc(pipestancesPath, container, pipeline, psid, mroVersion, &wg)
					}
				}
			}
		}
	}
	wg.Wait()
	return pscount
}

func (self *PipestanceManager) inventoryPipestances() {
	// Look for pipestances that are not marked as completed, reattach to them
	// and put them in the run list.
	core.LogInfo("pipeman", "Begin pipestance inventory.")

	self.traversePipestancesPaths(self.paths,
		func(pipestancesPath string, container string, pipeline string, psid string, mroVersion string, wg *sync.WaitGroup) {
			psPath := path.Join(pipestancesPath, container, pipeline, psid, mroVersion)
			defer wg.Done()

			// If mroVersion has .tmp suffix and no mroVersion without .tmp suffix exists,
			// this pipestance was about to be renamed prior to Marsoc shutdown
			if strings.HasSuffix(mroVersion, ".tmp") {
				permanentMroVersion := strings.TrimSuffix(mroVersion, ".tmp")
				newPsPath := path.Join(pipestancesPath, container, pipeline, psid, permanentMroVersion)
				if _, err := os.Stat(newPsPath); err != nil {
					os.Rename(psPath, newPsPath)
				}
			}
		})
	pscount := self.traversePipestancesPaths([]string{self.aggregatePath},
		func(pipestancesPath string, container string, pipeline string, psid string, mroVersion string, wg *sync.WaitGroup) {
			pkey := makePipestanceKey(container, pipeline, psid)
			defer wg.Done()

			// Only continue process non-archived pipestances
			if mroVersion != "HEAD" {
				return
			}

			self.loadPipestance(pkey)
		})
	self.mutex.Lock()
	self.writeCache()
	self.mutex.Unlock()
	core.LogInfo("pipeman", "%d pipestances inventoried.", pscount)
}

func (self *PipestanceManager) cleanScratchPaths() {
	for _, scratchPath := range self.scratchPaths {
		scratchPsInfos, _ := ioutil.ReadDir(scratchPath)
		for _, scratchPsInfo := range scratchPsInfos {
			name := scratchPsInfo.Name()
			modTime := scratchPsInfo.ModTime()

			if time.Since(modTime) < time.Hour*scratchTimeout {
				continue
			}

			container, pipeline, psid := parsePipestanceKey(name)
			pkey := makePipestanceKey(container, pipeline, psid)

			state, ok := self.GetPipestanceState(container, pipeline, psid)
			if !ok || state == "failed" {
				if err := self.WipePipestance(container, pipeline, psid); err != nil {
					core.LogError(err, "pipeman", "Failed to wipe pipestance %s", pkey)
				}
			}
		}
	}
}

// Start an infinite process loop.
func (self *PipestanceManager) GoRunLoop() {
	self.goProcessLoop()
	self.goCleanLoop()
}

func (self *PipestanceManager) goProcessLoop() {
	go func() {
		// Sleep for 5 seconds to let the webserver fail on port rebind.
		time.Sleep(time.Second * time.Duration(5))
		for {
			if self.runLoop {
				self.processRunningPipestances()
			}

			// Wait for a bit.
			time.Sleep(time.Second * time.Duration(self.stepms))
		}
	}()
}

func (self *PipestanceManager) goCleanLoop() {
	go func() {
		for {
			self.cleanScratchPaths()

			time.Sleep(time.Hour * time.Duration(24))
		}
	}()
}

func (self *PipestanceManager) makePipestancePath(container string, pipeline string, psid string) string {
	return path.Join(self.aggregatePath, container, pipeline, psid, "HEAD")
}

func (self *PipestanceManager) copyPipestance(pkey string) {
	container, pname, psid := parsePipestanceKey(pkey)

	// Calculate permanent storage version path
	headPath := self.makePipestancePath(container, pname, psid)
	aggregatePsPath, _ := os.Readlink(headPath)
	psPath, err := os.Readlink(aggregatePsPath)
	if err == nil {
		// If pipestance path has scratch prefix, we know the permanent storage version path is on the aggregate
		for _, scratchPath := range self.scratchPaths {
			if strings.HasPrefix(psPath, scratchPath) {
				psPath = aggregatePsPath
				break
			}
		}
	} else {
		// Aggregate pipestance path is not a symlink so the pipestance has already been copied
		return
	}

	// If pipestance path symlink is broken and .tmp file exists in same directory,
	// this pipestance was about to be renamed prior to Marsoc shutdown
	hardPsPath, err := filepath.EvalSymlinks(psPath)
	newPsPath := psPath + ".tmp"
	if err != nil {
		if _, err := os.Stat(newPsPath); err == nil {
			os.Rename(newPsPath, psPath)
		}
	}

	if fileinfo, _ := os.Lstat(psPath); fileinfo.Mode()&os.ModeSymlink == os.ModeSymlink {
		// Check to make sure this isn't already being copied
		if _, ok := self.copying[pkey]; ok {
			return
		}
		self.copying[pkey] = true
		go func() {
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
					err = copyFile(oldPath, newPath)
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
				self.mailer.Sendmail(
					[]string{},
					fmt.Sprintf("%s of %s copy failed!", pname, psid),
					fmt.Sprintf("Hey Preppie,\n\n%s of %s/%s at %s failed to copy:\n\n%s", pname, container, psid, psPath, err.Error()),
				)
			}

			self.mutex.Lock()
			delete(self.copying, pkey)
			self.mutex.Unlock()
			if err == nil && pname == "BCL_PROCESSOR_PD" {
				self.mutex.Lock()
				self.analysisQueue = append(self.analysisQueue, &AnalysisNotification{Fcid: psid})
				self.mutex.Unlock()
			}
		}()
	}
}

func (self *PipestanceManager) processRunningPipestances() {
	running := map[string]*core.Pipestance{}

	// Copy the current run list
	self.mutex.Lock()
	for pkey, pipestance := range self.running {
		running[pkey] = pipestance
	}
	self.mutex.Unlock()

	// Concurrently step all pipestances in the run list copy.
	var wg sync.WaitGroup
	wg.Add(len(running))

	for pkey, pipestance := range running {
		go func(pkey string, pipestance *core.Pipestance, wg *sync.WaitGroup) {
			pipestance.RefreshState()

			state := pipestance.GetState()
			if state == "complete" {
				// If pipestance is done, remove from run list, mark it in the
				// cache as completed, and flush the cache.
				core.LogInfo("pipeman", "Complete and removing from run list: %s.", pkey)

				// VDR Kill
				killReport := pipestance.VDRKill()
				core.LogInfo("pipeman", "VDR killed %d files, %s from %s.", killReport.Count, humanize.Bytes(killReport.Size), pkey)

				// Unlock.
				pipestance.Unlock()

				// Post processing.
				pipestance.PostProcess()

				self.mutex.Lock()
				delete(self.running, pkey)
				self.completed[pkey] = true
				self.copyPipestance(pkey)
				self.mutex.Unlock()

				// Email notification.
				container, pname, psid := parsePipestanceKey(pkey)
				if pname == "BCL_PROCESSOR_PD" {
					// For BCL_PROCESSOR_PD, just email the admins.
					self.mailer.Sendmail(
						[]string{},
						fmt.Sprintf("%s of %s has succeeded!", pname, psid),
						fmt.Sprintf("Hey Preppie,\n\n%s of %s is done.\n\nCheck out my rad moves at http://%s/pipestance/%s/%s/%s.\n\nBtw I also saved you %s with VDR. Show me love!", pname, psid, self.mailer.InstanceName, psid, pname, psid, humanize.Bytes(killReport.Size)),
					)
				} else {
					// For ANALYZER_PD, queue up notification for batch email of users.
					self.mutex.Lock()
					self.mailQueue = append(self.mailQueue, &PipestanceNotification{
						Name:      psid,
						State:     "complete",
						Container: container,
						Pname:     pname,
						Psid:      psid,
						Vdrsize:   killReport.Size,
					})
					self.mutex.Unlock()
				}
			} else if state == "failed" {
				// If pipestance is failed, remove from run list, mark it in the
				// cache as failed, and flush the cache.
				core.LogInfo("pipeman", "Failed and removing from run list: %s.", pkey)

				// Unlock.
				pipestance.Unlock()

				self.mutex.Lock()
				delete(self.running, pkey)
				self.failed[pkey] = true
				self.mutex.Unlock()

				// Email notification.
				container, pname, psid := parsePipestanceKey(pkey)
				invocation := pipestance.GetInvocation()
				stage, summary, errlog, kind, errpaths := pipestance.GetFatalError()

				// Write pipestance to fail coop.
				self.writePipestanceToFailCoop(pkey, stage, summary, errlog, kind, errpaths, invocation)

				// Delete jobs for failed stage.
				deleteJobs(stage)

				if pname == "BCL_PROCESSOR_PD" {
					// For BCL_PROCESSOR_PD, just email the admins.
					self.mailer.Sendmail(
						[]string{},
						fmt.Sprintf("%s of %s has failed!", pname, psid),
						fmt.Sprintf("Hey Preppie,\n\n%s of %s failed.\n\n%s: %s\n\nDon't feel bad, but check out what you messed up at http://%s/pipestance/%s/%s/%s.", pname, psid, stage, summary, self.mailer.InstanceName, psid, pname, psid),
					)
				} else {
					// For ANALYZER_PD, queue up notification for batch email of users.
					self.mutex.Lock()
					self.mailQueue = append(self.mailQueue, &PipestanceNotification{
						Name:      psid,
						State:     "failed",
						Container: container,
						Pname:     pname,
						Psid:      psid,
						Vdrsize:   0,
						Summary:   summary,
						Stage:     stage,
					})
					self.mutex.Unlock()
				}
			} else {
				// If it is not done, check job heartbeats and step the nodes.
				pipestance.CheckHeartbeats()
				pipestance.StepNodes()
			}
			wg.Done()
		}(pkey, pipestance, &wg)
	}

	// Wait for this round of processing to complete, then write
	// out any changes to the complete/fail cache.
	wg.Wait()

	self.mutex.Lock()
	self.writeCache()
	self.mutex.Unlock()
}

func (self *PipestanceManager) writePipestanceToFailCoop(pkey string, stage string, summary string,
	errlog string, kind string, errpaths []string, invocation interface{}) {
	now := time.Now()
	currentDatePath := path.Join(self.failCoopPath, now.Format("2006-01-02"))
	if _, err := os.Stat(currentDatePath); err != nil {
		os.MkdirAll(currentDatePath, 0755)
	}

	filename := getFilenameWithSuffix(currentDatePath, fmt.Sprintf("%s-%s", self.mailer.InstanceName, pkey))
	psPath := path.Join(currentDatePath, filename)
	os.Mkdir(psPath, 0755)

	// Create failure summary JSON.
	summaryJson := map[string]interface{}{
		"pipestance": pkey,
		"stage":      stage,
		"summary":    summary,
		"errlog":     errlog,
		"kind":       kind,
		"invocation": invocation,
		"instance":   self.mailer.InstanceName,
		"timestamp":  now.Format("2006-01-02 03:04:05PM"),
	}
	summaryPath := path.Join(psPath, "summary.json")
	writeJson(summaryPath, summaryJson)

	// Copy all related metadata files.
	for _, errpath := range errpaths {
		newPath := path.Join(psPath, path.Base(errpath))
		copyFile(errpath, newPath)
	}
}

func (self *PipestanceManager) removePendingPipestance(pkey string, unfail bool) {
	self.mutex.Lock()
	delete(self.pending, pkey)
	if unfail {
		self.failed[pkey] = true
	}
	self.mutex.Unlock()
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

func (self *PipestanceManager) Invoke(container string, pipeline string, psid string, src string, tags []string) error {
	pkey := makePipestanceKey(container, pipeline, psid)

	self.mutex.Lock()
	// Check if pipestance has already been invoked
	if _, ok := self.getPipestanceState(container, pipeline, psid); ok {
		self.mutex.Unlock()
		return &core.PipestanceExistsError{psid}
	}
	scratchPath, err := self.getScratchPath()
	if err != nil {
		self.mutex.Unlock()
		return err
	}
	mroPath, mroVersion, envs, err := self.GetPipestanceEnvironment(container, pipeline, psid)
	if err != nil {
		self.mutex.Unlock()
		return err
	}
	self.pending[pkey] = true
	self.mutex.Unlock()
	core.LogInfo("pipeman", "Instantiating and pushed to pending list: %s.", pkey)

	psDir := path.Join(self.writePath, container, pipeline, psid)
	mroVersionPath := getFilenameWithSuffix(psDir, mroVersion)
	psPath := path.Join(psDir, mroVersionPath)

	scratchPsPath := path.Join(scratchPath, fmt.Sprintf("%s.%s.%s.%s", container, pipeline, psid, mroVersionPath))
	aggregatePsPath := path.Join(self.aggregatePath, container, pipeline, psid, mroVersionPath)

	// Clear all paths
	os.Remove(psPath)
	os.Remove(aggregatePsPath)
	os.RemoveAll(scratchPsPath)

	// Create symlink from permanent storage version path -> scratch path
	os.MkdirAll(scratchPsPath, 0755)
	os.MkdirAll(path.Dir(psPath), 0755)
	os.Symlink(scratchPsPath, psPath)

	if aggregatePsPath != psPath {
		// Create symlink from aggregate version path -> permanent storage version path
		os.MkdirAll(path.Dir(aggregatePsPath), 0755)
		os.Symlink(psPath, aggregatePsPath)
	}

	pipestance, err := self.rt.InvokePipeline(src, "./argshim", psid, aggregatePsPath, mroPath, mroVersion, envs, tags)
	if err != nil {
		self.removePendingPipestance(pkey, false)
		return err
	}

	// Create symlink from HEAD -> aggregate version path
	headPath := self.makePipestancePath(container, pipeline, psid)
	os.Remove(headPath)
	os.Symlink(aggregatePsPath, headPath)

	pipestance.LoadMetadata()

	core.LogInfo("pipeman", "Finished instantiating and pushing to run list: %s.", pkey)
	self.mutex.Lock()
	delete(self.pending, pkey)
	self.running[pkey] = pipestance
	self.writeCache()
	self.mutex.Unlock()

	return nil
}

func (self *PipestanceManager) ArchivePipestanceHead(container string, pipeline string, psid string) error {
	self.mutex.Lock()
	delete(self.completed, makePipestanceKey(container, pipeline, psid))
	self.writeCache()
	self.mutex.Unlock()
	headPath := self.makePipestancePath(container, pipeline, psid)
	return os.Remove(headPath)
}

func (self *PipestanceManager) KillPipestance(container string, pipeline string, psid string) error {
	pkey := makePipestanceKey(container, pipeline, psid)

	self.mutex.Lock()
	pipestance, ok := self.running[pkey]
	if !ok {
		self.mutex.Unlock()
		return &core.PipestanceNotRunningError{psid}
	}
	delete(self.running, pkey)
	self.pending[pkey] = true
	self.mutex.Unlock()

	fqname := core.MakeFQName(pipeline, psid)
	if output, err := deleteJobs(fqname); err != nil {
		core.LogError(err, "pipeman", "qdel for pipestance %s failed: %s", pkey, string(output))
		// If qdel failed because jobs didn't exist, we ignore the error since local stages
		// could be running.
		user, _ := user.Current()
		if !strings.Contains(string(output), fmt.Sprintf("The job %s* of user(s) %s does not exist", fqname, user.Username)) {
			self.mutex.Lock()
			self.running[pkey] = pipestance
			delete(self.pending, pkey)
			self.mutex.Unlock()
			return err
		}
	}
	pipestance.Kill()
	pipestance.Unlock()

	self.mutex.Lock()
	delete(self.pending, pkey)
	self.failed[pkey] = true
	self.writeCache()
	self.mutex.Unlock()
	return nil
}

func (self *PipestanceManager) WipePipestance(container string, pipeline string, psid string) error {
	pkey := makePipestanceKey(container, pipeline, psid)

	self.mutex.Lock()
	if state, _ := self.getPipestanceState(container, pipeline, psid); state != "failed" {
		self.mutex.Unlock()
		return &core.PipestanceNotFailedError{psid}
	}
	delete(self.failed, pkey)
	self.pending[pkey] = true
	self.mutex.Unlock()

	headPath := self.makePipestancePath(container, pipeline, psid)
	aggregatePsPath, _ := os.Readlink(headPath)
	psPath, _ := os.Readlink(aggregatePsPath)
	hardPsPath, _ := filepath.EvalSymlinks(psPath)

	for _, scratchPath := range self.scratchPaths {
		if strings.HasPrefix(hardPsPath, scratchPath) {
			core.LogInfo("pipeman", "Wiping pipestance: %s.", pkey)
			go func() {
				os.Remove(headPath)
				os.Remove(aggregatePsPath)
				os.Remove(psPath)
				os.RemoveAll(hardPsPath)

				core.LogInfo("pipeman", "Finished wiping pipestance: %s.", pkey)
				self.mutex.Lock()
				delete(self.pending, pkey)
				self.writeCache()
				self.mutex.Unlock()
			}()
			return nil
		}
	}

	self.removePendingPipestance(pkey, true)
	return &core.PipestanceWipeError{psid}
}

func (self *PipestanceManager) UnfailPipestance(container string, pipeline string, psid string) error {
	pkey := makePipestanceKey(container, pipeline, psid)

	self.mutex.Lock()
	state, _ := self.getPipestanceState(container, pipeline, psid)
	// Check if pipestance is being copied right now.
	if state == "copying" {
		self.mutex.Unlock()
		return &core.PipestanceCopyingError{psid}
	}
	// Check if pipestance is failed
	if state != "failed" {
		self.mutex.Unlock()
		return &core.PipestanceNotFailedError{psid}
	}
	delete(self.failed, pkey)
	self.pending[pkey] = true
	self.mutex.Unlock()
	core.LogInfo("pipeman", "Unfailing and pushed to pending list: %s.", pkey)

	readOnly := false
	pipestance, ok := self.GetPipestance(container, pipeline, psid, readOnly)
	if !ok {
		self.removePendingPipestance(pkey, true)
		return &core.PipestanceNotExistsError{psid}
	}

	nodes := pipestance.GetFailedNodes()
	for _, node := range nodes {
		deleteJobs(node.GetFQName())
	}

	if err := pipestance.Reset(); err != nil {
		self.removePendingPipestance(pkey, true)
		return err
	}

	core.LogInfo("pipeman", "Finished unfailing and pushing to run list: %s.", pkey)
	self.mutex.Lock()
	delete(self.pending, pkey)
	self.running[pkey] = pipestance
	self.writeCache()
	self.mutex.Unlock()
	return nil
}

func (self *PipestanceManager) getPipestanceState(container string, pipeline string, psid string) (string, bool) {
	pkey := makePipestanceKey(container, pipeline, psid)
	if _, ok := self.copying[pkey]; ok {
		return "copying", true
	}
	if _, ok := self.completed[pkey]; ok {
		return "complete", true
	}
	if _, ok := self.failed[pkey]; ok {
		return "failed", true
	}
	if run, ok := self.running[pkey]; ok {
		return run.GetState(), true
	}
	if _, ok := self.pending[pkey]; ok {
		return "waiting", true
	}
	return "", false
}

func (self *PipestanceManager) GetPipestanceState(container string, pipeline string, psid string) (string, bool) {
	self.mutex.Lock()
	state, ok := self.getPipestanceState(container, pipeline, psid)
	self.mutex.Unlock()
	return state, ok
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

	// Cache serialization if pipestance is complete
	if state, _ := self.GetPipestanceState(container, pipeline, psid); state == "complete" {
		pipestance.Immortalize()
		if ser, ok := self.rt.GetSerialization(psPath, name); ok {
			return ser, true
		}
	}

	return pipestance.Serialize(name), true
}

func (self *PipestanceManager) GetPipestanceMetadata(container string, pipeline string, psid string, metadataPath string) (string, error) {
	psPath := self.makePipestancePath(container, pipeline, psid)
	permanentPsPath, _ := os.Readlink(psPath)
	return self.rt.GetMetadata(permanentPsPath, metadataPath)
}

func (self *PipestanceManager) GetPipestance(container string, pipeline string, psid string, readOnly bool) (*core.Pipestance, bool) {
	pkey := makePipestanceKey(container, pipeline, psid)

	// Check if requested pipestance actually exists.
	if _, ok := self.GetPipestanceState(container, pipeline, psid); !ok {
		return nil, false
	}

	// Check the run table.
	self.mutex.Lock()
	if pipestance, ok := self.running[pkey]; ok {
		self.mutex.Unlock()
		return pipestance, true
	}
	self.mutex.Unlock()

	// Reattach to the pipestance.
	psPath := self.makePipestancePath(container, pipeline, psid)
	pipestance, err := self.ReattachToPipestance(container, pipeline, psid, psPath, readOnly)
	if err != nil {
		return nil, false
	}

	// Load its metadata and return.
	pipestance.LoadMetadata()
	return pipestance, true
}

func (self *PipestanceManager) ReattachToPipestance(container string, pipeline string, psid string, psPath string, readOnly bool) (*core.Pipestance, error) {
	mroPath, mroVersion, envs, err := self.GetPipestanceEnvironment(container, pipeline, psid)
	if err != nil {
		return nil, err
	}
	permanentPsPath, _ := os.Readlink(psPath)
	return self.rt.ReattachToPipestance(psid, permanentPsPath, "", mroPath, mroVersion, envs, false, readOnly)
}

func (self *PipestanceManager) getPipestanceMetadata(container string, pipeline string, psid string, fname string) (string, error) {
	psPath := self.makePipestancePath(container, pipeline, psid)

	data, err := ioutil.ReadFile(path.Join(psPath, fname))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (self *PipestanceManager) GetPipestanceInvokeSrc(container string, pipeline string, psid string) (string, error) {
	return self.getPipestanceMetadata(container, pipeline, psid, "_invocation")
}

func (self *PipestanceManager) GetPipestanceTimestamp(container string, pipeline string, psid string) (string, error) {
	data, err := self.getPipestanceMetadata(container, pipeline, psid, "_timestamp")
	if err != nil {
		return "", err
	}
	return core.ParseTimestamp(data), nil
}

func (self *PipestanceManager) GetPipestanceVersions(container string, pipeline string, psid string) (string, string, error) {
	data, err := self.getPipestanceMetadata(container, pipeline, psid, "_versions")
	if err != nil {
		return "", "", err
	}
	return core.ParseVersions(data)
}

func (self *PipestanceManager) GetPipestanceOuts(container string, pipeline string, psid string, forkIndex int) map[string]interface{} {
	psPath := self.makePipestancePath(container, pipeline, psid)
	permanentPsPath, _ := os.Readlink(psPath)
	metadataPath := path.Join(permanentPsPath, pipeline, fmt.Sprintf("fork%d", forkIndex), "_outs")
	if data, err := self.GetPipestanceMetadata(container, pipeline, psid, metadataPath); err == nil {
		var v map[string]interface{}
		if err := json.Unmarshal([]byte(data), &v); err == nil {
			return v
		}
	}
	return map[string]interface{}{}
}

func (self *PipestanceManager) GetPipestanceEnvironment(container string, pipeline string, psid string) (string, string, map[string]string, error) {
	return self.packages.GetPipestanceEnvironment(container, pipeline, psid)
}
