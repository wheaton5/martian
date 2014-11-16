//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Mario pipeline viewer.
//
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mario/core"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/docopt/docopt-go"
)

func extractBugidFromBranch(branch string) string {
	parts := strings.Split(branch, "/")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

type Directory struct {
	startPort int
	pstances  map[string]map[string]string
	config    map[string]interface{}
	mutex     sync.Mutex
}

func NewDirectory(startPort int, config interface{}) *Directory {
	self := &Directory{}
	self.startPort = startPort
	self.config = config.(map[string]interface{})
	self.pstances = map[string]map[string]string{}
	return self
}

func (self *Directory) deregister(port string) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	delete(self.pstances, port)
}

func (self *Directory) register(info map[string]string) string {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	// Pick first available port starting from 5600.
	i := self.startPort
	port := ""
	for {
		port = strconv.Itoa(i)
		if _, ok := self.pstances[port]; !ok {
			break
		}
		i += 1
	}

	// Register the info block.
	info["port"] = port
	info["mrobug_id"] = extractBugidFromBranch(info["mrobranch"])
	self.pstances[port] = info

	return port
}

func (self *Directory) getConfig() map[string]interface{} {
	return self.config
}

func (self *Directory) getSortedPipestances() []map[string]string {
	sortedPorts := []string{}
	for port, _ := range self.pstances {
		sortedPorts = append(sortedPorts, port)
	}
	sort.Strings(sortedPorts)
	sortedPstances := []map[string]string{}
	for _, port := range sortedPorts {
		sortedPstances = append(sortedPstances, self.pstances[port])
	}
	return sortedPstances
}

func main() {
	core.SetupSignalHandlers()

	//=========================================================================
	// Commandline argument and environment variables.
	//=========================================================================
	// Parse commandline.
	doc := `Mario Pipeline Viewer.

Usage: 
    mrv [options]
    mrv -h | --help | --version

Options:
    --port=<num>     Serve UI at http://localhost:<num>
    --config=<file>  JSON file with user names and avatar URLs.
    -h --help        Show this message.
    --version        Show version.`
	marioVersion := core.GetVersion()
	opts, _ := docopt.Parse(doc, nil, true, marioVersion, false)
	core.LogInfo("*", "Mario Reverse-Proxy Viewer")
	core.LogInfo("version", marioVersion)
	core.LogInfo("cmdline", strings.Join(os.Args, " "))

	// Compute UI port.
	uiport := "8080"
	if value := opts["--port"]; value != nil {
		uiport = value.(string)
	}

	// Load the configuration file.
	var config interface{}
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
	directory := NewDirectory(5600, config)

	// Start web server.
	runWebServer(uiport, directory)

	// Let daemons take over.
	done := make(chan bool)
	<-done
}
