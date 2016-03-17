//
// File management interface.
//
package manager

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//
// port of find_input_files_10x_preprocess in tenkit/fasta.py
//
func BclProcessorFastqPaths(rootPath string, readType string, sampleIndex string, lanes []string, maxNs int) []string {
	var siPattern string
	var files []string
	var laneInts = make([]int, len(lanes))
	for idx, lane := range(lanes) {
		laneInt, err := strconv.Atoi(lane)
		if err != nil {
			panic(fmt.Sprintf("Unrecognized lane format: %s", lane))
		}
		laneInts[idx] = laneInt
	}
	if sampleIndex != "*" {
		chars := strings.Split(sampleIndex, "")
		regexpTokens := make([]string, len(chars))
		for i, char := range(chars) {
			regexpTokens[i] = fmt.Sprintf("[%sN]", char)
		}
		siPattern = strings.Join(regexpTokens, "")
	} else {
		siPattern = "*"
		maxNs = 100
	}

	if lanes == nil || len(lanes) == 0 {
		filePattern := path.Join(rootPath, fmt.Sprintf("read-%s_si-%s_*.fastq*", readType, siPattern))
		files, _ = filepath.Glob(filePattern)
	} else {
		for _, lane := range(laneInts) {
			filePattern := path.Join(rootPath,
				fmt.Sprintf("read-%s_si-%s_lane-%03d[_\\-]*.fastq.*", readType, siPattern, lane))
			globMatches, _ := filepath.Glob(filePattern)
			files = append(files, globMatches...)
		}
	}

	// filter files to remove those with > 2 Ns in the sample index
	goodFiles := []string{}
	goodRe := regexp.MustCompile(".*si-([A-Z]*)_")
	for _, file := range(files) {
		submatches := goodRe.FindStringSubmatch(file)
		if submatches != nil {
			matchSI := submatches[1]
			numNs := strings.Count(matchSI, "N")
			if numNs <= maxNs {
				goodFiles = append(goodFiles, file)
			}
		}
	}
	sort.Strings(goodFiles)
	return goodFiles
}

// TODO: add bcldirect mode if necessary for HWM project
