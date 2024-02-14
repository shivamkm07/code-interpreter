// Copyright 2023 Microsoft Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package codeexecution

import (
	"net/http"
	"time"

	"github.com/microsoft/jupyterpython/jupyterservices"
	"github.com/microsoft/jupyterpython/util"
	"github.com/rs/zerolog/log"
)

var lastCodeHealthCheck bool

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	if !lastCodeHealthCheck {
		util.SendHTTPResponse(w, http.StatusInternalServerError, "unhealthy exec code failed", true)
		return
	}
	util.SendHTTPResponse(w, http.StatusOK, "healthy", true)
}

func PeriodicCodeExecution() {
	time.Sleep(30 * time.Second)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	sampleCode := "1+1"
	for range ticker.C {
		kernelId, sessionId, err := jupyterservices.CheckKernels("")
		if err != nil {
			log.Error().Msg("Failed to check kernels: " + err.Error())
			panic("Health Ping Failed with error: " + err.Error())
		}
		response := executeCode(kernelId, sessionId, sampleCode)
		if response.ErrorName == "" || response.Stderr == "" {
			lastCodeHealthCheck = true
			log.Info().Msg("Periodic code execution successful")
		} else {
			lastCodeHealthCheck = false
			log.Error().Msg("Failed to execute code")
		}
	}
}
