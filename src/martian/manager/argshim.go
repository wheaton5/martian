//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Marsoc argshim shim.
//
package manager

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"martian/core"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

type TestKey struct {
	category string
	id       string
}

type ArgShim struct {
	cmd              *exec.Cmd
	cmdPath          string
	debug            bool
	envs             map[string]string
	testPipelinesMap map[TestKey]string
	stdin            io.WriteCloser
	stdout           io.ReadCloser
	writer           *bufio.Writer
	reader           *bufio.Reader
	mutex            *sync.Mutex
}

func NewArgShim(argshimPath string, envs map[string]string, debug bool) *ArgShim {
	self := &ArgShim{}
	self.cmdPath = argshimPath
	self.envs = envs
	self.debug = debug
	self.mutex = &sync.Mutex{}
	self.testPipelinesMap = map[TestKey]string{}

	self.start()
	return self
}

func (self *ArgShim) start() {
	self.cmd = exec.Command(self.cmdPath)
	self.cmd.Env = core.MergeEnv(self.envs)
	self.stdin, _ = self.cmd.StdinPipe()
	self.writer = bufio.NewWriterSize(self.stdin, 1000000)
	self.stdout, _ = self.cmd.StdoutPipe()
	self.reader = bufio.NewReaderSize(self.stdout, 1000000)
	self.cmd.Start()
}

func (self *ArgShim) Kill() {
	self.stdin.Close()
	self.stdout.Close()

	pid := self.cmd.Process.Pid
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		core.LogError(err, "argshim", "Failed to kill argshim process with PID %d", pid)
	}
	self.cmd.Wait()
}

func (self *ArgShim) Restart() {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	self.Kill()
	self.start()
}

func (self *ArgShim) readAll() ([]byte, error) {
	// Block until new line or error
	line, err := self.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	// Read rest of the bytes available
	numBytes := self.reader.Buffered()
	if numBytes > 0 {
		buf := make([]byte, numBytes)
		if _, err := self.reader.Read(buf); err != nil {
			return nil, err
		}
		line = append(line, buf...)
	}

	// Trim new lines from end of string
	line = bytes.TrimRight(line, "\n")

	return line, nil
}

func (self *ArgShim) invoke(function string, arguments []interface{}) (interface{}, error) {
	input := map[string]interface{}{
		"function":  function,
		"arguments": arguments,
	}
	data, _ := json.Marshal(input)
	if self.debug {
		fmt.Printf("%s\n\n", string(data))
	}

	self.mutex.Lock()
	self.writer.Write([]byte(string(data) + "\n"))
	self.writer.Flush()

	line, err := self.readAll()
	self.mutex.Unlock()
	if err != nil {
		core.LogInfo("argshim", "Failed to read argshim %s output", self.cmdPath)
		return nil, err
	}
	if self.debug {
		fmt.Printf("%s\n\n", string(line))
	}

	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		msg := fmt.Sprintf("Failed to convert argshim %s output to JSON: %s", self.cmdPath, line)
		core.LogError(err, "argshim", msg)
		return nil, errors.New(msg)
	}
	return v, nil
}

func (self *ArgShim) GetPipelineForTest(category string, id string, sbag interface{}) string {
	skey := TestKey{category, id}

	self.mutex.Lock()
	pipeline, ok := self.testPipelinesMap[skey]
	self.mutex.Unlock()
	if ok {
		return pipeline
	}

	if v, err := self.invoke("getPipelineForTest", []interface{}{category, id, sbag}); err == nil {
		if tv, ok := v.(string); ok {
			self.mutex.Lock()
			self.testPipelinesMap[skey] = tv
			self.mutex.Unlock()
			return tv
		}
	}
	return ""
}

func (self *ArgShim) buildArgsForRun(rbag interface{}) map[string]interface{} {
	if v, err := self.invoke("buildArgsForRun", []interface{}{rbag}); err == nil {
		if tv, ok := v.(map[string]interface{}); ok {
			return tv
		}
	}
	return map[string]interface{}{}
}

func (self *ArgShim) buildArgsForTest(category string, id string, sbag interface{},
	fastqPaths map[string]string) map[string]interface{} {
	if v, err := self.invoke("buildArgsForTest", []interface{}{category, id, sbag, fastqPaths}); err == nil {
		if tv, ok := v.(map[string]interface{}); ok {
			return tv
		}
	}
	return map[string]interface{}{}
}

func (self *ArgShim) buildCallSource(rt *core.Runtime, shimout map[string]interface{}, mroPaths []string) string {
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
	src, _ := rt.BuildCallSource([]string{incpath}, pipeline, args, sweepargs, mroPaths)
	return src
}

func (self *ArgShim) BuildCallSourceForRun(rt *core.Runtime, rbag interface{}, mroPaths []string) string {
	return self.buildCallSource(rt, self.buildArgsForRun(rbag), mroPaths)
}

func (self *ArgShim) BuildCallSourceForTest(rt *core.Runtime, category string, id string, sbag interface{},
	fastqPaths map[string]string, mroPaths []string) string {
	return self.buildCallSource(rt, self.buildArgsForTest(category, id, sbag, fastqPaths), mroPaths)
}

func (self *ArgShim) GetWebshimResponseForTest(category string, function string, id string, sbag interface{},
	files interface{}) (interface{}, error) {
	v, err := self.invoke(function, []interface{}{category, id, sbag, files})
	if err != nil {
		return nil, err
	}
	return v, nil
}
