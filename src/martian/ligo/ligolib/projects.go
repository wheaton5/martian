// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package ligolib

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

/*
 * This defines metadata for a specific metric
 */
type MetricDef struct {
	JSONPath string `json:"-"`

	/* These JSON parameters help display the metric */
	HumanName string `json:",omitempty"`
	Owner     string `json:",omitempty"`
	Type      string `json:",omitempty"`

	/* Warn is value is below Low or above High */
	Low  *float64 `json:",omitempty"`
	High *float64 `json:",omitempty"`

	/* Warn when value changes by more than AbsDiffAllow */
	AbsDiffAllow *float64 `json:",omitempty"`

	/* Warn when the percentile change is more then RelDiffAllow */
	RelDiffAllow *float64 `json:",omitempty"`
}

/*
 * A collection of metrics
 */
type Project struct {
	Metrics    map[string]*MetricDef
	Where      string
	SampleIDs  []string
	WhereAble  WhereAble `json:"-"`
	TargetSets []TargetSet
}

type TargetSet struct {
	SampleIDs []string
	Targets   map[string]*MetricDef
}

/*
 * Parse a CSV file (already loaded as a byte array) into a TargetSet array.
 * The format of the file is:
 *
 * JUNK,/metric/1/path,/metric/2/path,/metric/3/path
 * sampleid1, metric_1_low/metric_1_high, metric_2_low/metric_2_high, metric_3_low/metric_3_high
 * sampleid2, metric_1_low/metric_1_high, metric_2_low/metric_2_high, metric_3_low/metric_3_high
 * ...
 *
 * Any line that astarts with # will be ignored.
 */
func TargetsFromCSV(csv []byte) []TargetSet {

	lines := strings.Split(string(csv), "\n")

	start := 0

	/* Skip blank and comment lines at the start */
	for ; len(lines[start]) == 0 || lines[start][0] == '#'; start++ {
	}

	/* Grab the metric names from the first real line */
	metric_names := strings.Split(lines[start], ",")

	/* Fast-forward to the next line */
	start++

	ts_a := make([]TargetSet, 0, len(lines))

	/* metric_names[0] is nonsense. metrics_names[1] is the name of the first metric....
	 * l[0] is the sample_id. l[1] is the target for the first metric l[2] is the larget for the second metric...
	 */
	for _, l := range lines[start:len(lines)] {

		/* Skip blank and comment lines */
		if len(l) == 0 || l[0] == '#' {
			continue
		}

		/* Parse out the 'C' in CSV */
		sid_targets := strings.Split(l, (","))

		/* Allocate a targetset object for this row */
		ts_a = append(ts_a, TargetSet{})
		ts := &ts_a[len(ts_a)-1]

		/* Target set applies to just one sample id... whoever is in column 0 */

		ts.SampleIDs = sid_targets[0:1]

		/* Use the other columns to compute metricdefs for this target */
		ts.Targets = make(map[string]*MetricDef)

		for i := 1; i < len(sid_targets); i++ {
			var low_s, high_s float64

			fmt.Sscanf(sid_targets[i], "%f/%f", &low_s, &high_s)
			md := new(MetricDef)

			/* Here's the magic! */
			md.JSONPath = metric_names[i]

			/* NaN check for low*/
			if low_s == low_s {
				lptr := new(float64)
				*lptr = low_s
				md.Low = lptr
			}

			/* NaN check for high */
			if high_s == high_s {
				hptr := new(float64)
				*hptr = high_s
				md.High = hptr
			}
			ts.Targets[md.JSONPath] = md;
		}
	}
	log.Printf("LOADED TARGETS: %v", ts_a);
	return ts_a
}

/*
 * A cache of all projects
 */

type ProjectsCache struct {
	/* Projects loaded from disk (and checked in */
	Projects map[string]*Project
	BasePath string

	/* These are temporary "projects" that are only cached in memory that
	 * people can build in the UI.
	 */

	TempProjects     map[string]*Project
	TempProjectsPath string
	TempId           int
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
	Diff     bool
	DiffPerc float64

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

type MetricResultByPercentSorter []MetricResult

func (m MetricResultByPercentSorter) Len() int           { return len(m) }
func (m MetricResultByPercentSorter) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m MetricResultByPercentSorter) Less(i, j int) bool { return m[i].DiffPerc > m[j].DiffPerc }

/*
 * This is a kludge to handle newline characters in JSON strings and comments
 * in JSON.
 * A newline in a JSON string will be redacted. Any line starting with '#' will
 * also be ignored.
 */

func removeBadChars(in []byte) []byte {
	output := make([]byte, len(in))
	output_index := 0

	/* state keeps track of a trivial stage machine that we use to yank comments.
	 * 0: normal text
	 * 1: just got a NL
	 * 2: in a comment (NL followed by #)
	 */
	state := 0

	for _, c := range in {

		if state == 1 {
			if c == '#' {
				state = 2
			} else {
				state = 0
			}
		}

		if c == '\n' || c == '\r' {
			state = 1
		}

		if state == 0 {
			output[output_index] = c
			output_index++
		}
	}
	return (output[0:output_index])
}

/*
 * Load a new temporary project, and return a key to find that project
 * later.
 */
func (pc *ProjectsCache) NewTempProject(txt string) (string, error) {
	/* Make up a name for this project */
	pc.TempId++
	temp_project_name := fmt.Sprintf("_T%v", pc.TempId)

	var project Project

	/* Load and parse the JSON for it */
	err := json.Unmarshal(removeBadChars([]byte(txt)), &project)
	if err != nil {
		return "", err
	}

	/*
	 * Fix up a bunch of stuff (see LoadProject)
	 */
	for k, _ := range project.Metrics {
		project.Metrics[k].JSONPath = k
	}
	project.WhereAble = MergeWhereClauses(NewStringWhere(project.Where), NewListWhere("sampleid", project.SampleIDs))
	pc.TempProjects[temp_project_name] = &project

	return temp_project_name, nil
}

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

	err = json.Unmarshal(removeBadChars(file_contents), &project)

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

	/* Because we know path ends in .json */
	basename := path[0:len(path)-5]
	csvname := basename + ".csv"
	_, err = os.Stat(csvname)
	if err == nil {
		log.Printf("Loading target data from: %v", csvname)
		csvdata, err := ioutil.ReadFile(csvname)
		if err != nil {
			panic(err)
		}

		project.TargetSets = TargetsFromCSV(csvdata)
	}

	/* Merge any sample IDs that explicitly appear in any targets set with the base sample ID list */
	for _, sid := range project.TargetSets {
		project.SampleIDs = append(project.SampleIDs, sid.SampleIDs...)
	}

	/* Remove duplicate elements from project.SampleIDs */
	sort.Strings(project.SampleIDs)

	out_idx := 1
	for in_idx := 1; in_idx < len(project.SampleIDs); in_idx++ {
		if project.SampleIDs[out_idx-1] == project.SampleIDs[in_idx] {

		} else {
			project.SampleIDs[out_idx] = project.SampleIDs[in_idx]
			out_idx++
		}
	}

	project.SampleIDs = project.SampleIDs[0:out_idx]

	/* Now merge the where clauses */
	project.WhereAble = MergeWhereClauses(NewStringWhere(project.Where), NewListWhere("sampleid", project.SampleIDs))

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
		name := p.Name()

		/* only consider files that end in .json */
		if len(name) > 5 && (name[len(name)-5:len(name)]) == ".json" {
			mdt, err := LoadProject(pc.BasePath + "/" + p.Name())
			if mdt != nil {
				projects[p.Name()] = mdt
			} else {
				log.Printf("Failed to load project %v: %v", p.Name(), err)
			}
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

	pc.TempProjects = make(map[string]*Project)
	return pc
}

/*
 * Search a project by name.
 */
func (pc *ProjectsCache) Get(path string) *Project {
	project := pc.Projects[path]

	if project == nil {
		project = pc.TempProjects[path]
	}
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
 * This implements the project definition resolution algorithm.  The idea is
 * that we have a "generic" set of targets in Project.Metrics. However, for a
 * particular metric might be overrided for a particular sample set.  we try to
 * find a sample set that includes this sample_ida nd defines this metric, if
 * we do, we use it.  Otherwise, we fall back to the generic definition. If
 * that is missing, you get nil.
 *
 * This function returns (nil,nil) if the metric is justmissing, and (nil,err)
 * if yo uare a horrible person.
 */
func (p *Project) LookupMetricDef(json_path string, sample_id string) (*MetricDef, error) {

	/* Generic metric to use? */
	base := p.Metrics[json_path]

	/* Did we find a better one? */
	var got_one *MetricDef

	/* Check every target set */
	for _, ts := range p.TargetSets {
		found := false
		for _, s := range ts.SampleIDs {
			if s == sample_id {
				found = true
				break
			}
		}
		/* Does this target set include sample_id and does it define this metric? */
		if found && ts.Targets[json_path] != nil {
			if got_one == nil {
				got_one = ts.Targets[json_path]
			} else {
				/* Uh oh! This target is defined twice. Get upset about it */
				return nil, errors.New(
					fmt.Sprintf("Is nothing sacred? Sample id %v, metric %v has multiple targets. Run! Run from the demons of fate. Now, everything is ruined.",
						sample_id, json_path))
			}
		}
	}

	/*
	 * TODO: We want to be more clever here! We want to merge |got_one| and |base| here,
	 * using the old definitions for anything not explicitly overrided.
	 */
	if got_one != nil {
		return got_one, nil
	} else {
		return base, nil
	}
}

/*
 * Does a metric meet the specification?
 */
func CheckOK(m *MetricDef, value interface{}) bool {

	/* Stuff misisng a target gets a pass */
	if m == nil {
		return true
	}

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
 * Look up the correct targets for a given sample ID and then check the given metric
 * against those targets.
 */
func ResolveAndCheckOK(p *Project, metric_name string, sampleid string, value interface{}) bool {

	// Find the target to use
	m, err := p.LookupMetricDef(metric_name, sampleid)

	/* XXX This is wrong! We'll drop an important error on the floor here! */
	if err != nil {
		panic(err)
	}

	if m == nil {
		// Undefined metrics pass by default
		return true
	} else {
		return CheckOK(m, value)
	}
}

/*
 * Decide if two numbers are different given a metric definition.
 */
func CheckDiff(m *MetricDef, oldguy float64, newguy float64) bool {

	var AbsDiffAllow *float64
	var RelDiffAllow *float64

	if m != nil {
		AbsDiffAllow = m.AbsDiffAllow
		RelDiffAllow = m.RelDiffAllow
	}

	/* If an absolute different threshhold is specified, use it */
	if AbsDiffAllow != nil {
		if Abs(oldguy-newguy) > *AbsDiffAllow {
			return true
		}
	}

	var max_percent float64

	/* If a max relative difference (percentile) is specified use it.
	 * If nothing at all is specified then, assume a max difference of
	 * 1.0.
	 */
	if RelDiffAllow == nil {
		if AbsDiffAllow == nil {
			max_percent = 1.0
		} else {
			/* If something else was specified, and RedDiffAllow was not
			 * specified, we're done.
			 */
			return false
		}
	} else {
		max_percent = *RelDiffAllow
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
 * Append a string to a list, unless it is already in the list.
 */
func AugmentMetrics(metrics []string, newmetric string) []string {

	for _, m := range metrics {
		if m == newmetric {
			return metrics
		}
	}

	return append(metrics, newmetric)
}

/*
 * Compare absolutely every metric between two pipestances.
 * |project| the project to use to color passing/failing metrics
 * ida, idb the test report IDs of the projects
 * |where| a where clause applied to the selection of metrics. We can use this to
 * ignore certain summary reports (like _perf which is HUGE)
 */
func CompareAll(project *Project, db *CoreConnection, ida int, idb int, where WhereAble) ([]MetricResult, error) {

	/* Grab every single reported metric for IDa and IDb */
	a_mets, err := db.GrabAllMetricsRaw(where, ida)
	if err != nil {
		return nil, err
	}

	b_mets, err := db.GrabAllMetricsRaw(where, idb)
	if err != nil {
		return nil, err
	}

	/* grab teh basic metadata for ida and idb so that we can look up which
	 * target set to apply for metric that happen to be defined in the project
	 */
	basedata_a_i, err := db.GrabRecords(NewStringWhere(fmt.Sprintf("ID='%v'", ida)),
		"test_reports",
		ReportRecord{})

	if err != nil {
		return nil, err
	}

	basedata_b_i, err := db.GrabRecords(NewStringWhere(fmt.Sprintf("ID='%v'", idb)),
		"test_reports",
		ReportRecord{})

	if err != nil {
		return nil, err
	}

	basedata_a := (basedata_a_i.([]ReportRecord))[0]
	basedata_b := (basedata_b_i.([]ReportRecord))[0]

	metric_map := make(map[string]*MetricResult)

	/* Iterate over all of the metrics from IDA and IDB and place them in a huge.
	 * map that assocaites the metric name (Datum.Path) with the metric result info.
	 *
	 *... First do it from ida and fill in MetricResult.BaseVal
	 */
	for _, met := range a_mets {
		ptr := metric_map[met.Path]
		if ptr == nil {
			ptr = new(MetricResult)
			metric_map[met.Path] = ptr
		}

		ptr.JSONPath = met.Path
		ptr.BaseVal = met.Value
	}

	/*
	 * Now do it for idb and fill in MetricResult.NewVal
	 */
	for _, met := range b_mets {
		ptr := metric_map[met.Path]
		if ptr == nil {
			ptr = new(MetricResult)
			metric_map[met.Path] = ptr
		}
		ptr.JSONPath = met.Path
		ptr.NewVal = met.Value
	}

	/* Iterate over that map and copy it into an array. While we're duing this,
	 * use AssignMetricResultInfo to compute the
	 * percent different and set various pass/fail fields.
	 */
	metric_array := make([]MetricResult, 0, 0)
	for k := range metric_map {
		md := metric_map[k]
		AssignMetricResultInfo(project, md, basedata_a.SampleId, basedata_b.SampleId)
		metric_array = append(metric_array, *md)
	}

	/* Sort the metric_array by percent difference */
	sort.Sort((MetricResultByPercentSorter)(metric_array))

	return metric_array, nil

}

/*
 * This function tries to convert an interface to a float64.
 * The return value is the float64 value and an error flag. (true on success).
 * These rules apply:
 * If  i is an integer, cast it to a float and return that.
 * If  i is a string, try to strconv it to a float
 * If  I is a float64, just return it.
 *
 * BUT.... Never Ever return NaN. If we catch that i is really a NaN, treat it
 * like an error instead.
 */
func i2f(i interface{}) (float64, bool) {

	/* is I a float64? */
	f, ok := i.(float64)
	if ok {
		/* NaN check */
		if f != f {
			return 0, false
		}
		return f, true
	}

	/* is I an int? */
	fi, ok := i.(int)
	if ok {
		return float64(fi), true
	}

	/* is I a string that looks like a float? */
	s, ok := i.(string)
	if ok {
		f, err := strconv.ParseFloat(s, 64)
		/* error and NaN check? */
		if err == nil && f == f {
			return f, true
		}
	}

	return 0, false
}

func AssignMetricResultInfo(project *Project, mr *MetricResult, base_sid string, new_sid string) {

	bok := ResolveAndCheckOK(project, mr.JSONPath, base_sid, mr.BaseVal)
	nok := ResolveAndCheckOK(project, mr.JSONPath, new_sid, mr.NewVal)

	var diffperc float64
	var diff bool
	if mr.BaseVal == mr.NewVal {
		diffperc = 0
		diff = false
	} else {
		bfloat, bok1 := i2f(mr.BaseVal)
		nfloat, nok1 := i2f(mr.NewVal)
		if nok1 && bok1 {
			diff = CheckDiff(project.Metrics[mr.JSONPath], bfloat, nfloat)
			if nfloat == bfloat {
				diffperc = 0.0
			} else {
				diffperc = Abs((nfloat - bfloat) / (nfloat + bfloat) / 2)
			}
		} else {
			diff = true
			diffperc = 1000000000.0
		}
	}

	mr.NewOK = nok
	mr.OldOK = bok
	mr.Diff = !diff
	mr.DiffPerc = diffperc
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

	sort.Strings(list_of_metrics)

	/* We absolutely need to keep the sample ID so that we can
	 * resolve the right targets on a per-sample ID basis later.
	 */
	list_of_metrics = AugmentMetrics(list_of_metrics, "sampleid")

	/* Grab the metric for each pipestance */
	log.Printf("Comparing %v and %v", base, newguy)
	basedata, err := db.JSONExtract2(NewStringWhere(fmt.Sprintf("test_reports.id = %v", base)),
		list_of_metrics,
		"",
		nil,
		nil)

	if err != nil {
		return nil, err
	}

	newdata, err := db.JSONExtract2(NewStringWhere(fmt.Sprintf("test_reports.id = %v", newguy)),
		list_of_metrics,
		"",
		nil,
		nil)

	if err != nil {
		return nil, err
	}

	/* XXX This can blow up! kaboom! */
	new_sampleid := basedata[0]["sampleid"].(string)
	old_sampleid := newdata[0]["sampleid"].(string)
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

		mr.NewOK = ResolveAndCheckOK(m, one_metric, new_sampleid, newval)
		mr.OldOK = ResolveAndCheckOK(m, one_metric, old_sampleid, baseval)

		results = append(results, mr)
	}
	sort.Sort((MetricResultSorter)(results))
	return results, nil
}
