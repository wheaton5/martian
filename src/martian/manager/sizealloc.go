package manager

import (
	"encoding/xml"
	"math"
	"os"
	"path"
	"strconv"
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

func GetAllocation(psid string, invocation Invocation) *PipestanceStorageAllocation {
	psname := PipelineFromInvocation(invocation)
	alloc := &PipestanceStorageAllocation{
		psid:   psid,
		psname: psname}

	var inputSize int64
	var weightedSize float64

	// get weighted size for bcl processor off the bat
	if strings.Contains(psname, "BCL_PROCESSOR") {
		var sequencer string
		runPath := invocation["run_path"].(string)
		numCycles := GetNumCycles(runPath)
		sequencer = BclProcessorSequencer(runPath)
		filePaths := SequencerBclPaths(runPath)
		inputSize = InputSizeTotal(filePaths)
		if sequencer == SEQ_MISEQ {
			// TODO update off of latest MARSOC figures
			// miseq BCL: 18x(sqrt read size, est.), stdev 1x
			weightedSize = 19.0 * float64(inputSize) / math.Sqrt(float64(numCycles))
		} else {
			// non-miseq BCL: 36x (sqrt read size), stdev 1.3
			weightedSize = 37.3 * float64(inputSize) / math.Sqrt(float64(numCycles))
		}
	} else {
		filePaths := FastqFilesFromInvocation(invocation)
		inputSize = InputSizeTotal(filePaths)
	}

	if strings.Contains(psname, "CELLRANGER") {
		// CELLRANGER_PD: 12.1x FASTQs, stdev 1.1x
		weightedSize = 13.2 * float64(inputSize)
	} else if strings.Contains(psname, "ANALYZER") {
		// ANALYZER_PD: 9.2x + 0.6 = 9.8
		weightedSize = 9.8 * float64(inputSize)
	} else if strings.Contains(psname, "PHASER_SVCALLER_EXOME") {
		GB_DOWNSAMPLE_RATIO := 9.4  // mean = 8.3 + 1.1
		NO_DOWNSAMPLE_RATIO := 11.6 // mean = 10.2 + 1.4
		// get downsample rate
		weightedSize = NO_DOWNSAMPLE_RATIO * float64(inputSize)
		downsample_iface := invocation["downsample"]
		if downsample_iface != nil {
			downsample := downsample_iface.(map[string]interface{})
			if gigabases, ok := downsample["gigabases"]; ok {
				// mean 8.3 + 1.1
				if gigabases, err := strconv.ParseFloat(gigabases.(string), 64); err == nil {
					weightedSize = GB_DOWNSAMPLE_RATIO * 1024 * 1024 * 1024 * gigabases
				}
			} else if subsample_rate, ok := downsample["subsample_rate"]; ok {
				if subsample_rate, err := strconv.ParseFloat(subsample_rate.(string), 64); err == nil {
					// mean = 10.2 + 1.4
					weightedSize *= subsample_rate
				}
			}
		}
	} else if strings.Contains(psname, "PHASER_SVCALLER") {
		// get downsample rate
		GB_DOWNSAMPLE_RATIO := 7.6
		NO_DOWNSAMPLE_RATIO := 11.6
		NO_DOWNSAMPLE_OFFSET := 30.0 * (1024 * 1024 * 1024)
		downsample_iface := invocation["downsample"]
		weightedSize = NO_DOWNSAMPLE_OFFSET + NO_DOWNSAMPLE_RATIO*float64(inputSize)
		if downsample_iface != nil {
			downsample := downsample_iface.(map[string]interface{})
			if gigabases, ok := downsample["gigabases"]; ok {
				// mean 6.5 + 1.1
				if gigabases, err := strconv.ParseFloat(gigabases.(string), 64); err == nil {
					weightedSize = GB_DOWNSAMPLE_RATIO * 1024 * 1024 * 1024 * gigabases
				}
			}
		}
	}

	alloc.inputSize = inputSize
	alloc.weightedSize = int64(weightedSize)
	return alloc
}
