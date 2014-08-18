//
// Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
//
// Marsoc argshim shim.
//
package main

import (
	"encoding/json"
	"fmt"
	"margo/core"
	"os/exec"
	"path"
	"strings"
)

type ArgShim struct {
	cmdPath            string
	samplePipelinesMap map[string]string
}

func NewArgShim(pipelinesPath string) *ArgShim {
	self := &ArgShim{}
	self.cmdPath = path.Join(pipelinesPath, "argshim", "marsoc.coffee")
	self.samplePipelinesMap = self.getSamplePipelinesMap()
	return self
}

func (self *ArgShim) invoke(function string, arguments []interface{}) interface{} {
	input := map[string]interface{}{
		"function":  function,
		"arguments": arguments,
	}
	bytes, _ := json.Marshal(input)

	cmd := exec.Command(self.cmdPath)
	cmd.Stdin = strings.NewReader(string(bytes))
	out, _ := cmd.Output()

	var v interface{}
	json.Unmarshal(out, &v)
	return v
}

func (self *ArgShim) getSamplePipelinesMap() map[string]string {
	v := self.invoke("getSamplePipelinesMap", []interface{}{})
	if tv, ok := v.(map[string]string); ok {
		return tv
	}
	return map[string]string{}
}

func (self *ArgShim) getPipelineForSample(sample interface{}) string {
	pipeline := self.samplePipelinesMap[sample.(map[string]interface{})["workflow"].(map[string]interface{})["name"].(string)]
	fmt.Println(pipeline)
	return pipeline
}

func (self *ArgShim) buildArgsForRun(run interface{}) map[string]interface{} {
	v := self.invoke("buildArgsForRun", []interface{}{
		run,
	})
	if tv, ok := v.(map[string]interface{}); ok {
		return tv
	}
	return map[string]interface{}{}
}

func (self *ArgShim) buildArgsForSample(run interface{}, sample interface{}, preprocess_outs interface{}) map[string]interface{} {
	v := self.invoke("buildArgsForSample", []interface{}{
		run,
		sample,
		preprocess_outs,
	})
	if tv, ok := v.(map[string]interface{}); ok {
		return tv
	}
	return map[string]interface{}{}
}

func (self *ArgShim) buildCallSourceForRun(rt *core.Runtime, run interface{}) string {
	shimout := self.buildArgsForRun(run)
	pipeline, ok := shimout["pipeline"].(string)
	if !ok {
		return ""
	}
	args, ok := shimout["args"].(map[string]interface{})
	if !ok {
		return ""
	}
	return rt.BuildCallSource(pipeline, args)
}
