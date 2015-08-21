// Copyright 2015 Unknwon
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package models

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/Unknwon/cae/zip"
	"github.com/Unknwon/com"
	"github.com/Unknwon/log"

	"github.com/Unknwon/cgx/modules/setting"
)

func init() {
	zip.Verbose = false

	go func() {
		for e := range buildQueue {
			e.Build()
		}
	}()
}

// Event represents a build event.
type Event struct {
	ID        int64 `xorm:"pk autoincr"`
	Ref       string
	IsSucceed bool
	Error     string    `xorm:"TEXT"`
	Created   time.Time `xorm:"CREATED"`
	Updated   time.Time `xorm:"UPDATED"`
}

func (e *Event) setError(errmsg string) {
	e.Error = errmsg
	if _, err := x.Id(e.ID).AllCols().Update(e); err != nil {
		log.Error("Event.setError[%d]: %v", e.ID, err)
	}
}

var buildQueue = make(chan *Event, 100)

func (e *Event) packTarget(srcPath, name, destDir string, target setting.Target) (err error) {
	binName := name
	if target.GOOS == "windows" {
		binName += ".exe"
	}

	if err = os.MkdirAll(destDir, os.ModePerm); err != nil {
		return err
	}

	targetPath := path.Join(destDir, name+"_"+e.targetString(target))
	log.Debug("Packing target to: %s", targetPath)

	if err = os.RemoveAll(targetPath); err != nil {
		return err
	}

	zipPath := targetPath + ".zip"
	packPath := path.Join(targetPath, name)
	if err = os.MkdirAll(packPath, os.ModePerm); err != nil {
		return err
	}

	if err = os.Rename(path.Join(srcPath, binName), path.Join(packPath, binName)); err != nil {
		return err
	}

	// Pack resources.
	for _, resName := range setting.Resources {
		os.Rename(path.Join(srcPath, resName), path.Join(packPath, resName))
	}

	if err = zip.PackTo(targetPath, zipPath); err != nil {
		return err
	}

	os.RemoveAll(targetPath)
	return nil
}

func (e *Event) targetString(target setting.Target) string {
	str := fmt.Sprintf("%s_%s_%s", e.Ref, target.GOOS, target.GOARCH)
	if len(target.GOARM) > 0 {
		str += "_arm" + com.ToStr(target.GOARM)
	}
	return str
}

func (e *Event) Build() {
	defer func() {
		log.Debug("Build finished: %s", e.Ref)
	}()

	// Fetch archive for reference.
	archiveURL := com.Expand(setting.Repository.ArchiveURL, map[string]string{"ref": e.Ref})
	localPath := path.Join(setting.GOPATHSrc, path.Dir(setting.Repository.ImportPath))
	name := path.Base(setting.Repository.ImportPath)
	fullLocalPath := path.Join(localPath, name)
	zipPath := path.Join(localPath, name) + ".zip"

	defer os.RemoveAll(fullLocalPath)
	defer os.RemoveAll(zipPath)

	log.Debug("Fetching archive: %s", archiveURL)
	if err := com.HttpGetToFile(http.DefaultClient, archiveURL, nil, zipPath); err != nil {
		e.setError(fmt.Sprintf("Event.Build.HttpGetToFile: %v", err))
		return
	}

	// Start building targets.
	for _, target := range setting.Targets {
		os.RemoveAll(fullLocalPath)

		if err := zip.ExtractTo(zipPath, localPath); err != nil {
			e.setError(fmt.Sprintf("Event.Build.ExtractTo: %v", err))
			return
		}

		// Rename directory and move files to designed path.
		dir, err := os.Open(localPath)
		if err != nil {
			e.setError(fmt.Sprintf("Event.Build.(open local path): %v", err))
			return
		}
		defer dir.Close()

		dirs, err := dir.Readdir(0)
		if err != nil {
			e.setError(fmt.Sprintf("Event.Build.(read local path info): %v", err))
			return
		}

		dirName := ""
		for _, d := range dirs {
			if !strings.HasPrefix(d.Name(), name) || dirName == name {
				continue
			}
			dirName = d.Name()
			break
		}
		if len(dirName) == 0 {
			e.setError(fmt.Sprintf("Event.Build.(read local path info): local path does not contain expected file"))
			return
		}
		// fmt.Println(dirName)

		srcPath := path.Join(localPath, name)
		if err = os.Rename(path.Join(localPath, dirName), srcPath); err != nil {
			e.setError(fmt.Sprintf("Event.Build.Rename: %v", err))
			return
		}

		envs := append([]string{
			"GOPATH=" + setting.GOPATH,
			"GOOS=" + target.GOOS,
			"GOARCH=" + target.GOARCH,
			"GOARM=" + target.GOARM,
		}, os.Environ()...)
		tags := setting.Cfg.Section("tags." + target.GOOS).Key("TAGS").MustString("")

		bufOut := new(bytes.Buffer)
		bufErr := new(bytes.Buffer)

		log.Debug("Getting dependencies: %s", e.targetString(target))
		cmd := exec.Command("go", "get", "-v", "-tags", tags)
		cmd.Env = envs

		cmd.Dir = srcPath
		cmd.Stdout = bufOut
		cmd.Stderr = bufErr

		if err = cmd.Run(); err != nil {
			fmt.Println(bufOut.String(), bufErr.String())
			e.setError(fmt.Sprintf("Event.Build.(get dependencies): %s", bufErr.String()))
			return
		}
		bufOut.Reset()

		log.Debug("Building target: %s", e.targetString(target))
		cmd = exec.Command("go", "build", "-v", "-tags", tags)
		cmd.Env = envs

		cmd.Dir = srcPath
		cmd.Stdout = bufOut
		cmd.Stderr = bufErr

		if err = cmd.Run(); err != nil {
			fmt.Println(bufOut.String(), bufErr.String())
			e.setError(fmt.Sprintf("Event.Build.(build target): %s", bufErr.String()))
			return
		}
		bufOut.Reset()

		if err = e.packTarget(srcPath, name, setting.ArchivePath, target); err != nil {
			e.setError(fmt.Sprintf("Event.Build.packTarget: %v", err))
			return
		}
	}

	e.IsSucceed = true
	if _, err := x.Id(e.ID).AllCols().Update(e); err != nil {
		log.Error("Event.Build.Update: %v", err)
	}
}

// Build creates and starts a build event.
func Build(ref string) error {
	event := &Event{
		Ref: ref,
	}
	if _, err := x.Insert(event); err != nil {
		return fmt.Errorf("Insert: %v", err)
	}

	buildQueue <- event
	return nil
}
