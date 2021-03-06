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

// Cgx(Continuous Go Cross-compiler) is a real-time cross-compiler for your Go apps.
package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Unknwon/com"
	"github.com/Unknwon/log"
	"github.com/Unknwon/macaron"
	"github.com/macaron-contrib/pongo2"

	"github.com/Unknwon/cgx/models"
	"github.com/Unknwon/cgx/modules/middleware"
	"github.com/Unknwon/cgx/modules/setting"
)

const APP_VER = "0.1.0.0903"

func main() {
	setting.AppVer = APP_VER

	log.Info("%s %s", setting.AppName, setting.AppVer)
	log.Info("Run Mode: %s", strings.Title(macaron.Env))

	m := macaron.Classic()
	m.Use(macaron.Static(setting.ArchivePath, macaron.StaticOptions{
		Prefix: "/archive",
	}))
	m.Use(pongo2.Pongoer())
	m.Use(middleware.Contexter())

	m.Get("/", func(ctx *middleware.Context) {
		ctx.Data["Targets"] = models.Targets
		ctx.HTML(200, "home")
	})
	if setting.Webhook.Mode == "test" {
		m.Get("/hook", func(ctx *middleware.Context) {
			if err := models.Build(ctx.Query("ref")); err != nil {
				ctx.JSON(500, map[string]interface{}{
					"error": err.Error(),
				})
				return
			}

			ctx.Status(200)
		})
	} else {
		m.Post("/hook", func(ctx *middleware.Context) {

		})
	}

	listenAddr := "0.0.0.0:" + com.ToStr(setting.HTTPPort)
	log.Info("Listen on http://%s", listenAddr)
	fmt.Println(http.ListenAndServe(listenAddr, m))
}
