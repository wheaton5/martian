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