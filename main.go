package main

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

import (
	"net/http"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/gorilla/mux"
	"github.com/microsoft/jupyterpython/codeexecution"
	"github.com/microsoft/jupyterpython/fileservices"
	"github.com/microsoft/jupyterpython/jupyterservices"
	"github.com/microsoft/jupyterpython/util"
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(os.Stdout)
}

func main() {
	r := mux.NewRouter()
	setToken()

	log.Info().Msgf("Starting Jupyter API server with token: %s", jupyterservices.Token)

	// Define your routes
	r.HandleFunc("/", initializeJupyter).Methods("GET")
	r.HandleFunc("/execute", codeexecution.Execute).Methods("POST")

	// health check
	r.HandleFunc("/health", codeexecution.HealthHandler).Methods("GET")
	r.HandleFunc("/listfiles", fileservices.ListFilesHandler).Methods("GET")
	r.HandleFunc("/listfiles/{path:.*}", fileservices.ListFilesHandler).Methods("GET")
	r.HandleFunc("/upload", fileservices.UploadFileHandler).Methods("POST")
	r.HandleFunc("/upload/{path:.*}", fileservices.UploadFileHandler).Methods("POST")
	r.HandleFunc("/download/{filename}", fileservices.DownloadFileHandler).Methods("GET")
	r.HandleFunc("/download/{path:.*}/{filename}", fileservices.DownloadFileHandler).Methods("GET")
	r.HandleFunc("/delete/{filename}", fileservices.DeleteFileHandler).Methods("DELETE")
	r.HandleFunc("/get/{filename}", fileservices.GetFileHandler).Methods("GET")

	// Run health check in the background
	go codeexecution.PeriodicCodeExecution()

	var cfg = util.GetConfig()

	if cfg.UseTls == "true" {
		log.Info().Msg("Starting server on port :6000 with cert " + cfg.XdsCertFilePath + " and key " + cfg.XdsCertKeyFilePath)
		error := http.ListenAndServeTLS(":6000", cfg.XdsCertFilePath, cfg.XdsCertKeyFilePath, r)
		if error != nil {
			log.Error().Msg("HTTPS Server Error: " + error.Error())
		}
	} else {
		log.Info().Msg("Starting server on port :6000")
		error := http.ListenAndServe(":6000", r)
		if error != nil {
			log.Error().Msg("HTTP Server Error: " + error.Error())
		}
	}
}

// func to take token from the environment variable
func setToken() {
	jupyterservices.Token = os.Getenv("JUPYTER_GEN_TOKEN")
	if jupyterservices.Token == "" {
		jupyterservices.Token = "test"
		log.Info().Msg("Token not found in environment variable, using default token: " + jupyterservices.Token)
	} else {
		log.Info().Msg("Token found in environment variable: " + jupyterservices.Token)
	}
}

// func to initialize jupyter
func initializeJupyter(w http.ResponseWriter, r *http.Request) {
	_, _, err := jupyterservices.CheckKernels("")
	if err != nil {
		log.Err(err).Msg("Failed to check kernels")
		util.SendHTTPResponse(w, http.StatusInternalServerError, "error checking kernels"+err.Error(), true)
	}
	util.SendHTTPResponse(w, http.StatusOK, "jupyter initialized with token: "+jupyterservices.Token, true)
}
