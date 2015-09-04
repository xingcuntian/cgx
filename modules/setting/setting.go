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

package setting

import (
	"os"
	"path/filepath"

	"github.com/Unknwon/com"
	"github.com/Unknwon/log"
	"github.com/Unknwon/macaron"
	"gopkg.in/ini.v1"
)

type Target struct {
	GOOS, GOARCH, GOARM string
}

var (
	AppName      string
	AppVer       string
	RunMode      string
	HTTPPort     int
	DatabasePath string
	ArchivePath  string
	GOPATH       = "gopath"
	GOPATHSrc    = "gopath/src"

	Repository struct {
		ImportPath string
	}

	Webhook struct {
		Mode string
	}

	Branches []string

	Targets []Target

	Resources []string

	Cfg *ini.File
)

func init() {
	log.Prefix = "[CGX]"

	var err error
	Cfg, err = ini.Load("conf/app.ini")
	if err != nil {
		log.Fatal("Fail to load config: %v", err)
	}
	if com.IsFile("custom/app.ini") {
		if err = Cfg.Append("custom/app.ini"); err != nil {
			log.Fatal("Fail to load custom config: %v", err)
		}
	}

	AppName = Cfg.Section("").Key("APP_NAME").MustString("Continuous Go Cross-compiler")
	RunMode = Cfg.Section("").Key("RUN_MODE").In("dev", []string{"dev", "prod"})
	if RunMode == "prod" {
		macaron.Env = macaron.PROD
	}
	HTTPPort = Cfg.Section("").Key("HTTP_PORT").MustInt(3050)
	DatabasePath = Cfg.Section("").Key("DATABASE_PATH").MustString("data/cgx.db")
	ArchivePath = Cfg.Section("").Key("ARCHIVE_PATH").MustString("data/archive")

	if !filepath.IsAbs(GOPATH) {
		wd, _ := os.Getwd()
		GOPATH = filepath.Join(wd, GOPATH)
	}

	sec := Cfg.Section("repository")
	Repository.ImportPath = sec.Key("IMPORT_PATH").MustString("")

	sec = Cfg.Section("webhook")
	Webhook.Mode = sec.Key("MODE").In("test", []string{"test", "travis", "github"})

	branchesInfo := Cfg.Section("branches").Keys()
	Branches = make([]string, len(branchesInfo))
	for i := range branchesInfo {
		Branches[i] = branchesInfo[i].String()
	}

	targetsInfo := Cfg.Section("targets").Keys()
	Targets = make([]Target, len(targetsInfo))
	for i := range targetsInfo {
		infos := targetsInfo[i].Strings(" ")
		if len(infos) < 2 {
			log.Fatal("target at least contain GOOS and GOARCH: %s", targetsInfo[i])
		}
		Targets[i].GOOS = infos[0]
		Targets[i].GOARCH = infos[1]
		if len(infos) >= 3 {
			Targets[i].GOARM = infos[2]
		}

	}

	resInfo := Cfg.Section("resources").Keys()
	Resources = make([]string, len(resInfo))
	for i := range resInfo {
		Resources[i] = resInfo[i].MustString("")
	}
}
