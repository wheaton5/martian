//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
package manager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"martian/util"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const versionParts = 3

type PackageManager interface {
	GetPipestanceEnvironment(string, string, string) ([]string, string, string, map[string]string, error)
	GetPackageEnvironment(string) ([]string, string, string, map[string]string, error)
}

type Package struct {
	Name        string            `json:"name"`
	Target      string            `json:"target"`
	BuildDate   string            `json:"build_date"`
	ArgshimPath string            `json:"argshim_path"`
	MroPaths    []string          `json:"mro_paths"`
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
	BuildDate   string            `json:"build_date"`
	Envs        []*PackageJsonEnv `json:"envs"`
}

type PackageJsonEnv struct {
	Value    string `json:"value"`
	Type     string `json:"type"`
	Key      string `json:"key"`
	Wildcard bool   `json:"wildcard,omitempty"`
}

func NewPackage(packagePath string, debug bool) *Package {
	self := &Package{}
	self.Name, self.Target, self.BuildDate, self.ArgshimPath, self.MroPaths, self.Envs, _ = VerifyPackage(packagePath)
	self.MroVersion, _ = util.GetMroVersion(self.MroPaths)
	self.Argshim = NewArgShim(self.ArgshimPath, self.Envs, debug)
	return self
}

func (self *Package) IsDirty() bool {
	mroVersion := strings.TrimPrefix(self.MroVersion, self.Name+"-")
	util.PrintInfo("package", "%s mroVersion: %s", self.Name, mroVersion)
	parts := strings.Split(mroVersion, ".")

	if len(parts) != versionParts {
		return true
	}

	for _, part := range parts {
		if _, err := strconv.Atoi(part); err != nil {
			return true
		}
	}

	return false
}

func (self *Package) RestartArgShim() {
	self.Argshim.Restart()
}

func VerifyPackage(packagePath string) (string, string, string, string, []string, map[string]string, error) {
	packageFile := path.Join(packagePath, "marsoc.json")
	if _, err := os.Stat(packageFile); os.IsNotExist(err) {
		util.PrintInfo("package", "Package config file %s does not exist.", packageFile)
		return "", "", "", "", nil, nil, err
	}
	bytes, _ := ioutil.ReadFile(packageFile)

	var packageJson *PackageJson
	if err := json.Unmarshal(bytes, &packageJson); err != nil {
		util.PrintInfo("package", "Package config file %s does not contain valid JSON.", packageFile)
		return "", "", "", "", nil, nil, err
	}

	argshimPath := path.Join(packagePath, packageJson.ArgshimPath)
	if _, err := os.Stat(argshimPath); err != nil {
		util.PrintInfo("package", "Package argshim file %s does not exist.", argshimPath)
		return "", "", "", "", nil, nil, err
	}

	mroPaths := []string{}
	for _, mroPath := range util.ParseMroPath(packageJson.MroPath) {
		mroPath := path.Join(packagePath, mroPath)
		if _, err := os.Stat(mroPath); err != nil {
			util.PrintInfo("package", "Package mro path %s does not exist.", mroPath)
			return "", "", "", "", nil, nil, err
		}
		mroPaths = append(mroPaths, mroPath)
	}

	name := packageJson.Name
	target := packageJson.Target
	buildDate := packageJson.BuildDate

	envs := map[string]string{}
	for _, envJson := range packageJson.Envs {
		key, value := envJson.Key, envJson.Value
		switch envJson.Type {
		case "path":
			if !path.IsAbs(value) {
				value = path.Join(packagePath, value)
			}
		case "path_prepend":
			if !path.IsAbs(value) {
				value = path.Join(packagePath, value)
			}
			if envJson.Wildcard {
				matches, err := filepath.Glob(value)
				if err != nil {
					util.PrintInfo("package", "Could not expand path prepend wildcard %s.", value)
					return "", "", "", "", nil, nil, err
				}
				if len(matches) == 0 {
					util.PrintInfo("package", "%s did not find any files.", value)
				} else {
					if len(matches) > 1 {
						util.PrintInfo("package", "%s found %d matches.  Using %s.",
							value, len(matches), matches[len(matches)-1])
					}
					value = matches[len(matches)-1]
				}
			}
			// Prepend value to current environment variable
			if prefix, ok := envs[key]; ok {
				value = value + ":" + prefix
			} else if prefix := os.Getenv(key); len(prefix) > 0 {
				value = value + ":" + prefix
			}
		case "setaside":
			envs["_TENX_"+key] = os.Getenv(key)
		case "string":
			break
		default:
			util.PrintInfo("package", "Unsupported env variable type %s.", envJson.Type)
			return "", "", "", "", nil, nil, &util.MartianError{fmt.Sprintf(
				"Unsupported env variable type %s.", envJson.Type)}
		}
		envs[key] = value
	}

	return name, target, buildDate, argshimPath, mroPaths, envs, nil
}

func GoRefreshPackageVersions(packages []*Package, mutex *sync.Mutex) {
	go func() {
		for {
			for _, p := range packages {
				mroVersion, err := util.GetMroVersion(p.MroPaths)
				if err != nil {
					util.LogError(err, "package", "Failed to get package %s version", p.Name)
					continue
				}

				if p.MroVersion != mroVersion {
					p.RestartArgShim()
					util.LogInfo("package", "Restarted package %s argshim for version %s",
						p.Name, mroVersion)
				}

				mutex.Lock()
				p.MroVersion = mroVersion
				mutex.Unlock()
			}

			time.Sleep(time.Minute * time.Duration(5))
		}
	}()
}
