//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"martian/manager"
	"martian/util"
	"os"
	"path"
	"sync"
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
	packages map[string]*manager.Package
	in       map[string]chan WebShimQuery
	mutex    *sync.Mutex
}

func NewPackageManager(packagesPath string, maxProcs int) *PackageManager {
	self := &PackageManager{}
	self.packages = map[string]*manager.Package{}
	self.in = map[string]chan WebShimQuery{}
	self.mutex = &sync.Mutex{}

	self.verifyPackages(packagesPath, maxProcs)

	util.LogInfo("package", "%d packages found.", len(self.packages))
	return self
}

func (self *PackageManager) GetWebshimResponseForSample(id string, product string, function string, bag interface{}, files map[string]interface{}) (interface{}, error) {
	if _, ok := self.packages[product]; ok {
		out := make(chan WebShimResult)
		query := WebShimQuery{function, id, bag, files, out}
		self.in[product] <- query
		result := <-out
		return result.v, result.err
	}
	return nil, errors.New(fmt.Sprintf("Product %s not found", product))
}

func (self *PackageManager) verifyPackages(packagesPath string, maxProcs int) {
	infos, err := ioutil.ReadDir(packagesPath)
	if err != nil {
		util.PrintInfo("package", "Packages path %s does not exist.", packagesPath)
		os.Exit(1)
	}
	for _, info := range infos {
		packagePath := path.Join(packagesPath, info.Name())
		name, _, _, _, _, _, err := manager.VerifyPackage(packagePath)
		if err != nil {
			os.Exit(1)
		}

		if _, ok := self.packages[name]; ok {
			util.PrintInfo("package", "Duplicate package %s found.", name)
			os.Exit(1)
		}

		p := manager.NewPackage(packagePath, false)

		// Kill package argshim process since a new argshim process is started per request
		p.Argshim.Kill()

		self.packages[name] = p
		self.in[name] = make(chan WebShimQuery)
		for i := 0; i < maxProcs; i++ {
			self.startWebShim(p)
		}
	}
}

func (self *PackageManager) startWebShim(p *manager.Package) {
	go func(p *manager.Package) {
		for {
			query := <-self.in[p.Name]
			argshim := manager.NewArgShim(p.ArgshimPath, p.Envs, false)
			v, err := argshim.GetWebshimResponseForTest("lena", query.function, query.id, query.bag, query.files)
			argshim.Kill()
			result := WebShimResult{v, err}
			query.out <- result
		}
	}(p)
}
