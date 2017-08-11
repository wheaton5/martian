//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Martian pipeline viewer.
//
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"martian/core"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/martian-lang/docopt.go"
)

func runReprobeLoop(dir *Directory) {
	for {
		dir.reprobe()

		// Wait for a bit.
		time.Sleep(time.Second * time.Duration(5))
	}
}

func extractBugidFromBranch(branch string) string {
	parts := strings.Split(branch, "/")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

type Directory struct {
	startPort int
	pstances  map[string]map[string]interface{}
	config    map[string]interface{}
	mutex     sync.Mutex
}

func NewDirectory(startPort int, config interface{}) *Directory {
	self := &Directory{}
	self.startPort = startPort
	self.config = config.(map[string]interface{})
	self.pstances = make(map[string]map[string]interface{})
	return self
}

func (self *Directory) register(info map[string]interface{}) string {
	// Pick first available port starting from 5600.
	self.mutex.Lock()
	i := self.startPort
	port := ""
	for {
		port = strconv.Itoa(i)
		if _, ok := self.pstances[port]; !ok {
			if l, err := net.Listen("tcp", ":"+port); err == nil {
				l.Close()
				break
			}
		}
		i += 1
	}
	self.mutex.Unlock()

	// Register the info block.
	self.upsert(port, info)

	return port
}

func (self *Directory) getConfig() map[string]interface{} {
	return self.config
}

func (self *Directory) getSortedPipestances() []map[string]interface{} {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	sortedPorts := []string{}
	for port := range self.pstances {
		sortedPorts = append(sortedPorts, port)
	}
	sort.Strings(sortedPorts)
	sortedPstances := make([]map[string]interface{}, 0, len(sortedPorts))
	for _, port := range sortedPorts {
		sortedPstances = append(sortedPstances, self.pstances[port])
	}
	return sortedPstances
}

func (self *Directory) upsert(port string, info map[string]interface{}) {
	//fmt.Printf("upsert %s\n", port)
	self.mutex.Lock()
	info["port"] = port
	if branch, ok := info["mrobranch"].(string); ok {
		info["mrobug_id"] = extractBugidFromBranch(branch)
	}
	self.pstances[port] = info
	self.mutex.Unlock()
}

func (self *Directory) remove(port string) {
	//fmt.Printf("remove %s\n", port)
	self.mutex.Lock()
	delete(self.pstances, port)
	self.mutex.Unlock()
}

func (self *Directory) probe(hostname string, port string, wg *sync.WaitGroup) {
	//fmt.Printf("probing %s:%s\n", hostname, port)
	defer wg.Done()
	u := fmt.Sprintf("http://%s:%s/api/get-info", hostname, port)
	if res, err := http.Get(u); err == nil {
		if content, err := ioutil.ReadAll(res.Body); err == nil {
			//fmt.Printf("%d %s\n", res.StatusCode, string(content))
			if res.StatusCode == 200 {
				var info map[string]interface{}
				if err := json.Unmarshal(content, &info); err == nil {
					self.upsert(port, info)
					return
				}
			} else {
				// For old mrp's that don't have get-info, we still
				// want to consume the port so we don't give out a
				// used port to new mrp's.
				self.upsert(port, map[string]interface{}{})
				return
			}
		}
	}
	self.remove(port)
}

func (self *Directory) reprobe() {
	self.mutex.Lock()
	pstances := make(map[string]map[string]interface{}, len(self.pstances))
	for port, pstance := range self.pstances {
		pstances[port] = pstance
	}
	self.mutex.Unlock()

	var wg sync.WaitGroup
	for port, pstance := range pstances {
		if host, ok := pstance["hostname"].(string); ok {
			wg.Add(1)
			go self.probe(host, port, &wg)
		} else {
			self.remove(port)
		}
	}
	wg.Wait()
}

func (self *Directory) discover() {
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go self.probe("localhost", strconv.Itoa(i+self.startPort), &wg)
	}
	wg.Wait()
}

func main() {
	core.SetupSignalHandlers()

	//=========================================================================
	// Commandline argument and environment variables.
	//=========================================================================
	// Parse commandline.
	doc := `Martian Pipeline Viewer.

Usage:
    mrv [options]
    mrv -h | --help | --version

Options:
    --port=<num>     Serve UI at http://<hostname>:<num>
    --dport=<num>    Starting port number available to mrp's.
    --config=<file>  JSON file with user names and avatar URLs.
    -h --help        Show this message.
    --version        Show version.`
	martianVersion := core.GetVersion()
	opts, _ := docopt.Parse(doc, nil, true, martianVersion, false)
	core.LogInfo("*", "Martian Reverse-Proxy Viewer")
	core.LogInfo("version", martianVersion)
	core.LogInfo("cmdline", strings.Join(os.Args, " "))

	// Compute UI port.
	uiport := "8080"
	if value := opts["--port"]; value != nil {
		uiport = value.(string)
	}

	// Compute distributed port.
	dport := 5600
	if value := opts["--dport"]; value != nil {
		if num, err := strconv.Atoi(value.(string)); err == nil {
			dport = num
		}
	}

	// Load the configuration file.
	var config map[string]interface{}
	if configfile := opts["--config"]; configfile != nil {
		if bytes, err := ioutil.ReadFile(configfile.(string)); err == nil {
			if err := json.Unmarshal(bytes, &config); err != nil {
				fmt.Printf("%s\n", err.Error())
				os.Exit(1)
			}
		} else {
			fmt.Printf("%s\n", err.Error())
			os.Exit(1)
		}
	}

	// Create the directory.
	dir := NewDirectory(dport, config)

	// Discover existing mrps.
	dir.discover()

	// Start web server.
	go runWebServer(uiport, dir)

	// Start reprobe loop.
	go runReprobeLoop(dir)

	// Let daemons take over.
	done := make(chan bool)
	<-done
}
