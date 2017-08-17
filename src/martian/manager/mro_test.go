package manager

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

type DummySettings struct {
	ThreadsPerJob int `json:"threads_per_job"`
	MemGBPerJob   int `json:"memGB_per_job"`
}

// workaround for verifyJobManager which does
// a relative path search to get jobmanagers/config.json;
// need it to get Runtime to parse an MRO
func init() {
	dummyConfig := map[string]interface{}{
		"settings": &DummySettings{ThreadsPerJob: 1, MemGBPerJob: 6}}
	configJSON, _ := json.Marshal(dummyConfig)

	execName, _ := filepath.Abs(os.Args[0])
	// need to get ../jobmanagers from exec folder
	jobmgrFolder := filepath.Join(filepath.Dir(filepath.Dir(execName)), "jobmanagers")
	if err := os.MkdirAll(jobmgrFolder, 0755); err == nil {
		if ferr := ioutil.WriteFile(filepath.Join(jobmgrFolder, "config.json"), configJSON, 0644); ferr != nil {
			fmt.Println("Could not write config.json: %s", ferr.Error())
		}
	} else {
		fmt.Println("Could not create dummy jobmanager folder: %s", err.Error())
	}
}

func getTestMROPath(relPath string) string {
	absRoot, err := filepath.Abs("../test/manager/mro")
	if err == nil {
		return filepath.Join(absRoot, relPath)
	} else {
		return ""
	}
}

func TestFastqFilesFromInvocation(t *testing.T) {
	sourcePath := getTestMROPath("test_wgs.mro")
	mroPaths := []string{}
	//if value := os.Getenv("MROPATH"); len(value) > 0 {
	//	mroPaths = util.ParseMroPath(value)
	//}
	var paths []string
	if source, err := ioutil.ReadFile(sourcePath); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", sourcePath))
	} else {
		invocation := InvocationFromMRO(string(source), sourcePath, mroPaths)
		paths, _ = FastqFilesFromInvocation(invocation)
	}
	// RA+I1 only
	assert.Len(t, paths, 5, "WGS mode, oligo sample indices")

	sourcePath = getTestMROPath("test_wgs_si_set.mro")
	if source, err := ioutil.ReadFile(sourcePath); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", sourcePath))
	} else {
		invocation := InvocationFromMRO(string(source), sourcePath, mroPaths)
		paths, _ = FastqFilesFromInvocation(invocation)
	}
	assert.Len(t, paths, 5, "WGS mode, sample index set")
}

func TestPipelineFromInvocation(t *testing.T) {
	sourcePath := getTestMROPath("test_wgs.mro")
	mroPaths := []string{}

	if source, err := ioutil.ReadFile(sourcePath); err != nil {
		assert.Fail(t, fmt.Sprintf("Could not read file: %s", sourcePath))
	} else {
		invocation := InvocationFromMRO(string(source), sourcePath, mroPaths)
		pipeline := PipelineFromInvocation(invocation)
		assert.Equal(t, pipeline, "SAMPLE_DEF_TEST")
	}
}
