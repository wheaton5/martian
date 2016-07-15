package manager

import (
	"encoding/xml"
	"martian/core"
	"math"
	"os"
	"path"
	"strings"
)

// TODO: this is a duplicate of marsoc/sequencers.go, but importing the
// marsoc package results in a circular import.  Perhaps figure out a way
// to move this into core read types.
type XMLFlowcellLayout struct {
	XMLName      xml.Name `xml:"FlowcellLayout"`
	LaneCount    int      `xml:"LaneCount,attr"`
	SurfaceCount int      `xml:"SurfaceCount,attr"`
	SwathCount   int      `xml:"SwathCount,attr"`
	TileCount    int      `xml:"TileCount,attr"`
}

type XMLRead struct {
	XMLName       xml.Name `xml:"Read"`
	Number        int      `xml:"Number,attr"`
	NumCycles     int      `xml:"NumCycles,attr"`
	IsIndexedRead string   `xml:"IsIndexedRead,attr"`
}

type XMLReads struct {
	XMLName xml.Name  `xml:"Reads"`
	Reads   []XMLRead `xml:"Read"`
}

type XMLRun struct {
	XMLName        xml.Name `xml:"Run"`
	Id             string   `xml:"Id,attr"`
	Number         int      `xml:"Number,attr"`
	Flowcell       string
	Instrument     string
	Date           string
	Reads          XMLReads          `xml:"Reads"`
	FlowcellLayout XMLFlowcellLayout `xml:"FlowcellLayout"`
}

type XMLRunInfo struct {
	XMLName xml.Name `xml:"RunInfo"`
	Run     XMLRun   `xml:"Run"`
}

// TODO: END REPEATED XML STRUCT

type PipestanceStorageAllocation struct {
	psid         string
	psname       string
	sequencer    string
	inputSize    int64
	weightedSize int64
}

func GetRunInfo(runPath string) *XMLRunInfo {
	var xmlRunInfo XMLRunInfo
	file, err := os.Open(path.Join(runPath, "RunInfo.xml"))
	if err != nil {
		return nil
	}
	defer file.Close()
	if err := xml.NewDecoder(file).Decode(&xmlRunInfo); err != nil {
		return nil
	}
	return &xmlRunInfo
}

func GetNumCycles(runPath string) int {
	runInfo := GetRunInfo(runPath)
	numCycles := 0
	if runInfo != nil {
		for _, read := range runInfo.Run.Reads.Reads {
			numCycles += read.NumCycles
		}
	}
	return numCycles
}

func GetAllocation(psid string, invocation Invocation) (*PipestanceStorageAllocation, error) {
	if invocation == nil {
		err := &core.PipestanceSizeError{psid}
		core.LogError(err, "storage", "getting allocation (nil, possibly malformed MRO): %s", psid)
		return nil, err
	}
	psname := PipelineFromInvocation(invocation)
	alloc := &PipestanceStorageAllocation{
		psid:   psid,
		psname: psname}

	invokeArgs := invocation["args"].(map[string]interface{})

	var inputSize int64
	var weightedSize float64

	// get weighted size for bcl processor off the bat
	if strings.Contains(psname, "BCL_PROCESSOR") {
		var sequencer string
		runPath := invokeArgs["run_path"].(string)
		numCycles := GetNumCycles(runPath)
		sequencer = BclProcessorSequencer(runPath)
		// TODO log on error
		filePaths, _ := BclPathsFromRunPath(runPath)
		inputSize = InputSizeTotal(filePaths)
		if sequencer == SEQ_MISEQ {
			// miseq BCL: 18x(sqrt read size, est.), stdev 1x
			weightedSize = 19.0 * float64(inputSize) / math.Sqrt(float64(numCycles))
		} else {
			// non-miseq BCL: 36x (sqrt read size), stdev 1.3
			weightedSize = 37.3 * float64(inputSize) / math.Sqrt(float64(numCycles))
		}

	} else if strings.Contains(psname, "SAMPLE_INDEX_QCER") {
		// effectively a noop, according to pryvkin
		weightedSize = 0
	} else {
		filePaths, err := FastqFilesFromInvocation(invocation)
		if err != nil {
			return nil, err
		}
		inputSize = InputSizeTotal(filePaths)

		if strings.Contains(psname, "CELLRANGER") {
			// CELLRANGER_PD: 12.1x FASTQs, stdev 1.1x
			weightedSize = 13.2 * float64(inputSize)
		} else if strings.Contains(psname, "ANALYZER") {
			// JM 3-29-2016: increase ratios by 33% to match observed
			// ANALYZER_PD: 9.2x + 0.6 = 9.8
			weightedSize = 13.0 * float64(inputSize)
		} else if strings.Contains(psname, "PHASER_SVCALLER_EXOME") {
			// JM 3-29-2016: increase ratios by 25% to handle observed delay in TRIM_READS vdrkill
			GB_DOWNSAMPLE_RATIO := 11.7 // mean = 8.3 + 1.1 (*1.25)
			NO_DOWNSAMPLE_RATIO := 14.5 // mean = 10.2 + 1.4 (*1.25)
			// get downsample rate
			weightedSize = NO_DOWNSAMPLE_RATIO * float64(inputSize)
			downsample_iface := invokeArgs["downsample"]
			if downsample_iface != nil {
				downsample := downsample_iface.(map[string]interface{})
				if gigabases, ok := downsample["gigabases"]; ok {
					// mean 8.3 + 1.1
					if gb, ok := gigabases.(int64); ok {
						weightedSize = GB_DOWNSAMPLE_RATIO * float64(1024*1024*1024*gb)
					} else if gb, ok := gigabases.(float64); ok {
						weightedSize = GB_DOWNSAMPLE_RATIO * float64(1024*1024*1024) * gb
					}

				} else if subsample_rate, ok := downsample["subsample_rate"]; ok {
					if sr, ok := subsample_rate.(float64); ok {
						weightedSize *= sr
					} else if sr, ok := subsample_rate.(int64); ok {
						weightedSize *= float64(sr)
					}
				}
			}
		} else if strings.Contains(psname, "PHASER_SVCALLER") {
			// get downsample rate
			// JM 3-29-2016: increase ratios by 25% to handle observed delay in TRIM_READS vdrkill
			GB_DOWNSAMPLE_RATIO := 9.5
			NO_DOWNSAMPLE_RATIO := 14.5
			NO_DOWNSAMPLE_OFFSET := 30.0 * float64(1024*1024*1024)
			downsample_iface := invokeArgs["downsample"]
			weightedSize = NO_DOWNSAMPLE_OFFSET + NO_DOWNSAMPLE_RATIO*float64(inputSize)
			if downsample_iface != nil {
				downsample := downsample_iface.(map[string]interface{})
				if gigabases, ok := downsample["gigabases"]; ok {
					if gb, ok := gigabases.(int64); ok {
						weightedSize = GB_DOWNSAMPLE_RATIO * float64(1024*1024*1024*gb)
					} else if gb, ok := gigabases.(float64); ok {
						weightedSize = GB_DOWNSAMPLE_RATIO * float64(1024*1024*1024) * gb
					}
				} else if subsample_rate, ok := downsample["subsample_rate"]; ok {
					if sr, ok := subsample_rate.(float64); ok {
						weightedSize *= sr
					} else if sr, ok := subsample_rate.(int64); ok {
						weightedSize *= float64(sr)
					}
				}
			}
		} else if strings.Contains(psname, "ASSEMBLER") {
			// get downsample rate
			// JM 3-29-2016: increase ratios by 25% to handle observed delay in TRIM_READS vdrkill
			GB_DOWNSAMPLE_RATIO := 9.5
			NO_DOWNSAMPLE_RATIO := 14.5
			NO_DOWNSAMPLE_OFFSET := 30.0 * float64(1024*1024*1024)
			downsample_iface := invokeArgs["downsample"]
			weightedSize = NO_DOWNSAMPLE_OFFSET + NO_DOWNSAMPLE_RATIO*float64(inputSize)
			if downsample_iface != nil {
				downsample := downsample_iface.(map[string]interface{})
				if gigabases, ok := downsample["gigabases"]; ok {
					if gb, ok := gigabases.(int64); ok {
						weightedSize = GB_DOWNSAMPLE_RATIO * float64(1024*1024*1024*gb)
					} else if gb, ok := gigabases.(float64); ok {
						weightedSize = GB_DOWNSAMPLE_RATIO * float64(1024*1024*1024) * gb
					}
				} else if subsample_rate, ok := downsample["subsample_rate"]; ok {
					if sr, ok := subsample_rate.(float64); ok {
						weightedSize *= sr
					} else if sr, ok := subsample_rate.(int64); ok {
						weightedSize *= float64(sr)
					}
				} else if nreads, ok := downsample["target_reads"]; ok {
					// TODO: evil harcoded read length 150 below
					if nr, ok := nreads.(int64); ok {
						weightedSize = GB_DOWNSAMPLE_RATIO * float64(150*nr)
					} else if nr, ok := nreads.(float64); ok {
						weightedSize = GB_DOWNSAMPLE_RATIO * float64(150) * nr
					}
				}
			}
		} else {
			return nil, &core.PipestanceSizeError{psid}
		}
	}

	alloc.inputSize = inputSize
	alloc.weightedSize = int64(weightedSize)
	return alloc, nil
}
