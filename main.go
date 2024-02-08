package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/gorilla/mux"
	"github.com/microsoft/jupyterpython/codeexecution"
	"github.com/microsoft/jupyterpython/fileservices"
	"github.com/microsoft/jupyterpython/jupyterservices"
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(os.Stdout)
}

var token = "test"
var lastCodeHealthCheck bool

func main() {
	r := mux.NewRouter()

	log.Info().Msgf("Starting Jupyter API server with token: %s", token)

	// Define your routes
	r.HandleFunc("/", initializeJupyter).Methods("GET")
	r.HandleFunc("/execute", codeexecution.Execute).Methods("POST")

	// health check
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/listfiles", fileservices.ListFilesHandler).Methods("GET")
	r.HandleFunc("/listfiles/{path:.*}", fileservices.ListFilesHandler).Methods("GET")
	r.HandleFunc("/upload", fileservices.UploadFileHandler).Methods("POST")
	r.HandleFunc("/upload/{path:.*}", fileservices.UploadFileHandler).Methods("POST")
	r.HandleFunc("/download/{filename}", fileservices.DownloadFileHandler).Methods("GET")
	r.HandleFunc("/download/{path:.*}/{filename}", fileservices.DownloadFileHandler).Methods("GET")
	r.HandleFunc("/delete/{filename}", fileservices.DeleteFileHandler).Methods("DELETE")
	r.HandleFunc("/get/{filename}", fileservices.GetFileHandler).Methods("GET")

	fmt.Println("Server listening on :8080")

	// Run health check in the background
	go periodicCodeExecution()

	http.ListenAndServe(":8080", r)
}

// func to take token from the environment variable
func getToken() string {
	token = os.Getenv("JUPYTER_TOKEN")
	if token == "" {
		token = "test"
		log.Info().Msg("Token not found in environment variable, using default token: " + token)
	}
	return token
}

// func to initialize jupyter
func initializeJupyter(w http.ResponseWriter, r *http.Request) {
	// get token from the environment variable
	token = getToken()
	jupyterservices.CheckKernels("")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Jupyter initialized with token, ` + token + `."}`))

}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if !lastCodeHealthCheck {
		http.Error(w, "Unhealthy code exec failed", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "Healthy")
}

func periodicCodeExecution() {
	time.Sleep(25 * time.Second)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	sampleCode := "1+1"
	for {
		select {
		case <-ticker.C:
			kernelId, sessionId := jupyterservices.CheckKernels("")
			response := codeexecution.ExecuteCode(kernelId, sessionId, sampleCode)
			if response.ErrorName == "" || response.Stderr == "" {
				lastCodeHealthCheck = true
				log.Info().Msg("Periodic code execution successful")
			} else {
				lastCodeHealthCheck = false
				log.Error().Msg("Failed to execute code")
				panic("Health Ping Failed")
			}
		}
	}
}
