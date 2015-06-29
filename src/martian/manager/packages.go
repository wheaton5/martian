//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
package manager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"martian/core"
	"os"
	"path"
	"strings"
)

type PackageManager interface {
	GetPipestanceEnvironment(string, string, string) (string, string, map[string]string, error)
}

type Package struct {
	Name        string            `json:"name"`
	Target      string            `json:"target"`
	ArgshimPath string            `json:"argshim_path"`
	MroPath     string            `json:"mro_path"`
	MroVersion  string            `json:"mro_version"`
	State       string            `json:"state"`
	Envs        map[string]string `json:"envs"`
	Argshim     *ArgShim          `json:"-"`
}

type PackageJson struct {
	Name        string            `json:"name"`
	Target      string            `json:"target"`
	ArgshimPath string            `json:"argshim_path"`
	MroPath     string            `json:"mro_path"`
	Envs        []*PackageJsonEnv `json:"envs"`
}

type PackageJsonEnv struct {
	Value string `json:"value"`
	Type  string `json:"type"`
	Key   string `json:"key"`
}

func NewPackage(packagePath string, debug bool) *Package {
	self := &Package{}
	self.Name, self.Target, self.ArgshimPath, self.MroPath, self.Envs, _ = VerifyPackage(packagePath)
	self.MroVersion = core.GetMroVersion(self.MroPath)
	self.Argshim = NewArgShim(self.ArgshimPath, self.Envs, debug)
	return self
}

func VerifyPackage(packagePath string) (string, string, string, string, map[string]string, error) {
	packageFile := path.Join(packagePath, "marsoc.json")
	if _, err := os.Stat(packageFile); os.IsNotExist(err) {
		core.PrintInfo("package", "Package config file %s does not exist.", packageFile)
		return "", "", "", "", nil, err
	}
	bytes, _ := ioutil.ReadFile(packageFile)

	var packageJson *PackageJson
	if err := json.Unmarshal(bytes, &packageJson); err != nil {
		core.PrintInfo("package", "Package config file %s does not contain valid JSON.", packageFile)
		return "", "", "", "", nil, err
	}

	argshimPath := path.Join(packagePath, packageJson.ArgshimPath)
	if _, err := os.Stat(argshimPath); err != nil {
		core.PrintInfo("package", "Package argshim file %s does not exist.", argshimPath)
		return "", "", "", "", nil, err
	}

	mroPath := path.Join(packagePath, packageJson.MroPath)
	if _, err := os.Stat(mroPath); err != nil {
		core.PrintInfo("package", "Package mro path %s does not exist.", mroPath)
		return "", "", "", "", nil, err
	}

	name := packageJson.Name
	target := packageJson.Target

	envs := map[string]string{}
	for _, envJson := range packageJson.Envs {
		key, value := envJson.Key, envJson.Value
		switch envJson.Type {
		case "path":
			if !strings.HasPrefix(value, "/") {
				value = path.Join(packagePath, value)
			}
		case "path_prepend":
			if !strings.HasPrefix(value, "/") {
				value = path.Join(packagePath, value)
			}

			// Prepend value to current environment variable
			if prefix, ok := envs[key]; ok {
				value = value + ":" + prefix
			} else if prefix := os.Getenv(key); len(prefix) > 0 {
				value = value + ":" + prefix
			}
		case "string":
			break
		default:
			core.PrintInfo("package", "Unsupported env variable type %s.", envJson.Type)
			return "", "", "", "", nil, &core.MartianError{fmt.Sprintf(
				"Unsupported env variable type %s.", envJson.Type)}
		}
		envs[key] = value
	}

	return name, target, argshimPath, mroPath, envs, nil
}
