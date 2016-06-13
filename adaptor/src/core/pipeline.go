// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.
package core

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
)

type ProjectInfo struct {
	TopLevel        string
	Name            string
	SummaryJSONPath string
}

var ProjectDefs = []ProjectInfo{
	{"PHASER_SVCALLER_EXOME_PD", "longranger-exome", "PHASER_SVCALLER_EXOME_PD/SUMMARIZE_REPORTS_PD/fork0/files/summary.json"},
	{"PHASER_SVCALLER_PD", "longranger-exome", "PHASER_SVCALLER_PD/SUMMARIZE_REPORTS_PD/fork0/files/summary.json"},
}

func GuessProject(path string) *ProjectInfo {

	for i := 0; i < len(ProjectDefs); i++ {
		try := &ProjectDefs[i]
		_, err := os.Stat(path + "/" + try.TopLevel)
		if err == nil {
			return try
		}
	}
	return nil
}

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

func GetPipestanceVersion(pipestance_path string) (string, error) {
	vf := pipestance_path + "/_versions"
	jsondata, err := jsonload(vf)

	if err != nil {
		return "", err
	}

	version := jsondata["pipelines"].(string)

	log.Printf("autodetect version of (%v): %v", pipestance_path, version)
	return version, nil

}
