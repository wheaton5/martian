//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"martian/core"
	"martian/manager"
	"os"
	"path"
)

type WebShimQuery struct {
	function string
	id       string
	bag      interface{}
	files    map[string]interface{}
	out      chan WebShimResult
}

type WebShimResult struct {
	v   interface{}
	err error
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

func (self *PackageManager) GetWebshimResponseForSample(id string, product string, function string, bag interface{}, files map[string]interface{}) (interface{}, error) {
	if _, ok := self.packages[product]; ok {
		out := make(chan WebShimResult)
		query := WebShimQuery{function, id, bag, files, out}
		self.in <- query
		result := <-out
		return result.v, result.err
	}
	return nil, errors.New(fmt.Sprintf("Product %s not found", product))
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
			v, err := p.Argshim.GetWebshimResponseForTest("lena", query.function, query.id, query.bag, query.files)
			result := WebShimResult{v, err}
			query.out <- result
		}
	}(p)
}
