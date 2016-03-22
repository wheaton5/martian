//
// File management interface.
//
package manager

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const SEQ_MISEQ string = "miseq"
const SEQ_HISEQ_2500 string = "hiseq_2500"
const SEQ_HISEQ_4000 string = "hiseq_4000"
const SEQ_NEXTSEQ string = "nextseq"
const SEQ_XTEN string = "xten"
const SEQ_UNKNOWN string = "unknown"

var SEQUENCER_PREFIXES = map[string]string {
	SEQ_MISEQ: "miseq",
	SEQ_HISEQ_2500: "hiseq",
	SEQ_HISEQ_4000: "4kseq",
	SEQ_NEXTSEQ: "nxseq",
	SEQ_XTEN: "xtseq",
}

//
// Return the id of the sequencer from the root path
//
func BclProcessorSequencer(rootPath string) string {
	for seq, prefix := range(SEQUENCER_PREFIXES) {
		if strings.Contains(rootPath, prefix) {
			return seq
		}
	}
	return SEQ_UNKNOWN
}

func BclProcessorWGSFastqPaths(rootPath string, sampleIndex string, lanes []int) []string {
	allFiles := []string{}
	allFiles = append(allFiles, BclProcessorFastqPaths(rootPath, "RA", sampleIndex, lanes, 2)...)
	allFiles = append(allFiles, BclProcessorFastqPaths(rootPath, "I1", sampleIndex, lanes, 2)...)
	return allFiles
}

func BclProcessorCRFastqPaths(rootPath string, sampleIndex string, lanes []int, bc_read_type string, si_read_type string) []string {
	allFiles := []string{}
	allFiles = append(allFiles, BclProcessorFastqPaths(rootPath, "RA", sampleIndex, lanes, 2)...)
	allFiles = append(allFiles, BclProcessorFastqPaths(rootPath, bc_read_type, sampleIndex, lanes, 2)...)
	allFiles = append(allFiles, BclProcessorFastqPaths(rootPath, si_read_type, sampleIndex, lanes, 2)...)
	return allFiles
}

//
// port of find_input_files_10x_preprocess in tenkit/fasta.py
//
func BclProcessorFastqPaths(rootPath string, readType string, sampleIndex string, lanes []int, maxNs int) []string {
	var siPattern string
	var files []string
	if sampleIndex != "*" {
		chars := strings.Split(sampleIndex, "")
		regexpTokens := make([]string, len(chars))
		for i, char := range chars {
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
		for _, lane := range lanes {
			filePattern := path.Join(rootPath,
				fmt.Sprintf("read-%s_si-%s_lane-%03d[_\\-]*.fastq.*", readType, siPattern, lane))
			globMatches, _ := filepath.Glob(filePattern)
			files = append(files, globMatches...)
		}
	}

	// filter files to remove those with > 2 Ns in the sample index
	goodFiles := []string{}
	goodRe := regexp.MustCompile(".*si-([A-Z]*)_")
	for _, file := range files {
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

func InputSizeTotal(paths []string) int64 {
	var size int64
	for _, path := range(paths) {
		if fileInfo, err := os.Stat(path); err == nil {
			size += fileInfo.Size()
		} else {
			fmt.Printf("Could not read file: %s", path)
		}
	}
	return size
}

// TODO: add bcldirect mode if necessary for HWM project

//
// Find all the bcl.gz files in the flow cell.
//
func SequencerBclPaths(flowCellPath string) []string {
	var baseCallFolder = filepath.Join(flowCellPath, "Data/Intensities/BaseCalls")
	// first star is lane, second star is cycle
	files, _ := filepath.Glob(filepath.Join(baseCallFolder, "*", "*", "*.bcl.gz"))
	return files
}
