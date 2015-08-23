//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// Houston submission manager.
//

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"martian/core"
	"martian/manager"
	"os"
	"path"
	"sort"
	"strings"
)

const SMALLFILE_THRESHOLD = 10 * 1000 * 1000 // 10MB

type Submission struct {
	Source  string      `json:"source"`
	Date    string      `json:"date"`
	Name    string      `json:"name"`
	Kind    string      `json:"kind"`
	State   string      `json:"state"`
	Fname   string      `json:"fname"`
	Path    string      `json:"path"`
	Summary interface{} `json:"summary"`
}

// Sorting support for Submission
type ByDate []*Submission

func (a ByDate) Len() int      { return len(a) }
func (a ByDate) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByDate) Less(i, j int) bool {
	if a[i].Date == a[j].Date {
		return a[i].Source > a[j].Source
	}
	return a[i].Date > a[j].Date
}

type SubmissionManager struct {
	hostname               string
	instanceName           string
	filesPath              string
	cachePath              string
	pipestanceSummaryPaths []string
	cache                  map[string]*Submission
	rt                     *core.Runtime
	mailer                 *manager.Mailer
}

func NewSubmissionManager(hostname string, instanceName string, filesPath string,
	cachePath string, pipestanceSummaryPaths []string, rt *core.Runtime,
	mailer *manager.Mailer) *SubmissionManager {
	self := &SubmissionManager{}
	self.instanceName = instanceName
	self.filesPath = filesPath
	self.cachePath = path.Join(cachePath, "submissions")
	self.cache = map[string]*Submission{}
	self.pipestanceSummaryPaths = pipestanceSummaryPaths
	self.rt = rt
	self.mailer = mailer
	self.loadCache()
	self.hostname = hostname
	return self
}

func (self *SubmissionManager) loadCache() error {
	bytes, err := ioutil.ReadFile(self.cachePath)
	if err != nil {
		core.LogInfo("submngr", "Could not read cache file %s.", self.cachePath)
		return err
	}

	if err := json.Unmarshal(bytes, &self.cache); err != nil {
		core.LogError(err, "submngr", "Could not parse JSON in cache file %s.", self.cachePath)
		return err
	}

	core.LogInfo("submngr", "%d pipestances loaded from cache.", len(self.cache))
	return nil
}

func writeJson(fpath string, object interface{}) {
	bytes, _ := json.MarshalIndent(object, "", "    ")
	if err := ioutil.WriteFile(fpath, bytes, 0644); err != nil {
		core.LogError(err, "submngr", "Could not write JSON file %s.", fpath)
	}
}

func (self *SubmissionManager) makeSubmissionKey(container string, pname string, psid string) string {
	return container + "/" + pname + "/" + psid
}

func (self *SubmissionManager) InventorySubmissions() {
	// List of new submissions
	newSubmissions := []*Submission{}

	// Traverse the source/date/name hierarchy
	sourceInfos, _ := ioutil.ReadDir(self.filesPath)
	for _, sourceInfo := range sourceInfos {
		source := sourceInfo.Name()
		dateInfos, _ := ioutil.ReadDir(path.Join(self.filesPath, source))
		for _, dateInfo := range dateInfos {
			date := dateInfo.Name()
			nameInfos, _ := ioutil.ReadDir(path.Join(self.filesPath, source, date))
			for _, nameInfo := range nameInfos {

				name := nameInfo.Name()
				key := self.makeSubmissionKey(source, date, name)

				// If this submission is already cached, skip it
				if _, ok := self.cache[key]; ok {
					continue
				}

				p := path.Join(self.filesPath, source, date, name)
				fname := "none"
				kind := "unknown"
				state := "unknown"
				var summary interface{}

				// Check if this is a pipestance by presence of HEAD symlink
				if fi, err := os.Lstat(path.Join(p, "HEAD")); err == nil && (fi.Mode()&os.ModeSymlink == os.ModeSymlink) {
					// Cache serializations and state
					kind = "pipestance"
					state = self.GetPipestanceState(source, date, name)
					core.LogInfo("submngr", "Discovered %s %s at %s", state, kind, key)

					core.LogInfo("submngr", "    Immortalizing")
					_, _ = self.GetPipestanceSerialization(source, date, name, "finalstate")
					core.LogInfo("submngr", "    Finished immortalizing")

					if state == "complete" {
						// Check at specified paths for summary file
						for _, psp := range self.pipestanceSummaryPaths {
							summaryPath := path.Join(p, "HEAD", psp)
							if _, err := os.Stat(summaryPath); err == nil {
								bytes, err := ioutil.ReadFile(summaryPath)
								if err != nil {
									core.LogInfo("submngr", "    Could not read summary file %s.", summaryPath)
									continue
								}
								if err := json.Unmarshal(bytes, &summary); err != nil {
									core.LogError(err, "submngr", "    Could not parse JSON in file %s.", summaryPath)
									continue
								}
								core.LogInfo("submngr", "    Summary exists at %s", psp)
								break
							}
						}
					}
				} else {
					kind = "file"

					fileInfos, _ := ioutil.ReadDir(p)
					if len(fileInfos) > 0 {
						fname = fileInfos[0].Name()
						if fileInfos[0].Size() < SMALLFILE_THRESHOLD {
							kind = "smallfile"
						}
					}
					core.LogInfo("submngr", "Discovered %s %s at %s", kind, fname, key)
				}

				sub := Submission{
					Source:  source,
					Date:    date,
					Name:    name,
					Kind:    kind,
					State:   state,
					Fname:   fname,
					Path:    p,
					Summary: summary,
				}
				self.cache[key] = &sub
				newSubmissions = append(newSubmissions, &sub)
			}
		}
	}

	// Send out email enumerating newly discovered submissions
	if len(newSubmissions) > 0 {
		lines := []string{}
		for i, s := range newSubmissions {
			user := ""
			domain := ""
			parts := strings.Split(s.Source, "@")
			if len(parts) > 0 {
				domain = parts[0]
			}
			if len(parts) > 1 {
				user = parts[1]
			}
			line := ""
			if s.Kind == "pipestance" {
				line = fmt.Sprintf("%d. %s %s %s from %s@%s\n    http://%s/pipestance/%s/%s/%s\n",
					i+1, strings.ToUpper(s.State), s.Kind, s.Name, user, domain, self.hostname, s.Source, s.Date, s.Name)
			} else if s.Kind == "smallfile" {
				line = fmt.Sprintf("%d. %s from %s@%s\n    http://%s/file/%s/%s/%s/%s\n",
					i+1, s.Fname, user, domain, self.hostname, s.Source, s.Date, s.Name, s.Fname)
			} else {
				line = fmt.Sprintf("%d. %s from %s@%s\n    %s\n",
					i+1, s.Fname, user, domain, s.Path)
			}
			lines = append(lines, line)
		}
		subj := fmt.Sprintf("[%s] %d new customer submissions", self.instanceName, len(newSubmissions))
		body := strings.Join(lines, "\n")

		users := []string{}
		self.mailer.Sendmail(users, subj, body)
	}

	// Write submissions to persistent cache
	writeJson(self.cachePath, self.cache)
}

func (self *SubmissionManager) EnumerateSubmissions() []*Submission {
	subs := []*Submission{}
	for _, v := range self.cache {
		subs = append(subs, v)
	}
	sort.Sort(ByDate(subs))
	return subs
}

//
// Submission management methods
//

func (self *SubmissionManager) GetBareFile(container string, pname string, psid string, fname string) (string, error) {
	data, err := ioutil.ReadFile(path.Join(self.filesPath, container, pname, psid, fname))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

//
// Pipestance management methods
// Should be refactored against other PipestanceManagers
//

func (self *SubmissionManager) makePipestancePath(container string, pname string, psid string) string {
	return path.Join(self.filesPath, container, pname, psid, "HEAD")
}

func (self *SubmissionManager) getPipestanceMetadata(container string, pname string, psid string, fname string) (string, error) {
	filesPath := self.makePipestancePath(container, pname, psid)

	data, err := ioutil.ReadFile(path.Join(filesPath, fname))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (self *SubmissionManager) GetPipestance(container string, pname string, psid string, readOnly bool) (*core.Pipestance, bool) {
	filesPath := self.makePipestancePath(container, pname, psid)
	pipestance, _ := self.rt.ReattachToPipestanceWithMroSrc(psid, filesPath, "", "", "", map[string]string{}, false, true)
	pipestance.LoadMetadata()
	return pipestance, true
}

func (self *SubmissionManager) GetPipestanceState(container string, pname string, psid string) string {
	pipestance, ok := self.GetPipestance(container, pname, psid, true)
	if !ok {
		return "waiting"
	}

	return pipestance.GetState()
}

func (self *SubmissionManager) GetPipestanceTopFile(container string, pname string, psid string, fname string) (string, error) {
	return self.getPipestanceMetadata(container, pname, psid, fname)
}

func (self *SubmissionManager) GetPipestanceMetadata(container string, pname string, psid string, metadataPath string) (string, error) {
	filesPath := self.makePipestancePath(container, pname, psid)
	permanentPsPath, _ := os.Readlink(filesPath)
	return self.rt.GetMetadata(permanentPsPath, metadataPath)
}

func (self *SubmissionManager) GetPipestanceCommandline(container string, pname string, psid string) (string, error) {
	return self.getPipestanceMetadata(container, pname, psid, "_cmdline")
}

func (self *SubmissionManager) GetPipestanceInvokeSrc(container string, pname string, psid string) (string, error) {
	return self.getPipestanceMetadata(container, pname, psid, "_invocation")
}

func (self *SubmissionManager) GetPipestanceTimestamp(container string, pname string, psid string) (string, error) {
	data, err := self.getPipestanceMetadata(container, pname, psid, "_timestamp")
	if err != nil {
		return "", err
	}
	return core.ParseTimestamp(data), nil
}

func (self *SubmissionManager) GetPipestanceVersions(container string, pname string, psid string) (string, string, error) {
	data, err := self.getPipestanceMetadata(container, pname, psid, "_versions")
	if err != nil {
		return "", "", err
	}
	return core.ParseVersions(data)
}

func (self *SubmissionManager) GetPipestanceJobMode(container string, pname string, psid string) (string, string, string) {
	data, err := self.getPipestanceMetadata(container, pname, psid, "_cmdline")
	if err != nil {
		return "", "", ""
	}
	return core.ParseJobMode(data)
}

func (self *SubmissionManager) GetPipestanceSerialization(container string, pname string, psid string, name string) (interface{}, bool) {
	filesPath := self.makePipestancePath(container, pname, psid)
	if ser, ok := self.rt.GetSerialization(filesPath, name); ok {
		return ser, true
	}

	pipestance, ok := self.GetPipestance(container, pname, psid, true)
	if !ok {
		return nil, false
	}

	// Cache serialization of pipestance
	pipestance.Immortalize()
	if ser, ok := self.rt.GetSerialization(filesPath, name); ok {
		return ser, true
	}

	return pipestance.Serialize(name), true
}
