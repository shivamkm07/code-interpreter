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
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
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

type CreatePoolHandlerRequest struct {
	Location string `json:"location"`
}

type CreatePoolBody struct {
	Location   string                `json:"location"`
	Properties *CreatePoolProperties `json:"properties"`
}

type CreatePoolProperties struct {
	PoolManagementType       string                    `json:"poolManagementType"`
	MaxConcurrentSessions    int                       `json:"maxConcurrentSessions"`
	Name                     string                    `json:"name"`
	DynamicPoolConfiguration *DynamicPoolConfiguration `json:"dynamicPoolConfiguration"`
}

type DynamicPoolConfiguration struct {
	PoolType               string `json:"poolType"`
	ExecutionType          string `json:"executionType"`
	CoolDownPeriodInSecond int    `json:"coolDownPeriodInSecond"`
}

type CreatePoolResponse struct {
	Id         string                        `json:"id"`
	Name       string                        `json:"name"`
	Type       string                        `json:"type"`
	Properties *CreatePoolResponseProperties `json:"properties"`
}

type CreatePoolResponseProperties struct {
	ProvisioningState        string                    `json:"provisioningState"`
	MaxConcurrentSessions    int                       `json:"maxConcurrentSessions"`
	Name                     string                    `json:"name"`
	PoolManagementType       string                    `json:"poolManagementType"`
	DynamicPoolConfiguration *DynamicPoolConfiguration `json:"dynamicPoolConfiguration"`
	PoolManagementEndpoint   string                    `json:"poolManagementEndpoint"`
}

const (
	poolManagementURLFormat       = "https://management.azure.com/subscriptions/aa1bd316-43b3-463e-b78f-0d598e3b8972/resourceGroups/sessions-load-test/providers/Microsoft.App/sessionPools/%s?api-version=2023-08-01-preview"
	eventHubsNamespace            = "capps-test.servicebus.windows.net"
	eventHubName                  = "sessions-loadtest-results"
	XMsAllocationTime             = "X-Ms-Allocation-Time"
	XMsContainerExecutionDuration = "X-Ms-Container-Execution-Duration"
	XMsExecutionReadResponseTime  = "X-Ms-Execution-Read-Response-Time"
	XMsExecutionRequestTime       = "X-Ms-Execution-Request-Time"
	XMsOverallExecutionTime       = "X-Ms-Overall-Execution-Time"
	XMsPreparationTime            = "X-Ms-Preparation-Time"
	XMsTotalExecutionServiceTime  = "X-Ms-Total-Execution-Service-Time"
)

var (
	sessionsToken        = ""
	sessionsPoolEndpoint = "https://northcentralusstage.acasessions.io/subscriptions/aa1bd316-43b3-463e-b78f-0d598e3b8972/resourceGroups/sessions-perf-northcentralus/sessionPools/testpool/python/execute"
)

func getHTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * 60,
	}
}

func isSuccessStatusCode(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}
func getSessionsToken() (string, error) {
	// Create a new DefaultAzureCredential instance
	logger.Info("Fetching Sessions token")
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

func getManagementToken() (string, error) {
	// Create a new DefaultAzureCredential instance
	logger.Info("Fetching Management token")
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return "", fmt.Errorf("failed to create DefaultAzureCredential: %w", err)
	}

	// Get a token for the resource
	token, err := cred.GetToken(context.TODO(), policy.TokenRequestOptions{
		Scopes: []string{"https://management.azure.com/.default"},
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

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func createPoolHandler(w http.ResponseWriter, r *http.Request) {
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error reading create pool request body: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	createPoolHandlerRequest := &CreatePoolHandlerRequest{}
	err = json.Unmarshal(reqBody, createPoolHandlerRequest)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error unmarshalling create pool request body: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	testPoolName := "testpool-" + createPoolHandlerRequest.Location
	CreatePoolBody := &CreatePoolBody{
		Location: createPoolHandlerRequest.Location,
		Properties: &CreatePoolProperties{
			PoolManagementType:    "Dynamic",
			MaxConcurrentSessions: 4000,
			Name:                  testPoolName,
			DynamicPoolConfiguration: &DynamicPoolConfiguration{
				PoolType:               "JupyterPython",
				ExecutionType:          "Timed",
				CoolDownPeriodInSecond: 310,
			},
		},
	}
	CreatePoolBodyJSON, err := json.Marshal(CreatePoolBody)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error marshalling create pool request body: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	poolManagementURL := fmt.Sprintf(poolManagementURLFormat, testPoolName)
	logger.Info("Sending request to ", poolManagementURL)
	req, err := http.NewRequest("PUT", poolManagementURL, bytes.NewBuffer(CreatePoolBodyJSON))
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error creating request: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	managementToken, err := getManagementToken()
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error getting management token: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	req.Header = http.Header{
		"Authorization": []string{fmt.Sprintf("Bearer %s", managementToken)},
		"Content-Type":  []string{"application/json"},
	}
	client := getHTTPClient()
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
	if !isSuccessStatusCode(resp.StatusCode) {
		logAndReturnError(w, fmt.Sprintf("Error: Pool creation failed in %d ms. Response received: %s", delay.Milliseconds(), string(respBody)), resp.StatusCode)
		return
	}
	logger.Infof("Success: Pool created in %d ms. Response received: %s", delay.Milliseconds(), string(respBody))
	createPoolResponse := &CreatePoolResponse{}
	err = json.Unmarshal(respBody, createPoolResponse)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error unmarshalling response body: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	sessionsPoolEndpoint = createPoolResponse.Properties.PoolManagementEndpoint
	logger.Infof("Session pool endpoint set to: %s", sessionsPoolEndpoint)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
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

	if sessionsToken == "" {
		sessionsToken, err = getSessionsToken()
		if err != nil {
			logAndReturnError(w, fmt.Sprintf("Error getting sessions token: %s", err.Error()), http.StatusInternalServerError)
			return
		}
	}

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
	logger.Info("Sending request to ", sessionsPoolEndpoint)

	req, err := http.NewRequest("POST", sessionsPoolEndpoint, bytes.NewBuffer(sessionsExecuteBodyJSON))
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error creating request: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	req.Header = http.Header{
		"Authorization": []string{fmt.Sprintf("Bearer %s", sessionsToken)},
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

func publishEventHubsHandler(w http.ResponseWriter, r *http.Request) {
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error reading publish request body: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	defaultAzureCred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error creating DefaultAzureCredential: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	producerClient, err := azeventhubs.NewProducerClient(eventHubsNamespace, eventHubName, defaultAzureCred, nil)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error creating ProducerClient: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	batch, err := producerClient.NewEventDataBatch(context.TODO(), &azeventhubs.EventDataBatchOptions{})
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error creating EventDataBatch: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	event := &azeventhubs.EventData{
		Body: reqBody,
	}
	err = batch.AddEventData(event, nil)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error adding event to EventDataBatch: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	err = producerClient.SendEventDataBatch(context.TODO(), batch, nil)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error sending EventDataBatch: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	logger.Info("Metrics published to Event Hubs Successfully")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))

}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", rootHandler)
	r.HandleFunc("/create-pool", createPoolHandler)
	r.HandleFunc("/execute", executeHandler)
	r.HandleFunc("/publish-eventhubs", publishEventHubsHandler)

	logger.Info("Starting server on port 8080")
	http.ListenAndServe(":8080", r)
}
