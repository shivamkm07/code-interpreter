package main

import (
	"bufio"
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
	eventHubSummaryName           = "sessions-loadtest-results-summary"
	eventHubRealTimeName          = "sessions-loadtest-results-realtime"
	HTTPReqDuration               = "http_req_duration"
	XMsAllocationTime             = "X-Ms-Allocation-Time"
	XMsContainerExecutionDuration = "X-Ms-Container-Execution-Duration"
	XMsExecutionReadResponseTime  = "X-Ms-Execution-Read-Response-Time"
	XMsExecutionRequestTime       = "X-Ms-Execution-Request-Time"
	XMsOverallExecutionTime       = "X-Ms-Overall-Execution-Time"
	XMsPreparationTime            = "X-Ms-Preparation-Time"
	XMsTotalExecutionServiceTime  = "X-Ms-Total-Execution-Service-Time"
	RealTimeMetricsFile           = "real-time-metrics.json"
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

func getRealTimeMetricsFileName() string {
	fileName := os.Getenv("REAL_TIME_METRICS_FILE")
	if fileName == "" {
		fileName = RealTimeMetricsFile
	}
	return fileName
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

type RealTimeMetric struct {
	Type   string                 `json:"type"`
	Metric string                 `json:"metric"`
	Data   map[string]interface{} `json:"data"`
}

type SessionMetric struct {
	RunID     string  `json:"RunID"`
	SessionID string  `json:"SessionID"`
	Metric    string  `json:"Metric"`
	Value     float64 `json:"Value"`
}

func parseRealTimeMetrics(runId string) ([]SessionMetric, error) {
	fmt.Println(getRealTimeMetricsFileName())
	file, err := os.Open(getRealTimeMetricsFileName())
	if err != nil {
		return nil, fmt.Errorf("error opening real-time metrics file: %s", err.Error())
	}
	defer file.Close()
	var metrics []RealTimeMetric
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var metric RealTimeMetric
		// fmt.Println(string(scanner.Bytes()))
		err := json.Unmarshal(scanner.Bytes(), &metric)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling real-time metrics file: %s", err.Error())
		}
		metrics = append(metrics, metric)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading real-time metrics file: %s", err.Error())
	}
	requiredMetrics := map[string]string{
		"http_req_duration":                 "ReqDuration_Ms",
		"X_Ms_Allocation_Time":              "XMsAllocationTime_Ms",
		"X_Ms_Container_Execution_Duration": "XMsContainerExecutionDuration_Ms",
		"X_Ms_Execution_Read_Response_Time": "XMsExecutionReadResponseTime_Ms",
		"X_Ms_Execution_Request_Time":       "XMsExecutionRequestTime_Ms",
		"X_Ms_Overall_Execution_Time":       "XMsOverallExecutionTime_Ms",
		"X_Ms_Preparation_Time":             "XMsPreparationTime_Ms",
		"X_Ms_Total_Execution_Service_Time": "XMsTotalExecutionServiceTime_Ms",
	}
	var sessionMetrics []SessionMetric
	for _, metric := range metrics {
		// fmt.Println(metric)
		if metric.Type == "Point" {
			if name, ok := requiredMetrics[metric.Metric]; ok {
				value := float64(0)
				sessionId := ""
				if val, ok := metric.Data["value"].(float64); ok {
					value = val
				}
				if tags, ok := metric.Data["tags"].(map[string]interface{}); ok {
					if id, ok := tags["sessionId"].(string); ok {
						sessionId = id
					}
				}
				if sessionId == "" {
					continue
				}
				sessionMetrics = append(sessionMetrics, SessionMetric{
					RunID:     runId,
					SessionID: sessionId,
					Metric:    name,
					Value:     value,
				})
			}
		}
	}
	return sessionMetrics, nil
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func publishMetricsRealTimeHandler(w http.ResponseWriter, r *http.Request) {
	runId := r.Header.Get("RunID")
	if runId == "" {
		runId = uuid.New().String()
	}
	sessionMetrics, err := parseRealTimeMetrics(runId)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error parsing real-time metrics: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	chunkSize := 10000
	for i := 0; i < len(sessionMetrics); i += chunkSize {
		end := i + chunkSize
		if end > len(sessionMetrics) {
			end = len(sessionMetrics)
		}
		sessionMetricsChunk := sessionMetrics[i:end]
		sessionMetricsJSON, err := json.Marshal(sessionMetricsChunk)
		if err != nil {
			logAndReturnError(w, fmt.Sprintf("Error marshalling sessionMetrics: %s", err.Error()), http.StatusInternalServerError)
			return
		}
		err = publishDataToEventHubs(eventHubsNamespace, eventHubRealTimeName, sessionMetricsJSON)
		if err != nil {
			logAndReturnError(w, fmt.Sprintf("Error publishing data to Event Hubs: %s", err.Error()), http.StatusInternalServerError)
			return
		}
	}
	logger.Info("Real-time metrics published to Event Hubs Successfully")
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

func publishDataToEventHubs(eventHubsNamespace, eventHubName string, data []byte) error {
	defaultAzureCred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return fmt.Errorf("error creating DefaultAzureCredential: %s", err.Error())
	}

	producerClient, err := azeventhubs.NewProducerClient(eventHubsNamespace, eventHubName, defaultAzureCred, nil)
	if err != nil {
		return fmt.Errorf("error creating ProducerClient: %s", err.Error())
	}

	batch, err := producerClient.NewEventDataBatch(context.TODO(), &azeventhubs.EventDataBatchOptions{})
	if err != nil {
		return fmt.Errorf("error creating EventDataBatch: %s", err.Error())
	}
	event := &azeventhubs.EventData{
		Body: data,
	}
	err = batch.AddEventData(event, nil)
	if err != nil {
		return fmt.Errorf("error adding event to EventDataBatch: %s", err.Error())
	}
	err = producerClient.SendEventDataBatch(context.TODO(), batch, nil)
	if err != nil {
		return fmt.Errorf("error sending EventDataBatch: %s", err.Error())
	}
	return nil
}

func publishMetricsSummaryHandler(w http.ResponseWriter, r *http.Request) {
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error reading publish request body: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	err = publishDataToEventHubs(eventHubsNamespace, eventHubSummaryName, reqBody)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error publishing data to Event Hubs: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	logger.Info("Metrics summary published to Event Hubs Successfully")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", rootHandler)
	r.HandleFunc("/create-pool", createPoolHandler)
	r.HandleFunc("/execute", executeHandler)
	r.HandleFunc("/publish-metrics-summary", publishMetricsSummaryHandler)
	r.HandleFunc("/publish-metrics-real-time", publishMetricsRealTimeHandler)

	logger.Info("Starting server on port 8080")
	http.ListenAndServe(":8080", r)
}
