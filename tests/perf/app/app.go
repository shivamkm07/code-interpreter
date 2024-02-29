package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type ExecuteRequest struct {
	Code string `json:"code"`
}

// Create a logger
var logger = logrus.New()

type SessionsExecuteBody struct {
	Properties *SessionsExecuteProperties `json:"properties"`
}

type SessionsExecuteProperties struct {
	Identifier    string `json:"identifier"`
	CodeInputType string `json:"codeInputType"`
	ExecutionType string `json:"executionType"`
	PythonCode    string `json:"pythonCode"`
}

const (
	sessionsPrivateURL = "https://capps-azapi-session-9baa9.capps-snazase-shivamkumar.p.azurewebsites.net/subscriptions/cb58023b-caf0-4b5e-9a01-4b9cc66960db/resourceGroups/capps-shivamkumar-rg/sessionPools/testpool2/python/execute"
	sessionsProdURL    = "https://northcentralusstage.acasessions.io/subscriptions/88b30252-64f2-481d-888f-3bd5de377231/resourceGroups/sessions-perf-test/sessionPools/testpool/python/execute"
)

func getSessionsURL() string {
	return sessionsProdURL
}

func getHTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * 10,
	}
}

func isSuccessStatusCode(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}
func getAccessToken() (string, error) {
	// Check if the ACCESS_TOKEN environment variable is set, if yes return it as the access token
	accessToken := os.Getenv("ACCESS_TOKEN")
	if accessToken != "" {
		return accessToken, nil
	}
	// Create a new DefaultAzureCredential instance
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return "", fmt.Errorf("failed to create DefaultAzureCredential: %w", err)
	}

	// Get a token for the resource
	token, err := cred.GetToken(context.TODO(), policy.TokenRequestOptions{
		Scopes: []string{"https://acasessions.io/.default"},
	})
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	return token.Token, nil
}

func logAndReturnError(w http.ResponseWriter, message string, statusCode int) {
	logger.Error(message)
	http.Error(w, message, statusCode)
}

func executeHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello, World!"))
	return
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error reading request body: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	executeRequest := &ExecuteRequest{}
	err = json.Unmarshal(reqBody, executeRequest)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error unmarshalling request body: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	sessionID := r.Header.Get("IDENTIFIER")
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	accessToken, err := getAccessToken()
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error getting access token: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	sessionsURL := getSessionsURL()
	client := getHTTPClient()
	sessionsExecuteBody := &SessionsExecuteBody{
		Properties: &SessionsExecuteProperties{
			Identifier:    sessionID,
			CodeInputType: "inline",
			ExecutionType: "synchronous",
			PythonCode:    executeRequest.Code,
		},
	}

	sessionsExecuteBodyJSON, err := json.Marshal(sessionsExecuteBody)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error marshalling request body: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	logger.Info("Sending request to ", sessionsURL)

	req, err := http.NewRequest("POST", sessionsURL, bytes.NewBuffer(sessionsExecuteBodyJSON))
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error creating request: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	req.Header = http.Header{
		"Authorization": []string{fmt.Sprintf("Bearer %s", accessToken)},
		"Content-Type":  []string{"application/json"},
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error sending request: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	delay := time.Since(start)
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error reading response body: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	if isSuccessStatusCode(resp.StatusCode) {
		logger.Infof("Success: Code executed in %d ms. Response received: %s", delay.Milliseconds(), string(respBody))
	} else {
		logger.Errorf("Error: Code execution failed in %d ms. Response received: %s", delay.Milliseconds(), string(respBody))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/execute", executeHandler)

	logger.Info("Starting server on port 8080")
	http.ListenAndServe(":8080", r)
}
