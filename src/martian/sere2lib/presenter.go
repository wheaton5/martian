// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package sere2lib;


type TrackedDataSource struct {
	JSONPath string
}

type PresentingData struct {
	Sourcedef *TrackedDataSource
	XSeries []string;
	YSeries []string;
}

type PresenterConfig struct {
	Sources []TrackedDataSource
}

type PresentingPage struct {
	AllData []PresentingData
}

func LoadConfig(path string) *PresenterConfig {



}



func (pc * PresenterConfig) Present(dataclass string, sample_id string, dbconn *CoreConnection) *PresentingPage {
	
	pd := opc.ExtractPresentationData("", dbconn);
	return &PresentingPage{pd}



}

func (pc * PresenterConfig) ExtractPresentationData(where string, dbconn *CoreConnection) []PresentingData{
	paths := []string{"SHA", "SampleId", "UserId"};

	for _, source := range pc.Sources {
		paths = append(paths, source.JSONPath);
	}

	db_res := dbconn.JSONExtract("test_reports", where, paths);

	pd:= make([]PresentingData, len(pc.Sources));

	for i := range pc.Sources {
		pd[i].Sourcedef = &pc.Sources[i];
		pd[i].XPath, PresentingData[i].YPath = Rotate(db_res, "SHA", pc.Sources[i].JSONPath);
	}
	return pd;
}


func Rotate(data []map[string]interface{}, x string, y string) ([]string, []string) {

	xout := make([]string, len(data));
	yout := make([]string, len(data))
	for i, element := range data {
		xout[i] = element[x];
		yout[i] = element[y];
	}
	return xout, yout;
}

