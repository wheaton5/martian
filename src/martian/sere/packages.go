//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
package main

import (
	"fmt"
	"io/ioutil"
	"martian/core"
	"martian/manager"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
)

const packageBase = "package"

type PackageId struct {
	name   string
	target string
}

type PackageManager struct {
	packagePath string
	debug       bool
	packages    map[PackageId][]*manager.Package
	building    map[PackageId]bool
	mutex       *sync.RWMutex
	db          *DatabaseManager
}

func getPackagePath(name string, target string, sakePath string) string {
	return path.Join(sakePath, "packages", fmt.Sprintf("%s-%s", name, target))
}

func NewPackageManager(packagePath string, debug bool, db *DatabaseManager) *PackageManager {
	self := &PackageManager{}
	self.packagePath = packagePath
	self.debug = debug
	self.db = db
	self.building = map[PackageId]bool{}
	self.mutex = &sync.RWMutex{}

	self.verifyPackages()

	return self
}

func (self *PackageManager) GetPipestanceEnvironment(container string, pipeline string, psid string) (string, string, map[string]string, error) {
	programName, cycleId, roundId := parseContainerKey(container)
	round, err := self.db.GetRound(programName, cycleId, roundId)
	if err != nil {
		return "", "", nil, err
	}
	p, err := self.GetPackage(round.PackageName, round.PackageTarget, round.PackageVersion)
	if err != nil {
		return "", "", nil, err
	}
	return p.MroPath, p.MroVersion, p.Envs, nil
}

func (self *PackageManager) ManagePackages() []*manager.Package {
	packages := []*manager.Package{}

	self.mutex.RLock()
	for _, p := range self.packages {
		packages = append(packages, p...)
	}
	for pid, _ := range self.building {
		p := &manager.Package{
			Name:   pid.name,
			Target: pid.target,
		}
		packages = append(packages, p)
	}
	self.mutex.RUnlock()

	return packages
}

func (self *PackageManager) BuildPackage(name string, target string) error {
	pid := PackageId{name, target}
	cmd := exec.Command("sake", "build", "pkg", name, target)

	self.mutex.Lock()
	// Check to see if package is being built already
	if _, ok := self.building[pid]; ok {
		return &core.MartianError{fmt.Sprintf("Package %s-%s already being built.", name, target)}
	}
	self.building[pid] = true
	self.mutex.Unlock()

	sakeDir := path.Join(self.packagePath, name, target)
	sakePath := path.Join(sakeDir, core.GetFilenameWithSuffix(sakeDir, packageBase))
	os.RemoveAll(sakePath)
	os.MkdirAll(sakePath, 0755)

	core.LogInfo("package", "Package %s-%s building in %s.", name, target, sakePath)
	cmd.Dir = sakePath

	go func() {
		if output, err := cmd.CombinedOutput(); err != nil {
			core.LogError(err, "package", "Package %s-%s failed to build in %s: %s",
				name, target, sakePath, string(output))
			os.RemoveAll(sakePath)
		} else {
			core.LogInfo("package", "Package %s-%s successfully built in %s.", name, target, sakePath)
			if err := self.addPackage(name, target, sakePath); err != nil {
				core.LogError(err, "package", "Package %s-%s in %s cannot be added.",
					name, target, sakePath)
			}
		}

		self.mutex.Lock()
		delete(self.building, pid)
		self.mutex.Unlock()
	}()

	return nil
}

func (self *PackageManager) GetPackage(name string, target string, mroVersion string) (*manager.Package, error) {
	pid := PackageId{name, target}

	self.mutex.RLock()
	if packages, ok := self.packages[pid]; ok {
		for _, p := range packages {
			if p.MroVersion == mroVersion {
				return p, nil
			}
		}
	}
	self.mutex.RUnlock()

	return nil, &core.MartianError{fmt.Sprintf("Package %s-%s with mro version %s not found.", name, target, mroVersion)}
}

func (self *PackageManager) addPackage(name string, target string, path string) error {
	pid := PackageId{name, target}

	packagePath := getPackagePath(name, target, path)
	if _, _, _, _, _, err := manager.VerifyPackage(packagePath); err != nil {
		return err
	}
	p := manager.NewPackage(packagePath, self.debug)

	self.mutex.Lock()
	if _, ok := self.packages[pid]; !ok {
		self.packages[pid] = []*manager.Package{}
	}
	self.packages[pid] = append(self.packages[pid], p)
	self.mutex.Unlock()

	return nil
}

func (self *PackageManager) verifyPackages() {
	self.packages = map[PackageId][]*manager.Package{}

	paths := strings.Split(os.Getenv("PATH"), ":")
	if _, ok := core.SearchPaths("sake", paths); !ok {
		core.PrintInfo("package", "Cannot find sake along PATH.")
		os.Exit(1)
	}

	nameInfos, _ := ioutil.ReadDir(self.packagePath)
	for _, nameInfo := range nameInfos {
		name := nameInfo.Name()
		targetInfos, _ := ioutil.ReadDir(path.Join(self.packagePath, name))
		for _, targetInfo := range targetInfos {
			target := targetInfo.Name()

			i := 0
			for {
				sakePath := path.Join(self.packagePath, name, target, fmt.Sprintf("%s-%d", packageBase, i))
				if _, err := os.Stat(sakePath); err != nil {
					break
				}
				if err := self.addPackage(name, target, sakePath); err != nil {
					core.LogError(err, "package", "Package %s-%s in %s cannot be added.",
						name, target, sakePath)
				}

				i += 1
			}
		}
	}
}
