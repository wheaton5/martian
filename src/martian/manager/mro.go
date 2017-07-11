package manager

import (
	"errors"
	"fmt"
	"martian/core"
	"path/filepath"
	"regexp"
	"strings"
)

const CHROMIUM string = "chromium"
const GEMCODE string = "gemcode"

type SampleDef struct {
	bc_length      int
	bc_in_read     int
	sample_names   []string
	sample_indices []string
	read_path      string
	gem_group      int
	lanes          []int
	// cellranger (GemCode?) spec
	bc_read_type string
	fastq_mode   string
	si_read_type string
}

func mapInt(jsonDef map[string]interface{}, key string) (int, bool) {
	iface, found := jsonDef[key]
	var num int
	if found && iface != nil {
		num = int(iface.(int64))
	} else {
		found = false
	}
	return num, found
}

func mapString(jsonDef map[string]interface{}, key string) (string, bool) {
	iface, found := jsonDef[key]
	var str string
	if found && iface != nil {
		str = iface.(string)
	} else {
		found = false
	}
	return str, found
}

func mapIntArray(jsonDef map[string]interface{}, key string) ([]int, bool) {
	intArr := []int{}
	iface, found := jsonDef[key]
	if found && iface != nil {
		array := iface.([]interface{})
		if len(array) > 0 {
			intArr = make([]int, len(array))
			for i, num := range array {
				intArr[i] = int(num.(int64))
			}
		} else {
			found = false
		}
	} else {
		found = false
	}
	return intArr, found
}

func mapStringArray(jsonDef map[string]interface{}, key string) ([]string, bool) {
	stringArr := []string{}
	iface, found := jsonDef[key]
	if found && iface != nil {
		array := iface.([]interface{})
		if len(array) > 0 {
			stringArr = make([]string, len(array))
			for i, str := range array {
				stringArr[i] = str.(string)
			}
		} else {
			found = false
		}
	} else {
		found = false
	}
	return stringArr, found
}

// TODO-- ASK AROUND: can we get sample_def JSON source from MRO to use unmarshaler?
// or is it just un-JSON-like enough to not be able to use it?
func NewSampleDef(jsonDef map[string]interface{}) *SampleDef {
	sampleDef := &SampleDef{sample_indices: []string{"any"}}

	bc_length, found := mapInt(jsonDef, "bc_length")
	if found {
		sampleDef.bc_length = bc_length
	}
	bc_in_read, found := mapInt(jsonDef, "bc_in_read")
	if found {
		sampleDef.bc_in_read = bc_in_read
	}
	names, found := mapStringArray(jsonDef, "sample_names")
	if found {
		sampleDef.sample_names = names
	}
	indices, found := mapStringArray(jsonDef, "sample_indices")
	if found {
		sampleDef.sample_indices = indices
	}
	readPath, found := mapString(jsonDef, "read_path")
	if found {
		sampleDef.read_path = readPath
	}
	gem_group, found := mapInt(jsonDef, "gem_group")
	if found {
		sampleDef.gem_group = gem_group
	}
	lanes, found := mapIntArray(jsonDef, "lanes")
	if found {
		sampleDef.lanes = lanes
	}
	bc_read_type, found := mapString(jsonDef, "bc_read_type")
	if found {
		sampleDef.bc_read_type = bc_read_type
	}
	si_read_type, found := mapString(jsonDef, "si_read_type")
	if found {
		sampleDef.si_read_type = si_read_type
	}
	fastq_mode, found := mapString(jsonDef, "fastq_mode")
	if found {
		sampleDef.fastq_mode = fastq_mode
	}
	return sampleDef
}

func BclPathsFromRunPath(runPath string) ([]string, error) {
	allPaths := []string{}
	if !strings.HasPrefix(runPath, "/") {
		absPath, err := filepath.Abs(runPath)
		if err != nil {
			core.LogError(err, "storage", "can't get abs path of bcl run_path")
			return allPaths, err
		}
		runPath = absPath
	}
	if path, err := filepath.EvalSymlinks(runPath); err == nil {
		runPath = path
	} else {
		core.LogError(err, "storage", "eval symlinks doesn't work")
		return allPaths, err
	}
	return SequencerBclPaths(runPath), nil
}

func FastqPathsFromSampleDef(sampleDef *SampleDef) ([]string, error) {
	allPaths := []string{}
	readPath := sampleDef.read_path
	if !strings.HasPrefix(readPath, "/") {
		absPath, err := filepath.Abs(readPath)
		if err != nil {
			core.LogError(err, "storage", "Error getting absolute path: %s", readPath)
			return allPaths, err
		}
		readPath = absPath
	}
	readPath, err := filepath.EvalSymlinks(sampleDef.read_path)
	if err != nil {
		core.LogError(err, "storage", "Error evaluating symlink: %s", sampleDef.read_path)
		return allPaths, err
	}
	sampleOligos := []string{}
	for _, sampleIndex := range sampleDef.sample_indices {
		if oligos, ok := SAMPLE_INDEX_MAP[sampleIndex]; ok {
			sampleOligos = append(sampleOligos, oligos...)
		} else if sampleIndex == "any" {
			sampleOligos = append(sampleOligos, "*")
		} else if ok, _ := regexp.MatchString("[ACGNT]{8}", sampleIndex); ok {
			sampleOligos = append(sampleOligos, sampleIndex)
		} else {
			core.LogError(errors.New("Unknown sample index"), "storage", "Unknown sample index: %s", sampleIndex)
			return nil, &core.PipestanceSizeError{fmt.Sprintf("Unknown sample index: %s", sampleIndex)}
		}
	}
	for _, sampleIndex := range sampleOligos {
		var filePaths []string
		if sampleDef.si_read_type != "" {
			filePaths = BclProcessorCRFastqPaths(readPath, sampleIndex, sampleDef.lanes,
				sampleDef.bc_read_type, sampleDef.si_read_type)
		} else if sampleDef.bc_in_read == 0 {
			filePaths = BclProcessorCRFastqPaths(readPath, sampleIndex, sampleDef.lanes, "I1", "I2")
		} else {
			filePaths = BclProcessorWGSFastqPaths(readPath, sampleIndex, sampleDef.lanes)
		}
		allPaths = append(allPaths, filePaths...)
	}
	return allPaths, nil
}

func InvocationFromMRO(source string, srcPath string, mroPaths []string) *core.InvocationData {
	rt := core.NewRuntime("local", "disable", "disable", core.GetVersion())
	invocationData, err := rt.BuildCallData(source, srcPath, mroPaths)
	if err != nil {
		core.LogError(err, "storage", "Error getting MRO invocation: %s", srcPath)
	}
	return invocationData
}

func FastqFilesFromInvocation(invocation *core.InvocationData) ([]string, error) {
	args := invocation.Args
	if _, ok := args["sample_def"]; !ok {
		psname := PipelineFromInvocation(invocation)
		err := &core.PipestanceSizeError{psname}
		core.LogError(err, "storage", "Pipeline without sample def: %s", psname)
		return nil, err
	}
	sampleDefsJson := args["sample_def"].([]interface{})
	sampleDefs := []*SampleDef{}
	allPaths := []string{}
	for _, sampleDefJson := range sampleDefsJson {
		sampleDefs = append(sampleDefs, NewSampleDef(sampleDefJson.(map[string]interface{})))
	}
	// assume bclprocessor formatted fastqs for now
	for _, sampleDef := range sampleDefs {
		if defPaths, err := FastqPathsFromSampleDef(sampleDef); err == nil {
			allPaths = append(allPaths, defPaths...)
		} else {
			return []string{}, err
		}
	}
	return allPaths, nil
}

func PipelineFromInvocation(invocation *core.InvocationData) string {
	return invocation.Call
}
