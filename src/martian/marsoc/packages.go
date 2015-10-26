//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Marsoc package manager.
//
package main

import (
	"fmt"
	"io/ioutil"
	"martian/core"
	"martian/manager"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

type PackageFunc func(p *manager.Package) bool

type PackageManager struct {
	defaultPackage string
	isDirty        bool
	packages       map[string]*manager.Package
	mutex          *sync.Mutex
	lena           *Lena
	mailer         *manager.Mailer
}

func NewPackageManager(packagesPath string, defaultPackage string, debug bool, lena *Lena,
	mailer *manager.Mailer) *PackageManager {
	self := &PackageManager{}
	self.mutex = &sync.Mutex{}
	self.defaultPackage = defaultPackage
	self.lena = lena
	self.mailer = mailer
	self.packages = verifyPackages(packagesPath, defaultPackage, debug)

	core.LogInfo("package", "%d packages found.", len(self.packages))
	self.refreshVersions()
	return self
}

func (self *PackageManager) getPackages(packageFunc PackageFunc) []*manager.Package {
	packages := []*manager.Package{}
	for _, p := range self.packages {
		if packageFunc(p) {
			packages = append(packages, p)
		}
	}
	return packages
}

func (self *PackageManager) GetPackages() []*manager.Package {
	return self.getPackages(func(p *manager.Package) bool {
		return true
	})
}

func (self *PackageManager) CheckDirtyPackages() bool {
	packages := self.getPackages(func(p *manager.Package) bool {
		return p.IsDirty()
	})

	lastDirty := self.isDirty
	if len(packages) > 0 {
		self.isDirty = true
	} else {
		self.isDirty = false
	}

	// Send emails only when packages are first detected with dirty version.
	if !lastDirty {
		for _, p := range packages {
			self.mailer.Sendmail(
				[]string{},
				fmt.Sprintf("Package %s dirty!", p.Name),
				fmt.Sprintf("Hey Preppie,\n\nPackage %s has dirty version %s. Running pipestances is disabled until this is resolved!", p.Name, p.MroVersion),
			)
		}
	}

	return self.isDirty
}

// Argshim functions
func (self *PackageManager) GetPipelineForSample(sample *Sample) string {
	if p, ok := self.packages[sample.Product]; ok {
		sampleId := strconv.Itoa(sample.Id)
		sampleBag := self.lena.GetSampleBagWithId(sampleId)
		return p.Argshim.GetPipelineForTest("lena", sampleId, sampleBag)
	}
	return ""
}

func (self *PackageManager) BuildCallSourceForRun(rt *core.Runtime, run *Run) string {
	p := self.packages[self.defaultPackage]
	return p.Argshim.BuildCallSourceForRun(rt, run, p.MroPath)
}

func (self *PackageManager) BuildCallSourceForSample(rt *core.Runtime, sbag interface{}, fastqPaths map[string]string,
	sample *Sample) string {
	if p, ok := self.packages[sample.Product]; ok {
		return p.Argshim.BuildCallSourceForTest(rt, "lena", strconv.Itoa(sample.Id), sbag, fastqPaths, p.MroPath)
	}
	return ""
}

func (self *PackageManager) GetWebshimResponseForSample(sample *Sample, function string, sbag interface{}, files map[string]interface{}) interface{} {
	if p, ok := self.packages[sample.Product]; ok {
		sampleId := strconv.Itoa(sample.Id)
		return p.Argshim.GetWebshimResponseForTest("lena", function, sampleId, sbag, files)
	}
	return ""
}

// Pipestance manager functions
func (self *PackageManager) GetPipestanceEnvironment(container string, pipeline string, psid string) (string, string, string, map[string]string, error) {
	if pipeline == "BCL_PROCESSOR_PD" {
		return self.getDefaultPipestanceEnvironment()
	}
	return self.getPipestanceEnvironment(psid)
}

func (self *PackageManager) getPipestanceEnvironment(psid string) (string, string, string, map[string]string, error) {
	if sample := self.lena.GetSampleWithId(psid); sample != nil {
		if p, ok := self.packages[sample.Product]; ok {
			self.mutex.Lock()
			defer self.mutex.Unlock()

			return p.MroPath, p.MroVersion, p.ArgshimPath, p.Envs, nil
		}
	}
	return "", "", "", nil, &core.MartianError{fmt.Sprintf("PackageManagerError: Failed to get environment for pipestance '%s'.", psid)}
}

func (self *PackageManager) getDefaultPipestanceEnvironment() (string, string, string, map[string]string, error) {
	p := self.packages[self.defaultPackage]

	self.mutex.Lock()
	defer self.mutex.Unlock()

	return p.MroPath, p.MroVersion, p.ArgshimPath, p.Envs, nil
}

// Version functions
func (self *PackageManager) refreshVersions() {
	go func() {
		for {
			for _, p := range self.packages {
				mroVersion := core.GetMroVersion(p.MroPath)

				self.mutex.Lock()
				p.MroVersion = mroVersion
				self.mutex.Unlock()
			}

			time.Sleep(time.Minute * time.Duration(5))
		}
	}()
}

func (self *PackageManager) GetMroVersion() string {
	// Gets version from default package
	self.mutex.Lock()
	p := self.packages[self.defaultPackage]
	mroVersion := p.MroVersion
	self.mutex.Unlock()
	return mroVersion
}

func verifyPackages(packagesPath string, defaultPackage string, debug bool) map[string]*manager.Package {
	packages := map[string]*manager.Package{}

	infos, err := ioutil.ReadDir(packagesPath)
	if err != nil {
		core.PrintInfo("package", "Packages path %s does not exist.", packagesPath)
		os.Exit(1)
	}
	for _, info := range infos {
		packagePath := path.Join(packagesPath, info.Name())
		if _, _, _, _, _, _, err := manager.VerifyPackage(packagePath); err != nil {
			os.Exit(1)
		}

		p := manager.NewPackage(packagePath, debug)
		if _, ok := packages[p.Name]; ok {
			core.PrintInfo("package", "Duplicate package %s found.", p.Name)
			os.Exit(1)
		}
		packages[p.Name] = p
	}
	if _, ok := packages[defaultPackage]; !ok {
		core.PrintInfo("package", "Default package %s not found.", defaultPackage)
		os.Exit(1)
	}
	return packages
}
