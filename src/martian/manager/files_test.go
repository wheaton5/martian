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

	paths := BclProcessorFastqPaths(bclPath, "RA", "AAAGCGTA", nil, 2)
	assert.Len(t, paths, 3)

	lanes := []string{"1","2"}
	paths = BclProcessorFastqPaths(bclPath, "RA", "AAAGCGTA", lanes, 2)
	assert.Len(t, paths, 2)

	paths = BclProcessorFastqPaths(bclPath, "RA", "AAAGCGTA", nil, 0)
	assert.Len(t, paths, 2)

	paths = BclProcessorFastqPaths(bclPath, "I1", "AAAGCGTA", nil, 2)
	assert.Len(t, paths, 2)

	paths = BclProcessorFastqPaths(bclPath, "I2", "AAAGCGTA", nil, 2)
	assert.Len(t, paths, 1)

	paths = BclProcessorFastqPaths(bclPath, "RA", "AGAGCGTA", nil, 2)
	assert.Len(t, paths, 1)

	paths = BclProcessorFastqPaths(bclPath, "RA", "AAGGCGTA", nil, 2)
	assert.Empty(t, paths)

	paths = BclProcessorFastqPaths(bclPath, "RA", "*", nil, 0)
	assert.Len(t, paths, 4)
}

func TestFilesSequencerBclPaths(t *testing.T) {
	flowcell := getTestFilePath("HTESTBCXX");
	assert.NotEmpty(t, flowcell)

	paths := SequencerBclPaths(flowcell)
	assert.Len(t, paths, 8)
}