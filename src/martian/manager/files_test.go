package manager

import (
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func getTestFilePath(relPath string) string {
	absRoot, err := filepath.Abs("../test/manager")
	if err == nil {
		return filepath.Join(absRoot, relPath)
	} else {
		return ""
	}
}

func TestFilesBclProcessorFastqPaths(t *testing.T) {
	bclPath := getTestFilePath("bclprocessor")
	assert.NotEmpty(t, bclPath)

	paths := BclProcessorFastqPaths(bclPath, "RA", "AAAGCATA", nil, 2)
	assert.Len(t, paths, 3)

	lanes := []int{1,2}
	paths = BclProcessorFastqPaths(bclPath, "RA", "AAAGCATA", lanes, 2)
	assert.Len(t, paths, 2)

	paths = BclProcessorFastqPaths(bclPath, "RA", "AAAGCATA", nil, 0)
	assert.Len(t, paths, 2)

	paths = BclProcessorFastqPaths(bclPath, "I1", "AAAGCATA", nil, 2)
	assert.Len(t, paths, 2)

	paths = BclProcessorFastqPaths(bclPath, "I2", "AAAGCATA", nil, 2)
	assert.Len(t, paths, 1)

	paths = BclProcessorFastqPaths(bclPath, "RA", "AGAGCATA", nil, 2)
	assert.Len(t, paths, 1)

	paths = BclProcessorFastqPaths(bclPath, "RA", "AAGGCATA", nil, 2)
	assert.Empty(t, paths)

	paths = BclProcessorFastqPaths(bclPath, "RA", "*", nil, 0)
	assert.Len(t, paths, 4)
}

func TestFilesBclProcessorWGSFastqPaths(t *testing.T) {
	bclPath := getTestFilePath("bclprocessor")

	paths := BclProcessorWGSFastqPaths(bclPath, "AAAGCATA", nil)
	assert.Len(t, paths, 5)
}

func TestFilesSequencerBclPaths(t *testing.T) {
	flowcell := getTestFilePath("HTESTBCXX");
	assert.NotEmpty(t, flowcell)

	paths := SequencerBclPaths(flowcell)
	assert.Len(t, paths, 8)
}

func TestBclProcessorSequencer(t *testing.T) {
	seq := BclProcessorSequencer("/mnt/fleet/hiseq002a/160318_D00547_0652_BHKNNGBCXX")
	assert.Equal(t, seq, SEQ_HISEQ_2500)

	seq = BclProcessorSequencer("/mnt/fleet/4kseq001a/160318_ST-K00126_0164_AH77VLBBXX")
	assert.Equal(t, seq, SEQ_HISEQ_4000)

	seq = BclProcessorSequencer("/mnt/fleet/nxseq001a/160318_NB500915_0167_AHYMTVBGXX")
	assert.Equal(t, seq, SEQ_NEXTSEQ)

	seq = BclProcessorSequencer("/mnt/fleet/miseq001a/160316_M00308_0356_000000000-AN5A6")
	assert.Equal(t, seq, SEQ_MISEQ)

	seq = BclProcessorSequencer("/mnt/fleet/xtseqEXTa/160131_ST-E00314_0132_BHLCJTCCXX")
	assert.Equal(t, seq, SEQ_XTEN)

	seq = BclProcessorSequencer("/mnt/fleet/iontorrent/hypothetical")
	assert.Equal(t, seq, SEQ_UNKNOWN)
}