package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	sessionsPrivateURL            = "https://capps-azapi-session-9baa9.capps-snazase-shivamkumar.p.azurewebsites.net/subscriptions/cb58023b-caf0-4b5e-9a01-4b9cc66960db/resourceGroups/capps-shivamkumar-rg/sessionPools/testpool2/python/execute"
	sessionsProdURL               = "https://northcentralusstage.acasessions.io/subscriptions/aa1bd316-43b3-463e-b78f-0d598e3b8972/resourceGroups/sessions-perf-northcentralus/sessionPools/testpool/python/execute"
	XMsAllocationTime             = "X-Ms-Allocation-Time"
	XMsContainerExecutionDuration = "X-Ms-Container-Execution-Duration"
	XMsExecutionReadResponseTime  = "X-Ms-Execution-Read-Response-Time"
	XMsExecutionRequestTime       = "X-Ms-Execution-Request-Time"
	XMsOverallExecutionTime       = "X-Ms-Overall-Execution-Time"
	XMsPreparationTime            = "X-Ms-Preparation-Time"
	XMsTotalExecutionServiceTime  = "X-Ms-Total-Execution-Service-Time"
)

func getSessionsURL() string {
	return sessionsProdURL
}

func getHTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * 60,
	}
}

func isSuccessStatusCode(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}
func getAccessToken() (string, error) {
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

func copyXMsHeaderValues(respHeader http.Header, w http.ResponseWriter) {
	keys := []string{XMsAllocationTime, XMsContainerExecutionDuration, XMsExecutionReadResponseTime, XMsExecutionRequestTime, XMsOverallExecutionTime, XMsPreparationTime, XMsTotalExecutionServiceTime}
	for _, key := range keys {
		val := respHeader.Get(key)
		if val != "" {
			w.Header().Set(key, val)
		}
	}
}

func executeHandler(w http.ResponseWriter, r *http.Request) {
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
	copyXMsHeaderValues(resp.Header, w)
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
