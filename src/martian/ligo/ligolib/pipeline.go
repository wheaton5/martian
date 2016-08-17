// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

/*
 * This implements functions for extracting various bits of metadata from a pipeline
 */
package ligolib

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

/*
 * Load JSON from a path
 */
func jsonload(path string) (map[string]interface{}, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("cannot load: %v %v", path, err)
		return nil, err
	}

	res := make(map[string]interface{})
	err = json.Unmarshal(contents, &res)

	if err != nil {
		log.Printf("can't parse json: %v", err)
		return nil, err
	}

	return res, nil

}

/*
 * Get the version of a pipestance by inspecting the _versions file.
 */
func GetPipestanceVersion(pipestance_path string) (string, error) {
	versions_file_path := pipestance_path + "/_versions"
	jsondata, err := jsonload(versions_file_path)

	if err != nil {
		return "", err
	}

	/* Is this always right? What about cellranger or supernova? */
	version := jsondata["pipelines"].(string)

	log.Printf("autodetect version of (%v): %v", pipestance_path, version)
	return version, nil

}

func FindStageNameFromPath(path string) string {
	path_array := strings.Split(path, "/")

	for i := len(path_array) - 1; i > 0; i-- {
		if (len(path_array[i]) > 3) && path_array[i][0:4] == "fork" {
			return path_array[i-1]
		}
	}
	return path_array[len(path_array)-1]
}

/*
 * Grab every summary.json file from a pipestance and upload it to the database.
 */
func InsertPipestanceSummaries(db *CoreConnection, test_report_id int, pipestance_path string) {

	filepath.Walk(pipestance_path+"/", func(path string, info os.FileInfo, e error) error {
		if len(info.Name()) > 4 && info.Name()[0:4] == "chnk" {
			/* Don't grab stuff that's inside a chunk. If we're in a chunk, forget
			 * about this entire subtree
			 */
			return filepath.SkipDir
		}
		if info.Name() == "summary.json" {
			/* Woohoo! found a summary file.*/
			log.Printf("Found summary at %v", path)

			stage := FindStageNameFromPath(path)

			/* Grab the file */
			contents, err := ioutil.ReadFile(path)
			if err != nil {
				panic("Can't read a file that I found from filepath.Walk")
			}

			/* Check that the file is valid JSON. Don't try to upload invalid
			 * JSON*/
			var data_as_json interface{}
			if json.Unmarshal(contents, &data_as_json) != nil {
				log.Printf("file %v is not JSON!!!", path)
			} else {
				r := ReportSummaryFile{0, test_report_id, string(contents), stage}
				_, err = db.InsertRecord("test_report_summaries", r)
				if err != nil {
					panic("Trouble uploading file to DB")
				}
			}
		}
		return nil
	})
}

/*
 * Grab a specific JSON file and upload that to the database.
 */
func InsertSummary(db *CoreConnection, test_report_id int, path string, name string) error {
	contents, err := ioutil.ReadFile(path)

	if err != nil {
		log.Printf("Failed to read file %v: %v", path, err)
		return err
	}

	var as_json interface{}
	err = json.Unmarshal(contents, &as_json)

	if err != nil {
		log.Printf("Failed to parse json file %v: %v", path, err)
		return err
	}

	report := ReportSummaryFile{0, test_report_id, string(contents), name}

	_, err = db.InsertRecord("test_report_summaries", report)
	if err != nil {
		panic(err)
	}

	log.Printf("Found summary at %v (to be called %v)", path, name)
	return nil
}

/*
 * Get the date that the pipestance finished.
 */
func GetPipestanceDate(path string) time.Time {

	file_info, err := os.Stat(path + "/_timestamp")

	if err != nil {
		panic(err)
	}

	return file_info.ModTime()
}

/*
 * Did the pipestance complete?
 */
func GetPipestanceSuccess(path string) bool {
	_, err := os.Stat(path + "/_finalstate")
	return err == nil
}
