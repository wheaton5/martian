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
	"martian/core"
	"os/exec"
	"strings"
	"sync"
)

type ArgShim struct {
	cmdPath            string
	debug              bool
	samplePipelinesMap map[int]string
	writer             *bufio.Writer
	reader             *bufio.Reader
	mutex              *sync.Mutex
}

func NewArgShim(argshimPath string, debug bool) *ArgShim {
	self := &ArgShim{}
	self.cmdPath = argshimPath
	self.debug = debug
	self.mutex = &sync.Mutex{}
	self.samplePipelinesMap = map[int]string{}

	cmd := exec.Command(self.cmdPath)
	stdin, _ := cmd.StdinPipe()
	self.writer = bufio.NewWriterSize(stdin, 1000000)
	stdout, _ := cmd.StdoutPipe()
	self.reader = bufio.NewReaderSize(stdout, 1000000)
	cmd.Start()

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

func (self *ArgShim) getPipelineForSample(sample *Sample) string {
	if pipeline, ok := self.samplePipelinesMap[sample.Id]; ok {
		return pipeline
	}
	v := self.invoke("getPipelineForSample", []interface{}{sample})
	if tv, ok := v.(string); ok {
		self.samplePipelinesMap[sample.Id] = tv
		return tv
	}
	return ""
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
	pipeline, ok := shimout["call"].(string)
	if !ok {
		return ""
	}
	args, ok := shimout["args"].(map[string]interface{})
	if !ok {
		return ""
	}
	incpath := fmt.Sprintf("%s.mro", strings.ToLower(pipeline))
	src, _ := rt.BuildCallSource([]string{incpath}, pipeline, args)
	return src
}

func (self *ArgShim) buildCallSourceForRun(rt *core.Runtime, run *Run) string {
	return self.buildCallSource(rt, self.buildArgsForRun(run))
}

func (self *ArgShim) buildCallSourceForSample(rt *core.Runtime, sbag interface{}, fastqPaths map[string]string) string {
	return self.buildCallSource(rt, self.buildArgsForSample(sbag, fastqPaths))
}
