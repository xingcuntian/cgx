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

package routers

import (
	"github.com/Unknwon/cgx/models"
	"github.com/Unknwon/cgx/modules/middleware"
)

func TestHook(ctx *middleware.Context) {
	if err := models.Build(ctx.Query("ref")); err != nil {
		ctx.JSON(500, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	ctx.Status(200)
}

func Hook(ctx *middleware.Context) {

}
