package manager

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

func TestGetTotalReadCount(t *testing.T) {
	runPath := getTestFilePath("sequencers/miseq002/HTESTBCXX")
	numCycles := GetNumCycles(runPath)
	assert.Equal(t, numCycles, 125)
}

func TestUnknownAlloc(t *testing.T) {
	mro := getTestFilePath("mro/test_unknown.mro")
	mroPaths := []string{}
	var invocation Invocation
	if source, err := ioutil.ReadFile(mro); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", mro))
	} else {
		invocation = InvocationFromMRO(string(source), mro, mroPaths)
	}
	alloc, err := GetAllocation("test_cellranger", invocation)
	assert.NotNil(t, err)
	assert.Nil(t, alloc)
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
	alloc, _ := GetAllocation("test_cellranger", invocation)
	assert.Equal(t, 6, alloc.inputSize)
	assert.Equal(t, 79, alloc.weightedSize)
}

func TestAnalyzerAlloc(t *testing.T) {
	mro := getTestFilePath("mro/test_analyzer.mro")
	mroPaths := []string{}
	var invocation Invocation
	if source, err := ioutil.ReadFile(mro); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", mro))
	} else {
		invocation = InvocationFromMRO(string(source), mro, mroPaths)
	}
	alloc, _ := GetAllocation("test_cellranger", invocation)
	assert.Equal(t, 6, alloc.inputSize)
	assert.Equal(t, 78, alloc.weightedSize)
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
	alloc, _ := GetAllocation("test_phaser_svcaller_exome", invocation)
	assert.Equal(t, 6, alloc.inputSize)
	expectedValue := 1024*1024*1024*9*11.7
	assert.Equal(t, int64(expectedValue), alloc.weightedSize)

	mro = getTestFilePath("mro/test_phaser_svcaller_exome_subsample_rate.mro")
	mroPaths = []string{}
	if source, err := ioutil.ReadFile(mro); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", mro))
	} else {
		invocation = InvocationFromMRO(string(source), mro, mroPaths)
	}
	alloc, _ = GetAllocation("test_phaser_svcaller_exome", invocation)
	assert.Equal(t, 6, alloc.inputSize)
	// 11.6 * 5
	assert.Equal(t, 73, alloc.weightedSize)

	mro = getTestFilePath("mro/test_phaser_svcaller_exome.mro")
	mroPaths = []string{}
	if source, err := ioutil.ReadFile(mro); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", mro))
	} else {
		invocation = InvocationFromMRO(string(source), mro, mroPaths)
	}
	alloc, _ = GetAllocation("test_phaser_svcaller_exome", invocation)
	assert.Equal(t, 6, alloc.inputSize)
	// 11.6 * 5
	assert.Equal(t, 87, alloc.weightedSize)
}

func TestPhaserSvCallerAlloc(t *testing.T) {
	mroPaths := []string{}
	var invocation Invocation
	mro := getTestFilePath("mro/test_phaser_svcaller.mro")
	if source, err := ioutil.ReadFile(mro); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", mro))
	} else {
		invocation = InvocationFromMRO(string(source), mro, mroPaths)
	}
	alloc, _ := GetAllocation("test_phaser_svcaller", invocation)
	expectedValue := 30.0*1024*1024*1024 + 14.5*6
	assert.Equal(t, int64(expectedValue), alloc.weightedSize)

	mro = getTestFilePath("mro/test_phaser_svcaller_downsample.mro")
	if source, err := ioutil.ReadFile(mro); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", mro))
	} else {
		invocation = InvocationFromMRO(string(source), mro, mroPaths)
	}
	alloc, _ = GetAllocation("test_phaser_svcaller", invocation)
	assert.Equal(t, 6, alloc.inputSize)
	expectedSize := 1024*1024*1024*10*9.5
	assert.Equal(t, int64(expectedSize), alloc.weightedSize)
}

func TestBclAlloc(t *testing.T) {
	var invocation Invocation
	mroPaths := []string{}
	mro := getTestFilePath("mro/test_bclprocessor_hiseq.mro")

	if source, err := ioutil.ReadFile(mro); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", mro))
	} else {
		invocation = InvocationFromMRO(string(source), mro, mroPaths)
	}
	alloc, _ := GetAllocation("test_bclprocessor", invocation)
	assert.Equal(t, 26, alloc.weightedSize)

	mro = getTestFilePath("mro/test_bclprocessor_miseq.mro")
	if source, err := ioutil.ReadFile(mro); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", mro))
	} else {
		invocation = InvocationFromMRO(string(source), mro, mroPaths)
	}
	alloc, _ = GetAllocation("test_bclprocessor", invocation)
	assert.Equal(t, 13, alloc.weightedSize)

}