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
	"strconv"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type ExecuteRequest struct {
	Code     string `json:"code"`
	Location string `json:"location"`
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
	eventHubRealTimeName          = "sessions-loadtest-results-all"
	HTTPReqDuration               = "http_req_duration"
	XMsAllocationTime             = "X-Ms-Allocation-Time"
	XMsContainerExecutionDuration = "X-Ms-Container-Execution-Duration"
	XMsExecutionReadResponseTime  = "X-Ms-Execution-Read-Response-Time"
	XMsExecutionRequestTime       = "X-Ms-Execution-Request-Time"
	XMsOverallExecutionTime       = "X-Ms-Overall-Execution-Time"
	XMsPreparationTime            = "X-Ms-Preparation-Time"
	XMsTotalExecutionServiceTime  = "X-Ms-Total-Execution-Service-Time"
	RealTimeMetricsFile           = "real-time-metrics.json"
	ncusStageRegion               = "northcentralusstage"
)

var (
	sessionsToken        = ""
	sessionsPoolEndpoint = ""
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
	TIMESTAMP time.Time `json:"TIMESTAMP"`
	RunID     string    `json:"RunID"`
	SessionID string    `json:"SessionID"`
	Metric    string    `json:"Metric"`
	Value     float64   `json:"Value"`
	Passed    bool      `json:"Passed"`
}

type ExecutionMetric struct {
	TIMESTAMP                        time.Time `json:"TIMESTAMP"`
	RunID                            string    `json:"RunID"`
	SessionID                        string    `json:"SessionID"`
	Passed                           bool      `json:"Passed"`
	ReqDuration_Ms                   float64   `json:"ReqDuration_Ms"`
	XMsAllocationTime_Ms             float64   `json:"XMsAllocationTime_Ms"`
	XMsContainerExecutionDuration_Ms float64   `json:"XMsContainerExecutionDuration_Ms"`
	XMsExecutionReadResponseTime_Ms  float64   `json:"XMsExecutionReadResponseTime_Ms"`
	XMsExecutionRequestTime_Ms       float64   `json:"XMsExecutionRequestTime_Ms"`
	XMsOverallExecutionTime_Ms       float64   `json:"XMsOverallExecutionTime_Ms"`
	XMsPreparationTime_Ms            float64   `json:"XMsPreparationTime_Ms"`
	XMsTotalExecutionServiceTime_Ms  float64   `json:"XMsTotalExecutionServiceTime_Ms"`
}

func parseRealTimeMetrics(runId string) ([]ExecutionMetric, error) {
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
	executionMetrics := make(map[string]*ExecutionMetric)
	for _, metric := range metrics {
		// fmt.Println(metric)
		if metric.Type == "Point" {
			if name, ok := requiredMetrics[metric.Metric]; ok {
				value := float64(0)
				sessionId := ""
				statusCode := 0
				var timeStamp time.Time
				if val, ok := metric.Data["value"].(float64); ok {
					value = val
				}
				if ts, ok := metric.Data["time"].(string); ok {
					timeStamp, err = time.Parse(time.RFC3339, ts)
					if err != nil {
						return nil, fmt.Errorf("error parsing time: %s", err.Error())
					}
				} else {
					continue
				}
				if tags, ok := metric.Data["tags"].(map[string]interface{}); ok {
					if id, ok := tags["sessionId"].(string); ok {
						sessionId = id
					} else {
						continue
					}
					if name == "ReqDuration_Ms" {
						if status, ok := tags["status"]; ok {
							statusCode, err = strconv.Atoi(status.(string))
							if err != nil {
								return nil, fmt.Errorf("error converting status code to int: %s", err.Error())
							}
						} else {
							continue
						}
					}
				}
				passed := statusCode >= 200 && statusCode < 300
				if _, ok := executionMetrics[sessionId]; !ok {
					executionMetrics[sessionId] = &ExecutionMetric{}
				}
				executionMetric := executionMetrics[sessionId]
				switch name {
				case "ReqDuration_Ms":
					executionMetric.ReqDuration_Ms = value
					executionMetric.Passed = passed
					executionMetric.TIMESTAMP = timeStamp
					executionMetric.RunID = runId
					executionMetric.SessionID = sessionId
				case "XMsAllocationTime_Ms":
					executionMetric.XMsAllocationTime_Ms = value
				case "XMsContainerExecutionDuration_Ms":
					executionMetric.XMsContainerExecutionDuration_Ms = value
				case "XMsExecutionReadResponseTime_Ms":
					executionMetric.XMsExecutionReadResponseTime_Ms = value
				case "XMsExecutionRequestTime_Ms":
					executionMetric.XMsExecutionRequestTime_Ms = value
				case "XMsOverallExecutionTime_Ms":
					executionMetric.XMsOverallExecutionTime_Ms = value
				case "XMsPreparationTime_Ms":
					executionMetric.XMsPreparationTime_Ms = value
				case "XMsTotalExecutionServiceTime_Ms":
					executionMetric.XMsTotalExecutionServiceTime_Ms = value
				}
			}
		}
	}
	executionMetricList := make([]ExecutionMetric, 0, len(executionMetrics))
	for _, metric := range executionMetrics {
		executionMetricList = append(executionMetricList, *metric)
	}
	return executionMetricList, nil
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
	chunkSize := 500
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

func getPoolManagementReq(method string, poolManagementURL string, token string, reqBody []byte) (*http.Request, error) {
	req, err := http.NewRequest(method, poolManagementURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %s", err.Error())
	}
	req.Header = http.Header{
		"Authorization": []string{fmt.Sprintf("Bearer %s", token)},
		"Content-Type":  []string{"application/json"},
	}
	return req, nil
}

func createPool(location string) error {
	if location == "" {
		location = ncusStageRegion
	}
	testPoolName := "testpool-" + location
	CreatePoolBody := &CreatePoolBody{
		Location: location,
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
		return fmt.Errorf("error marshalling create pool request body: %s", err.Error())
	}
	poolManagementURL := fmt.Sprintf(poolManagementURLFormat, testPoolName)
	managementToken, err := getManagementToken()
	if err != nil {
		return fmt.Errorf("error getting management token: %s", err.Error())
	}
	client := getHTTPClient()

	// Delete existing pool if it exists
	req, err := getPoolManagementReq("DELETE", poolManagementURL, managementToken, nil)
	if err != nil {
		return fmt.Errorf("error creating DELETE request: %s", err.Error())
	}
	_, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending DELETE request: %s", err.Error())
	}
	// Create new pool
	req, err = getPoolManagementReq("PUT", poolManagementURL, managementToken, CreatePoolBodyJSON)
	if err != nil {
		return fmt.Errorf("error creating PUT request: %s", err.Error())
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending PUT request: %s", err.Error())
	}
	delay := time.Since(start)
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %s", err.Error())
	}
	if !isSuccessStatusCode(resp.StatusCode) {
		return fmt.Errorf("error: Pool creation failed in %d ms. Response received: %s", delay.Milliseconds(), string(respBody))
	}
	logger.Infof("Success: Pool created in %d ms. Response received: %s", delay.Milliseconds(), string(respBody))
	createPoolResponse := &CreatePoolResponse{}
	err = json.Unmarshal(respBody, createPoolResponse)
	if err != nil {
		return fmt.Errorf("error unmarshalling response body: %s", err.Error())
	}
	sessionsPoolEndpoint = createPoolResponse.Properties.PoolManagementEndpoint
	logger.Infof("Session pool endpoint set to: %s", sessionsPoolEndpoint)
	return nil
}

func createPoolHandler(w http.ResponseWriter, r *http.Request) {
	createPoolHandlerRequest := &CreatePoolHandlerRequest{}
	if r.Body != nil {
		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			logAndReturnError(w, fmt.Sprintf("Error reading create pool request body: %s", err.Error()), http.StatusInternalServerError)
			return
		}
		err = json.Unmarshal(reqBody, createPoolHandlerRequest)
		if err != nil {
			logAndReturnError(w, fmt.Sprintf("Error unmarshalling create pool request body: %s", err.Error()), http.StatusInternalServerError)
			return
		}
	}
	err := createPool(createPoolHandlerRequest.Location)
	if err != nil {
		logAndReturnError(w, fmt.Sprintf("Error creating pool: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
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
	if sessionsPoolEndpoint == "" {
		logAndReturnError(w, "Error: Sessions pool endpoint not set", http.StatusInternalServerError)
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
