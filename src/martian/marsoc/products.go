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
	"strings"
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
	Name        string                     `json:"name"`
	ArgshimPath string                     `json:"argshim_path"`
	MroPath     string                     `json:"mro_path"`
	Envs        map[string]*ProductJsonEnv `json:"envs"`
}

type ProductJsonEnv struct {
	Value string `json:"value"`
	Type  string `json:"type"`
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

func (self *ProductManager) getProducts() []*Product {
	products := []*Product{}
	for _, product := range self.products {
		products = append(products, product)
	}
	return products
}

// Argshim functions
func (self *ProductManager) getPipelineForSample(sample *Sample) string {
	if product, ok := self.products[sample.Product]; ok {
		return product.argshim.getPipelineForSample(sample)
	}
	return ""
}

func (self *ProductManager) buildCallSourceForRun(run *Run) string {
	product := self.products[self.defaultProduct]
	return product.argshim.buildCallSourceForRun(run)
}

func (self *ProductManager) buildCallSourceForSample(sbag interface{}, fastqPaths map[string]string, sample *Sample) string {
	if product, ok := self.products[sample.Product]; ok {
		return product.argshim.buildCallSourceForSample(sbag, fastqPaths)
	}
	return ""
}

// Pipestance manager functions
func (self *ProductManager) getPipestanceEnvironment(psid string) (string, string, map[string]string, error) {
	if sample := self.lena.getSampleWithId(psid); sample != nil {
		if product, ok := self.products[sample.Product]; ok {
			self.mutex.Lock()
			defer self.mutex.Unlock()

			return product.mroPath, product.mroVersion, product.envs, nil
		}
	}
	return "", "", nil, &core.MartianError{fmt.Sprintf("ProductManagerError: Failed to get environment for pipestance '%s'.", psid)}
}

func (self *ProductManager) getDefaultPipestanceEnvironment() (string, string, map[string]string, error) {
	product := self.products[self.defaultProduct]

	self.mutex.Lock()
	defer self.mutex.Unlock()

	return product.mroPath, product.mroVersion, product.envs, nil
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

	envs := map[string]string{}
	for key, envJson := range productJson.Envs {
		value := envJson.Value
		switch envJson.Type {
		case "path":
			value = parsePathEnv(value, productPath)
		case "path_prepend":
			value = parsePathEnv(value, productPath)
			if prefix := os.Getenv(key); len(prefix) > 0 {
				// Prepend value to current environment variable
				value = value + ":" + prefix
			}
		case "string":
			break
		default:
			core.PrintInfo("product", "Unsupported env variable type %s.", envJson.Type)
			os.Exit(1)
		}
		if _, ok := envs[key]; ok {
			core.PrintInfo("product", "Duplicate env variable %s found.", key)
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
