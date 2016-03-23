package manager

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

func TestGetTotalReadCount(t *testing.T) {
	runPath := getTestFilePath("HTESTBCXX")
	numCycles := GetNumCycles(runPath)
	assert.Equal(t, numCycles, 125)
}

func TestCellRangerAlloc(t *testing.T) {
	mro := getTestFilePath("mro/test_cellranger.mro")
	mroPaths := []string{}
	var invocation Invocation
	if source, err := ioutil.ReadFile(mro); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", mro))
	} else {
		invocation = InvocationFromMRO(string(source), mro, mroPaths)
	}
	alloc := GetAllocation("test_cellranger", invocation)
	assert.Equal(t, 6, alloc.inputSize)
	assert.Equal(t, 79, alloc.weightedSize)
}

func TestPhaserSvCallerExomeAlloc(t *testing.T) {
	mro := getTestFilePath("mro/test_phaser_svcaller_exome_downsample.mro")
	mroPaths := []string{}
	var invocation Invocation
	if source, err := ioutil.ReadFile(mro); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", mro))
	} else {
		invocation = InvocationFromMRO(string(source), mro, mroPaths)
	}
	alloc := GetAllocation("test_phaser_svcaller_exome", invocation)
	assert.Equal(t, 6, alloc.inputSize)
	expectedValue := 1024*1024*1024*9*9.4
	assert.Equal(t, int64(expectedValue), alloc.weightedSize)

	mro = getTestFilePath("mro/test_phaser_svcaller_exome_subsample_rate.mro")
	mroPaths = []string{}
	if source, err := ioutil.ReadFile(mro); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", mro))
	} else {
		invocation = InvocationFromMRO(string(source), mro, mroPaths)
	}
	alloc = GetAllocation("test_phaser_svcaller_exome", invocation)
	assert.Equal(t, 6, alloc.inputSize)
	// 11.6 * 5
	assert.Equal(t, 58, alloc.weightedSize)

	mro = getTestFilePath("mro/test_phaser_svcaller_exome.mro")
	mroPaths = []string{}
	if source, err := ioutil.ReadFile(mro); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", mro))
	} else {
		invocation = InvocationFromMRO(string(source), mro, mroPaths)
	}
	alloc = GetAllocation("test_phaser_svcaller_exome", invocation)
	assert.Equal(t, 6, alloc.inputSize)
	// 11.6 * 5
	assert.Equal(t, 69, alloc.weightedSize)
}

func TestPhaserSvCallerAlloc(t *testing.T) {
	mro := getTestFilePath("mro/test_phaser_svcaller.mro")
	mroPaths := []string{}
	var invocation Invocation
	if source, err := ioutil.ReadFile(mro); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", mro))
	} else {
		invocation = InvocationFromMRO(string(source), mro, mroPaths)
	}
	alloc := GetAllocation("test_phaser_svcaller", invocation)
	expectedValue := 30.0*1024*1024*1024 + 11.6*6
	assert.Equal(t, int64(expectedValue), alloc.weightedSize)

	mro = getTestFilePath("mro/test_phaser_svcaller_downsample.mro")
	mroPaths = []string{}
	if source, err := ioutil.ReadFile(mro); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", mro))
	} else {
		invocation = InvocationFromMRO(string(source), mro, mroPaths)
	}
	alloc = GetAllocation("test_phaser_svcaller", invocation)
	assert.Equal(t, 6, alloc.inputSize)
	expectedSize := 1024*1024*1024*10*7.6
	assert.Equal(t, int64(expectedSize), alloc.weightedSize)

}