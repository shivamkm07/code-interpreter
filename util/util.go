package util

import (
	"encoding/json"
	"net/http"
	"time"
)

type appResponse struct {
	Message string `json:"message,omitempty"`
}

func HTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * 10,
	}
}

func SendHTTPResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(appResponse{Message: message})
}
