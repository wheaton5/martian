// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package ligolib

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"sort"
)

/*
 * This defines metadata for a specific metric
 */
type MetricDef struct {
	JSONPath string

	/* These JSON parameters help display the metric */
	HumanName string
	Owner     string
	Type      string

	/* Warn is value is below Low or above High */
	Low  *float64
	High *float64

	/* Warn when value changes by more than AbsDiffAllow */
	AbsDiffAllow *float64

	/* Warn when the percentile change is more then RelDiffAllow */
	RelDiffAllow *float64
}

/*
 * A collection of metrics
 */
type Project struct {
	Metrics   map[string]*MetricDef
	Where     string
	WhereAble WhereAble
}

type ProjectsCache struct {
	Projects map[string]*Project
	BasePath string
}

/*
 * The result of comparing a metric frmo two pipestances
 */
type MetricResult struct {
	/* The old and new values */
	BaseVal interface{}
	NewVal  interface{}

	/* OK is true iff both values were successfully extracted.*/
	OK bool

	/* Are the values different (according to Def)*/
	Diff bool

	NewOK bool
	OldOK bool

	/* The definition of this metric */
	//Def *MetricDef

	HumanName string
	JSONPath  string
}

/*
 * How to order metric results in a stable way. (Just use JSON path)
 */
type MetricResultSorter []MetricResult

func (m MetricResultSorter) Len() int           { return len(m) }
func (m MetricResultSorter) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m MetricResultSorter) Less(i, j int) bool { return m[i].JSONPath < m[j].JSONPath }

/*
 * Load a metrics file from disk and return a Project structure that
 * describes the listed metrics.
 * The loads a file in the prescribed JSON format and then munges the result.
 */
func LoadProject(path string) (*Project, error) {

	/* Load file and parse JSON */
	file_contents, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, err
	}

	var project Project

	err = json.Unmarshal(file_contents, &project)

	if err != nil {
		return nil, err
	}

	/*
	 * Munge the result so that metricdef also knows the path to the metric
	 * (which is the key in the map that it is in
	 */
	for k, _ := range project.Metrics {
		project.Metrics[k].JSONPath = k
	}

	log.Printf("Loading metric from %v: %v (%v)", path, len(project.Metrics), project.Where)
	project.WhereAble = NewStringWhere(project.Where)
	return &project, nil
}

/*
 * Reload all project files into the projects cache.
 */
func (pc *ProjectsCache) Reload() error {

	paths, err := ioutil.ReadDir(pc.BasePath)

	if err != nil {
		panic(err)
	}

	projects := make(map[string]*Project)

	for _, p := range paths {
		/* Error handling here totally wrong XXX*/
		mdt, err := LoadProject(pc.BasePath + "/" + p.Name())
		if mdt != nil {
			projects[p.Name()] = mdt
		} else {
			log.Printf("Failed to load project %v: %v", p.Name(), err)
		}

	}

	pc.Projects = projects
	return nil
}

/*
 * Load all of the projects out of a directory and return the
 * projects cache object for them.
 */
func LoadAllProjects(basepath string) *ProjectsCache {
	pc := new(ProjectsCache)
	pc.BasePath = basepath
	pc.Reload()
	return pc
}

/*
 * Search a project by name.
 */
func (pc *ProjectsCache) Get(path string) *Project {
	project := pc.Projects[path]
	return project
}

/*
 * This all of the projects that we know of.
 */
func (pc *ProjectsCache) List() []string {

	plist := []string{}

	for k, _ := range pc.Projects {
		plist = append(plist, k)
	}
	return plist
}

func Abs(x float64) float64 {
	if x < 0 {
		return -x
	} else {
		return x
	}
}

/*
 * Does a metric meet the specification?
 */
func CheckOK(m *MetricDef, value interface{}) bool {

	asfloat, ok := value.(float64)

	/* No specification. Metric auto-passes.
	 */
	if m.High == nil && m.Low == nil {
		return true
	}

	/* Specification but no metric. Metric auto-fails */
	if !ok {
		return false
	}

	/* If the new value is outside of an prescribed range, we claim it
	 * is different (Regardless of the old value).
	 */
	if m.High != nil && asfloat > *m.High {
		return false
	}
	if m.Low != nil && asfloat < *m.Low {
		return false
	}

	return true

}

/*
 * Decide if two numbers are different given a metric definition.
 */
func CheckDiff(m *MetricDef, oldguy float64, newguy float64) bool {

	/* If an absolute different threshhold is specified, use it */
	if m.AbsDiffAllow != nil {
		if Abs(oldguy-newguy) > *m.AbsDiffAllow {
			return true
		}
	}

	var max_percent float64

	/* If a max relative difference (percentile) is specified use it.
	 * If nothing at all is specified then, assume a max difference of
	 * 1.0.
	 */
	if m.RelDiffAllow == nil {
		if m.AbsDiffAllow == nil {
			max_percent = 1.0
		} else {
			/* If something else was specified, and RedDiffAllow was not
			 * specified, we're done.
			 */
			return false
		}
	} else {
		max_percent = *m.RelDiffAllow
	}

	/* Handle division by zero: if oldguy==newguy there is no difference
	 * even if oldguy is 0.  Otherwise, if oldguy==0 and newguy!=0, there is
	 * a difference.
	 */
	if newguy == oldguy {
		return false
	}

	if oldguy == 0.0 {
		return true
	}

	if Abs((newguy-oldguy)/oldguy) > max_percent/100.0 {
		return true
	}

	return false
}

/*
 * Compare two pipestance invocations, specified by pipestance invocation ID.
 */
func Compare2(db *CoreConnection, m *Project, base int, newguy int) ([]MetricResult, error) {

	/* Flatten the list of metrics */
	list_of_metrics := make([]string, 0, len(m.Metrics))
	for metric_name, _ := range m.Metrics {
		list_of_metrics = append(list_of_metrics, metric_name)
	}

	/* Grab the metric for each pipestance */
	log.Printf("Comparing %v and %v", base, newguy)
	basedata, err := db.JSONExtract2(NewStringWhere(fmt.Sprintf("test_reports.id = %v", base)),
		list_of_metrics,
		"")

	if err != nil {
		return nil, err
	}

	newdata, err := db.JSONExtract2(NewStringWhere(fmt.Sprintf("test_reports.id = %v", newguy)),
		list_of_metrics,
		"")

	if err != nil {
		return nil, err
	}

	results := make([]MetricResult, 0, 0)

	/* Iterate over all metric definitions and compare the respective metrics */
	for _, one_metric := range list_of_metrics {
		newval := basedata[0][one_metric]
		baseval := newdata[0][one_metric]

		var mr MetricResult
		//mr.Def = (m.Metrics[one_metric])
		mr.HumanName = m.Metrics[one_metric].HumanName
		mr.JSONPath = m.Metrics[one_metric].JSONPath

		/* We expect all values that we compare to be floats */
		newfloat, ok1 := newval.(float64)
		basefloat, ok2 := baseval.(float64)
		mr.BaseVal = baseval
		mr.NewVal = newval

		if ok1 && ok2 {
			/* If we got the data, then compare them */
			mr.Diff = !CheckDiff((m.Metrics[one_metric]), newfloat, basefloat)
			mr.OK = true
		} else {
			mr.Diff = reflect.DeepEqual(newval, baseval)
			/* Something went wrong (missing metric? Not a float64?) */
			log.Printf("Trouble at %v %v (%v %v)", newval, baseval, ok1, ok2)
			mr.OK = false
		}

		mr.NewOK = CheckOK(m.Metrics[one_metric], newval)
		mr.OldOK = CheckOK(m.Metrics[one_metric], baseval)

		results = append(results, mr)
	}
	sort.Sort((MetricResultSorter)(results))
	return results, nil
}
