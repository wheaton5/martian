//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Marsoc package manager.
//
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"martian/core"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

type PackageManager struct {
	defaultPackage string
	packages       map[string]*Package
	mutex          *sync.Mutex
	lena           *Lena
}

type Package struct {
	name        string
	argshimPath string
	mroPath     string
	mroVersion  string
	envs        map[string]string
	argshim     *ArgShim
}

type PackageJson struct {
	Name        string                     `json:"name"`
	ArgshimPath string                     `json:"argshim_path"`
	MroPath     string                     `json:"mro_path"`
	Envs        map[string]*PackageJsonEnv `json:"envs"`
}

type PackageJsonEnv struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

func NewPackageManager(packagesPath string, defaultPackage string, debug bool, lena *Lena) *PackageManager {
	self := &PackageManager{}
	self.mutex = &sync.Mutex{}
	self.defaultPackage = defaultPackage
	self.lena = lena
	self.packages = verifyPackages(packagesPath, defaultPackage, debug)

	core.LogInfo("package", "%d packages found.", len(self.packages))
	self.refreshVersions()
	return self
}

func NewPackage(packagePath string, debug bool) *Package {
	self := &Package{}
	self.name, self.argshimPath, self.mroPath, self.envs = verifyPackage(packagePath)
	self.mroVersion = core.GetMroVersion(self.mroPath)
	self.argshim = NewArgShim(self.argshimPath, self.envs, debug)
	return self
}

func (self *PackageManager) getPackages() []*Package {
	packages := []*Package{}
	for _, p := range self.packages {
		packages = append(packages, p)
	}
	return packages
}

// Argshim functions
func (self *PackageManager) getPipelineForSample(sample *Sample) string {
	if p, ok := self.packages[sample.Product]; ok {
		return p.argshim.getPipelineForSample(sample)
	}
	return ""
}

func (self *PackageManager) buildCallSourceForRun(run *Run) string {
	p := self.packages[self.defaultPackage]
	return p.argshim.buildCallSourceForRun(run)
}

func (self *PackageManager) buildCallSourceForSample(sbag interface{}, fastqPaths map[string]string, sample *Sample) string {
	if p, ok := self.packages[sample.Product]; ok {
		return p.argshim.buildCallSourceForSample(sbag, fastqPaths)
	}
	return ""
}

// Pipestance manager functions
func (self *PackageManager) getPipestanceEnvironment(psid string) (string, string, map[string]string, error) {
	if sample := self.lena.getSampleWithId(psid); sample != nil {
		if p, ok := self.packages[sample.Product]; ok {
			self.mutex.Lock()
			defer self.mutex.Unlock()

			return p.mroPath, p.mroVersion, p.envs, nil
		}
	}
	return "", "", nil, &core.MartianError{fmt.Sprintf("PackageManagerError: Failed to get environment for pipestance '%s'.", psid)}
}

func (self *PackageManager) getDefaultPipestanceEnvironment() (string, string, map[string]string, error) {
	p := self.packages[self.defaultPackage]

	self.mutex.Lock()
	defer self.mutex.Unlock()

	return p.mroPath, p.mroVersion, p.envs, nil
}

// Version functions
func (self *PackageManager) refreshVersions() {
	go func() {
		for {
			self.mutex.Lock()
			for _, p := range self.packages {
				p.mroVersion = core.GetMroVersion(p.mroPath)
			}
			self.mutex.Unlock()

			time.Sleep(time.Minute * time.Duration(5))
		}
	}()
}

func (self *PackageManager) GetMroVersion() string {
	// Gets version from default package
	self.mutex.Lock()
	p := self.packages[self.defaultPackage]
	mroVersion := p.mroVersion
	self.mutex.Unlock()
	return mroVersion
}

// Package config verification
func verifyPackages(packagesPath string, defaultPackage string, debug bool) map[string]*Package {
	packages := map[string]*Package{}

	infos, err := ioutil.ReadDir(packagesPath)
	if err != nil {
		core.PrintInfo("package", "Packages path %s does not exist.", packagesPath)
		os.Exit(1)
	}
	for _, info := range infos {
		packagePath := path.Join(packagesPath, info.Name())

		p := NewPackage(packagePath, debug)
		if _, ok := packages[p.name]; ok {
			core.PrintInfo("package", "Duplicate package %s found.", p.name)
			os.Exit(1)
		}
		packages[p.name] = p
	}
	if _, ok := packages[defaultPackage]; !ok {
		core.PrintInfo("package", "Default package %s not found.", defaultPackage)
		os.Exit(1)
	}
	return packages
}

func verifyPackage(packagePath string) (string, string, string, map[string]string) {
	packageFile := path.Join(packagePath, "marsoc.json")
	if _, err := os.Stat(packageFile); os.IsNotExist(err) {
		core.PrintInfo("package", "Package config file %s does not exist.", packageFile)
		os.Exit(1)
	}
	bytes, _ := ioutil.ReadFile(packageFile)

	var packageJson *PackageJson
	if err := json.Unmarshal(bytes, &packageJson); err != nil {
		core.PrintInfo("package", "Package config file %s does not contain valid JSON.", packageFile)
		os.Exit(1)
	}

	argshimPath := path.Join(packagePath, packageJson.ArgshimPath)
	if _, err := os.Stat(argshimPath); err != nil {
		core.PrintInfo("package", "Package argshim file %s does not exist.", argshimPath)
		os.Exit(1)
	}

	mroPath := path.Join(packagePath, packageJson.MroPath)
	if _, err := os.Stat(mroPath); err != nil {
		core.PrintInfo("package", "Package mro path %s does not exist.", mroPath)
		os.Exit(1)
	}

	name := packageJson.Name

	envs := map[string]string{}
	for key, envJson := range packageJson.Envs {
		value := envJson.Value
		switch envJson.Type {
		case "path":
			value = parsePathEnv(value, packagePath)
		case "path_prepend":
			value = parsePathEnv(value, packagePath)
			if prefix := os.Getenv(key); len(prefix) > 0 {
				// Prepend value to current environment variable
				value = value + ":" + prefix
			}
		case "string":
			break
		default:
			core.PrintInfo("package", "Unsupported env variable type %s.", envJson.Type)
			os.Exit(1)
		}
		if _, ok := envs[key]; ok {
			core.PrintInfo("package", "Duplicate env variable %s found.", key)
			os.Exit(1)
		}
		envs[key] = value
	}

	return name, argshimPath, mroPath, envs
}

// Helper functions
func parsePathEnv(env string, cwd string) string {
	l := []string{}

	values := strings.Split(env, ":")
	for _, value := range values {
		if !strings.HasPrefix(value, "/") {
			// Assume path is relative if it does not begin with a slash
			value = path.Join(cwd, value)
		}
		l = append(l, value)
	}
	return strings.Join(l, ":")
}
