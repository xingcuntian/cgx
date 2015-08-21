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
	"os"
	"path"

	"github.com/Unknwon/log"
	"github.com/go-xorm/xorm"
	_ "github.com/mattn/go-sqlite3"

	"github.com/Unknwon/cgx/modules/setting"
)

var x *xorm.Engine

func init() {
	var err error
	os.MkdirAll(path.Dir(setting.DatabasePath), os.ModePerm)
	x, err = xorm.NewEngine("sqlite3", "file:"+setting.DatabasePath+"?cache=shared&mode=rwc")
	if err != nil {
		log.Fatal("Fail to init database: %v", err)
	}

	x.SetLogger(nil)
	if err = x.Sync2(new(Event)); err != nil {
		log.Fatal("Fail to sync database: %v", err)
	}
}

func sessionRelease(sess *xorm.Session) {
	if !sess.IsCommitedOrRollbacked {
		sess.Rollback()
	}
	sess.Close()
}
