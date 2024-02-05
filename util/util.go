package util

import (
	"net/http"
	"time"
)

func HTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * 10,
	}
}

func SendHTTPResponse(w http.ResponseWriter, statusCode int, message string, wrapMessage bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if wrapMessage {
		message = `{"message": "` + message + `"}`
	}
	w.Write([]byte(message))
}
