//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Marsoc argshim shim.
//
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"mario/core"
	"os/exec"
	"strings"
	"sync"
)

type ArgShim struct {
	cmdPath            string
	debug              bool
	samplePipelinesMap map[string]string
	writer             *bufio.Writer
	reader             *bufio.Reader
	mutex              *sync.Mutex
}

func NewArgShim(argshimPath string, debug bool) *ArgShim {
	self := &ArgShim{}
	self.cmdPath = argshimPath
	self.debug = debug
	self.mutex = &sync.Mutex{}

	cmd := exec.Command(self.cmdPath)
	stdin, _ := cmd.StdinPipe()
	self.writer = bufio.NewWriterSize(stdin, 1000000)
	stdout, _ := cmd.StdoutPipe()
	self.reader = bufio.NewReaderSize(stdout, 1000000)
	cmd.Start()

	self.samplePipelinesMap = self.getSamplePipelinesMap()

	return self
}

func (self *ArgShim) invoke(function string, arguments []interface{}) interface{} {
	input := map[string]interface{}{
		"function":  function,
		"arguments": arguments,
	}
	bytes, _ := json.Marshal(input)
	if self.debug {
		fmt.Printf("%s\n\n", string(bytes))
	}

	self.mutex.Lock()
	self.writer.Write([]byte(string(bytes) + "\n"))
	self.writer.Flush()

	line, _, _ := self.reader.ReadLine()
	if self.debug {
		fmt.Printf("%s\n\n", string(line))
	}
	self.mutex.Unlock()

	var v interface{}
	json.Unmarshal(line, &v)
	return v
}

func (self *ArgShim) getSamplePipelinesMap() map[string]string {
	// Just capture the sample to pipeline map and do lookups locally.
	// No sense in invoking Node.js each time.
	v := self.invoke("getSamplePipelinesMap", []interface{}{})
	if tv, ok := v.(map[string]interface{}); ok {
		ntv := map[string]string{}
		for k, v := range tv {
			ntv[k] = v.(string)
		}
		return ntv
	}
	return map[string]string{}
}

func (self *ArgShim) getPipelineForSample(sample *Sample) string {
	return self.samplePipelinesMap[sample.Workflow.Name]
}

func (self *ArgShim) buildArgsForRun(run *Run) map[string]interface{} {
	v := self.invoke("buildArgsForRun", []interface{}{run})
	if tv, ok := v.(map[string]interface{}); ok {
		return tv
	}
	return map[string]interface{}{}
}

func (self *ArgShim) buildArgsForSample(sbag interface{}, fastqPaths map[string]string) map[string]interface{} {
	v := self.invoke("buildArgsForSample", []interface{}{sbag, fastqPaths})
	if tv, ok := v.(map[string]interface{}); ok {
		return tv
	}
	return map[string]interface{}{}
}

func (self *ArgShim) buildCallSource(rt *core.Runtime, shimout map[string]interface{}) string {
	pipeline, ok := shimout["pipeline"].(string)
	if !ok {
		return ""
	}
	args, ok := shimout["args"].(map[string]interface{})
	if !ok {
		return ""
	}
	incpath := fmt.Sprintf("%s.mro", strings.ToLower(pipeline))
	return rt.BuildCallSource([]string{incpath}, pipeline, args)
}

func (self *ArgShim) buildCallSourceForRun(rt *core.Runtime, run *Run) string {
	return self.buildCallSource(rt, self.buildArgsForRun(run))
}

func (self *ArgShim) buildCallSourceForSample(rt *core.Runtime, sbag interface{}, fastqPaths map[string]string) string {
	return self.buildCallSource(rt, self.buildArgsForSample(sbag, fastqPaths))
}
