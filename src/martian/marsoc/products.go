//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Marsoc product manager.
//
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"martian/core"
	"os"
	"path"
	"sync"
	"time"
)

type ProductManager struct {
	defaultProduct string
	products       map[string]*Product
	mutex          *sync.Mutex
	lena           *Lena
}

type Product struct {
	name        string
	argshimPath string
	mroPath     string
	mroVersion  string
	envs        map[string]string
	argshim     *ArgShim
}

type ProductJson struct {
	Name        string            `json:"name"`
	ArgshimPath string            `json:"argshim_path"`
	MroPath     string            `json:"mro_path"`
	Envs        map[string]string `json:"envs"`
}

func NewProductManager(productsPath string, defaultProduct string, debug bool, lena *Lena) *ProductManager {
	self := &ProductManager{}
	self.mutex = &sync.Mutex{}
	self.defaultProduct = defaultProduct
	self.lena = lena
	self.products = verifyProducts(productsPath, defaultProduct, debug)

	core.LogInfo("product", "%d products found.", len(self.products))
	self.refreshVersions()
	return self
}

func NewProduct(productPath string, debug bool) *Product {
	self := &Product{}
	self.name, self.argshimPath, self.mroPath, self.envs = verifyProduct(productPath)
	self.mroVersion = core.GetMroVersion(self.mroPath)
	self.argshim = NewArgShim(self.argshimPath, debug)
	return self
}

// Argshim functions
func (self *ProductManager) getPipelineForSample(sample *Sample) string {
	if product, ok := self.products[sample.Product]; ok {
		return product.argshim.getPipelineForSample(sample)
	}
	return ""
}

func (self *ProductManager) buildCallSourceForRun(rt *core.Runtime, run *Run) string {
	if product, ok := self.products[self.defaultProduct]; ok {
		return product.argshim.buildCallSourceForRun(rt, run)
	}
	return ""
}

func (self *ProductManager) buildCallSourceForSample(rt *core.Runtime, sbag interface{}, fastqPaths map[string]string, sample *Sample) string {
	if product, ok := self.products[sample.Product]; ok {
		return product.argshim.buildCallSourceForSample(rt, sbag, fastqPaths)
	}
	return ""
}

// Pipestance manager functions
func (self *ProductManager) getPipestanceEnvironment(psid string) (string, string, map[string]string, error) {
	if sample := self.lena.getSampleWithId(psid); sample != nil {
		if product, ok := self.products[sample.Product]; ok {
			self.mutex.Lock()
			mroPath, mroVersion, envs := product.mroPath, product.mroVersion, product.envs
			self.mutex.Unlock()

			return mroPath, mroVersion, envs, nil
		}
	}
	return "", "", nil, &core.MartianError{fmt.Sprintf("ProductManagerError: Failed to get environment for pipestance '%s'.", psid)}
}

// Version functions
func (self *ProductManager) refreshVersions() {
	go func() {
		for {
			self.mutex.Lock()
			for _, product := range self.products {
				product.mroVersion = core.GetMroVersion(product.mroPath)
			}
			self.mutex.Unlock()

			time.Sleep(time.Minute * time.Duration(5))
		}
	}()
}

func (self *ProductManager) GetMroVersion() string {
	// Gets version from default product
	self.mutex.Lock()
	product := self.products[self.defaultProduct]
	mroVersion := product.mroVersion
	self.mutex.Unlock()
	return mroVersion
}

// Product config verification
func verifyProducts(productsPath string, defaultProduct string, debug bool) map[string]*Product {
	products := map[string]*Product{}

	infos, err := ioutil.ReadDir(productsPath)
	if err != nil {
		core.PrintInfo("product", "Products path %s does not exist.", productsPath)
		os.Exit(1)
	}
	for _, info := range infos {
		productPath := path.Join(productsPath, info.Name())

		product := NewProduct(productPath, debug)
		if _, ok := products[product.name]; ok {
			core.PrintInfo("product", "Duplicate product %s found.", product.name)
			os.Exit(1)
		}
		products[product.name] = product
	}
	if _, ok := products[defaultProduct]; !ok {
		core.PrintInfo("product", "Default product %s not found.", defaultProduct)
		os.Exit(1)
	}
	return products
}

func verifyProduct(productPath string) (string, string, string, map[string]string) {
	productFile := path.Join(productPath, "marsoc.json")
	if _, err := os.Stat(productFile); os.IsNotExist(err) {
		core.PrintInfo("product", "Product config file %s does not exist.", productFile)
		os.Exit(1)
	}
	bytes, _ := ioutil.ReadFile(productFile)

	var productJson *ProductJson
	if err := json.Unmarshal(bytes, &productJson); err != nil {
		core.PrintInfo("product", "Product config file %s does not contain valid JSON.", productFile)
		os.Exit(1)
	}

	argshimPath := path.Join(productPath, productJson.ArgshimPath)
	if _, err := os.Stat(argshimPath); err != nil {
		core.PrintInfo("product", "Product argshim file %s does not exist.", argshimPath)
		os.Exit(1)
	}

	mroPath := path.Join(productPath, productJson.MroPath)
	if _, err := os.Stat(mroPath); err != nil {
		core.PrintInfo("product", "Product mro path %s does not exist.", mroPath)
		os.Exit(1)
	}

	name := productJson.Name
	envs := productJson.Envs

	return name, argshimPath, mroPath, envs
}
