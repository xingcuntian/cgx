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
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/Unknwon/cae/zip"
	"github.com/Unknwon/com"
	"github.com/Unknwon/log"

	"github.com/Unknwon/cgx/modules/setting"
)

type Target struct {
	Name   string
	Branch string
	setting.Target
	LastBuild time.Time
}

var Targets []*Target

func init() {
	zip.Verbose = false

	go func() {
		for e := range buildQueue {
			e.Build()
		}
	}()

	Targets = make([]*Target, 0, len(setting.Branches)*len(setting.Targets))
	for _, branch := range setting.Branches {
		for _, target := range setting.Targets {
			t := &Target{
				Name: path.Base(setting.Repository.ImportPath) + "_" +
					targetString(branch, target.GOOS, target.GOARCH, target.GOARM),
				Branch: branch,
			}
			t.Target = target
			Targets = append(Targets, t)
		}
	}
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

func targetString(ref, goos, goarch, goarm string) string {
	str := fmt.Sprintf("%s_%s_%s", ref, goos, goarch)
	if len(goarm) > 0 {
		str += "_arm" + com.ToStr(goarm)
	}
	return str
}

func (e *Event) targetString(target setting.Target) string {
	return targetString(e.Ref, target.GOOS, target.GOARCH, target.GOARM)
}

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

func (e *Event) Build() {
	defer func() {
		log.Debug("Build finished: %s", e.Ref)
	}()

	// Fetch archive for reference.
	localPath := path.Join(setting.GOPATHSrc, path.Dir(setting.Repository.ImportPath))
	name := path.Base(setting.Repository.ImportPath)
	fullLocalPath := path.Join(localPath, name)

	defer os.RemoveAll(path.Join(setting.GOPATH, "bin"))

	// Start building targets.
	for _, target := range setting.Targets {
		envs := append([]string{
			"GOPATH=" + setting.GOPATH,
			"GOOS=" + target.GOOS,
			"GOARCH=" + target.GOARCH,
			"GOARM=" + target.GOARM,
		}, os.Environ()...)
		tags := setting.Cfg.Section("tags." + target.GOOS).Key("TAGS").MustString("")

		bufErr := new(bytes.Buffer)

		log.Debug("Getting dependencies: %s", e.targetString(target))
		cmd := exec.Command("go", "get", "-v", "-u", "-tags", tags, setting.Repository.ImportPath)
		cmd.Env = envs
		cmd.Stdout = os.Stdout
		cmd.Stderr = bufErr

		if err := cmd.Run(); err != nil {
			fmt.Println(bufErr.String())
			e.setError(fmt.Sprintf("Event.Build.(get dependencies): %s", bufErr.String()))
			return
		}

		log.Debug("Checking out branch: %s", e.Ref)
		cmd = exec.Command("git", "checkout", e.Ref)
		cmd.Env = envs
		cmd.Dir = fullLocalPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = bufErr

		if err := cmd.Run(); err != nil {
			fmt.Println(bufErr.String())
			e.setError(fmt.Sprintf("Event.Build.(checkout branch): %s", bufErr.String()))
			return
		}

		log.Debug("Building target: %s", e.targetString(target))
		cmd = exec.Command("go", "build", "-v", "-tags", tags)
		cmd.Env = envs
		cmd.Dir = fullLocalPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = bufErr

		if err := cmd.Run(); err != nil {
			fmt.Println(bufErr.String())
			e.setError(fmt.Sprintf("Event.Build.(build target): %s", bufErr.String()))
			return
		}

		if err := e.packTarget(fullLocalPath, name, setting.ArchivePath, target); err != nil {
			e.setError(fmt.Sprintf("Event.Build.packTarget: %v", err))
			return
		}

		targetName := name + "_" + e.targetString(target)
		for i := range Targets {
			if Targets[i].Name == targetName {
				Targets[i].LastBuild = time.Now()
				break
			}
		}
	}

	e.IsSucceed = true
	if _, err := x.Id(e.ID).AllCols().Update(e); err != nil {
		log.Error("Event.Build.Update: %v", err)
	}
}

// Build creates and starts a build event.
func Build(ref string) error {
	if len(ref) == 0 {
		return fmt.Errorf("empty ref")
	}

	event := &Event{
		Ref: ref,
	}
	if _, err := x.Insert(event); err != nil {
		return fmt.Errorf("Insert: %v", err)
	}

	buildQueue <- event
	return nil
}
