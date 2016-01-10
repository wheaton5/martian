//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
package main

import (
	"io/ioutil"
	"martian/core"
	"martian/manager"
	"os"
	"path"
	"strconv"
)

type WebShimQuery struct {
	function string
	sampleId string
	sbag     interface{}
	files    map[string]interface{}
	out      chan WebShimResult
}

type WebShimResult struct {
	v interface{}
}

type PackageManager struct {
	packages map[string][]*manager.Package
	in       chan WebShimQuery
}

func NewPackageManager(packagesPath string, maxProcs int, debug bool) *PackageManager {
	self := &PackageManager{}
	self.packages = map[string][]*manager.Package{}
	self.in = make(chan WebShimQuery)

	self.verifyPackages(packagesPath, maxProcs, debug)

	core.LogInfo("package", "%d packages found.", len(self.packages))
	return self
}

func (self *PackageManager) GetWebshimResponseForSample(sampleId int, product string, function string, sbag interface{}, files map[string]interface{}) interface{} {
	if _, ok := self.packages[product]; ok {
		out := make(chan WebShimResult)
		query := WebShimQuery{function, strconv.Itoa(sampleId), sbag, files, out}
		self.in <- query
		result := <-out
		return result.v
	}
	return ""
}

func (self *PackageManager) verifyPackages(packagesPath string, maxProcs int, debug bool) {
	infos, err := ioutil.ReadDir(packagesPath)
	if err != nil {
		core.PrintInfo("package", "Packages path %s does not exist.", packagesPath)
		os.Exit(1)
	}
	for _, info := range infos {
		packagePath := path.Join(packagesPath, info.Name())
		name, _, _, _, _, _, err := manager.VerifyPackage(packagePath)
		if err != nil {
			os.Exit(1)
		}

		if _, ok := self.packages[name]; ok {
			core.PrintInfo("package", "Duplicate package %s found.", name)
			os.Exit(1)
		}

		self.packages[name] = make([]*manager.Package, 0, maxProcs)
		for i := 0; i < maxProcs; i++ {
			p := manager.NewPackage(packagePath, debug)
			self.startWebShim(p)

			self.packages[p.Name] = append(self.packages[p.Name], p)
		}
	}
}

func (self *PackageManager) startWebShim(p *manager.Package) {
	go func(p *manager.Package) {
		for {
			query := <-self.in
			v := p.Argshim.GetWebshimResponseForTest("lena", query.function, query.sampleId, query.sbag, query.files)
			result := WebShimResult{v}
			query.out <- result
		}
	}(p)
}
