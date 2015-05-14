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

func NewArgShim(argshimPath string, envs map[string]string, debug bool) *ArgShim {
	self := &ArgShim{}
	self.cmdPath = argshimPath
	self.debug = debug
	self.mutex = &sync.Mutex{}
	self.samplePipelinesMap = map[int]string{}

	cmd := exec.Command(self.cmdPath)
	cmd.Env = core.MergeEnv(envs)
	stdin, _ := cmd.StdinPipe()
	self.writer = bufio.NewWriterSize(stdin, 1000000)
	stdout, _ := cmd.StdoutPipe()
	self.reader = bufio.NewReaderSize(stdout, 1000000)
	cmd.Start()

	return self
}

func (self *ArgShim) invoke(function string, arguments []interface{}) (interface{}, error) {
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

	line, _, err := self.reader.ReadLine()
	self.mutex.Unlock()
	if err != nil {
		core.LogInfo("argshim", "Failed to read argshim %s output", self.cmdPath)
		return nil, err
	}
	if self.debug {
		fmt.Printf("%s\n\n", string(line))
	}

	var v interface{}
	if err := json.Unmarshal(line, &v); err != nil {
		core.LogError(err, "argshim", "Failed to convert argshim %s output to JSON: %s", self.cmdPath, line)
		return nil, err
	}
	return v, nil
}

func (self *ArgShim) getPipelineForSample(sample *Sample) string {
	self.mutex.Lock()
	pipeline, ok := self.samplePipelinesMap[sample.Id]
	self.mutex.Unlock()
	if ok {
		return pipeline
	}

	if v, err := self.invoke("getPipelineForSample", []interface{}{sample}); err == nil {
		if tv, ok := v.(string); ok {
			self.mutex.Lock()
			self.samplePipelinesMap[sample.Id] = tv
			self.mutex.Unlock()
			return tv
		}
	}
	return ""
}

func (self *ArgShim) buildArgsForRun(run *Run) map[string]interface{} {
	if v, err := self.invoke("buildArgsForRun", []interface{}{run}); err == nil {
		if tv, ok := v.(map[string]interface{}); ok {
			return tv
		}
	}
	return map[string]interface{}{}
}

func (self *ArgShim) buildArgsForSample(sbag interface{}, fastqPaths map[string]string) map[string]interface{} {
	if v, err := self.invoke("buildArgsForSample", []interface{}{sbag, fastqPaths}); err == nil {
		if tv, ok := v.(map[string]interface{}); ok {
			return tv
		}
	}
	return map[string]interface{}{}
}

func (self *ArgShim) buildCallSource(rt *core.Runtime, shimout map[string]interface{}, mroPath string) string {
	pipeline, ok := shimout["call"].(string)
	if !ok {
		return ""
	}
	args, ok := shimout["args"].(map[string]interface{})
	if !ok {
		return ""
	}
	sweepargs := []string{}
	if sweeplist, ok := shimout["sweepargs"].([]interface{}); ok {
		sweepargs = core.ArrayToString(sweeplist)
	}
	incpath := fmt.Sprintf("%s.mro", strings.ToLower(pipeline))
	src, _ := rt.BuildCallSource([]string{incpath}, pipeline, args, sweepargs, mroPath)
	return src
}

func (self *ArgShim) buildCallSourceForRun(rt *core.Runtime, run *Run, mroPath string) string {
	return self.buildCallSource(rt, self.buildArgsForRun(run), mroPath)
}

func (self *ArgShim) buildCallSourceForSample(rt *core.Runtime, sbag interface{}, fastqPaths map[string]string, mroPath string) string {
	return self.buildCallSource(rt, self.buildArgsForSample(sbag, fastqPaths), mroPath)
}
