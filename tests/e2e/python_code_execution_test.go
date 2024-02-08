package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type ExecutionResponse struct {
	Hresult        int            `json:"hresult"`
	Result         string         `json:"result"`
	ErrorName      string         `json:"error_name"`
	ErrorMessage   string         `json:"error_message"`
	Stdout         string         `json:"stdout"`
	Stderr         string         `json:"stderr"`
	DiagnosticInfo DiagnosticInfo `json:"diagnosticInfo"`
}

type DiagnosticInfo struct {
	ExecutionDuration int `json:"executionDuration"`
}

func TestExecuteSumCode(t *testing.T) {
	//log message stating the test is running
	fmt.Println("Running TestBasicPythonCode")

	var httpPostRequest = "http://localhost:8080/execute"
	var httpPostBody = "{ \"code\": \"1+1\" }"

	response, err := http.Post(httpPostRequest, "application/json", bytes.NewBufferString(httpPostBody))

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// Unmarshal the response body
	var executionResponse ExecutionResponse
	err = json.Unmarshal(body, &executionResponse)

	assert.Nil(t, err, "No error")

	// check if executionResponse.Result contains 2
	assert.Equal(t, "2; ", executionResponse.Result, "Result contains 2; ")
}

func TestExecuteSleepAndPrintCode(t *testing.T) {
	var httpPostRequest = "http://localhost:8080/execute"
	var httpPostBody = "{ \"code\": \"import time \ntime.sleep(5) \nprint(\\\"Done Sleeping\\\")\" }"

	response, err := http.Post(httpPostRequest, "application/json", bytes.NewBufferString(httpPostBody))

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// Unmarshal the response body
	var executionResponse ExecutionResponse
	err = json.Unmarshal(body, &executionResponse)

	assert.Nil(t, err, "No error")

	// check if executionResponse.Result contains 2
	assert.Equal(t, 0, executionResponse.Hresult, "Hresult is 0")
}

func TestExecuteHelloEarthCode(t *testing.T) {
	var httpPostRequest = "http://localhost:8080/execute"
	var httpPostBody = "{ \"code\": \"print(\\\"Hello Earth\\\")\" }"

	response, err := http.Post(httpPostRequest, "application/json", bytes.NewBufferString(httpPostBody))

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// Unmarshal the response body
	var executionResponse ExecutionResponse
	err = json.Unmarshal(body, &executionResponse)

	assert.Nil(t, err, "No error")

	// check if executionResponse.Result contains 2
	assert.Equal(t, "Hello Earth\n", executionResponse.Stdout, "Stdout equals Hello Earth\n")
}

func TestExecuteMatplotlibCode(t *testing.T) {
	var httpPostRequest = "http://localhost:8080/execute"
	var httpPostBody = "{ \"code\": \"import matplotlib.pyplot as plt \nimport numpy as np \nx = np.linspace(-2*np.pi, 2*np.pi, 1000) \ny = np.tan(x) \nplt.plot(x, y) \nplt.ylim(-10, 10) \nplt.title('Tangent Curve') \nplt.xlabel('x') \nplt.ylabel('tan(x)') \nplt.grid(True) \nplt.show()\" }"

	response, err := http.Post(httpPostRequest, "application/json", bytes.NewBufferString(httpPostBody))

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// Unmarshal the response body
	var executionResponse ExecutionResponse
	err = json.Unmarshal(body, &executionResponse)

	assert.Nil(t, err, "No error")

	// check if executionResponse.Result contains 2
	assert.Equal(t, 0, executionResponse.Hresult, "Hresult is 0")
}

// Write test for all of this route shown below

//r.HandleFunc("/", initializeJupyter).Methods("GET")
//r.HandleFunc("/health", healthHandler).Methods("GET")
//r.HandleFunc("/listfiles", fileservices.ListFilesHandler).Methods("GET")
//r.HandleFunc("/listfiles/{path:.*}", fileservices.ListFilesHandler).Methods("GET")
//r.HandleFunc("/upload", fileservices.UploadFileHandler).Methods("POST")
//r.HandleFunc("/upload/{path:.*}", fileservices.UploadFileHandler).Methods("POST")
//r.HandleFunc("/download/{filename}", fileservices.DownloadFileHandler).Methods("GET")
//r.HandleFunc("/download/{path:.*}/{filename}", fileservices.DownloadFileHandler).Methods("GET")
//r.HandleFunc("/delete/{filename}", fileservices.DeleteFileHandler).Methods("DELETE")
//r.HandleFunc("/get/{filename}", fileservices.GetFileHandler).Methods("GET")

func TestInitializeJupyter(t *testing.T) {
	var httpGetRequest = "http://localhost:8080/"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// check if response body contains "Jupyter initialized with token, test."
	assert.Equal(t, "{\"message\": \"Jupyter initialized with token, test.\"}", string(body), "Response body contains Jupyter initialized with token, test.")
}

func TestHealthHandler(t *testing.T) {
	var httpGetRequest = "http://localhost:8080/health"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// check if response body contains "Jupyter initialized with token, test."
	assert.Equal(t, "Healthy\n", string(body), "Response body contains 'Healthy\n'")
}

func TestListFilesHandler(t *testing.T) {
	var httpGetRequest = "http://localhost:8080/listfiles"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// check if response body contains "Jupyter initialized with token, test."
	assert.Equal(t, "null", string(body), "Response body contains null")
}

// TODO: fix this test to upload a file
func TestUploadFileHandler(t *testing.T) {
	var httpPostRequest = "http://localhost:8080/upload"
	// upload a file as request body
	var httpPostBody = "file"

	response, err := http.Post(httpPostRequest, "application/json", bytes.NewBufferString(httpPostBody))

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// check if response body contains "Jupyter initialized with token, test."
	assert.Equal(t, "null", string(body), "Response body contains null")
}

// TODO: fix this test to download a file
func TestDownloadFileHandlerFileNotFound(t *testing.T) {
	var httpGetRequest = "http://localhost:8080/download/file"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// check if response body contains "Jupyter initialized with token, test."
	assert.Equal(t, "ERR_FILE_NOT_FOUND: File not found\n", string(body), "Response body contains ERR_FILE_NOT_FOUND")
}

// TODO: Add test to delete a file which is uploaded
func TestDeleteFileHandlerFileNotFound(t *testing.T) {
	var httpDeleteRequest = "http://localhost:8080/delete/file"
	request, err := http.NewRequest("DELETE", httpDeleteRequest, nil)
	response, err := http.DefaultClient.Do(request)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// check if response body contains "Jupyter initialized with token, test."
	assert.Equal(t, "ERR_FILE_NOT_FOUND: File not found\n", string(body), "Response body contains ERR_FILE_NOT_FOUND")
}

func TestGetFileHandlerFileNotFound(t *testing.T) {
	var httpGetRequest = "http://localhost:8080/get/file"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// check if response body contains "Jupyter initialized with token, test."
	assert.Equal(t, "ERR_FILE_NOT_FOUND: File not found\n", string(body), "Response body contains ERR_FILE_NOT_FOUND")
}

func TestListFilesHandlerWithPath(t *testing.T) {
	var httpGetRequest = "http://localhost:8080/listfiles/path"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// check if response body contains "Jupyter initialized with token, test."
	assert.Equal(t, "ERR_DIR_NOT_FOUND: File path not found\n", string(body), "Response body contains ERR_DIR_NOT_FOUND")
}

// TODO: fix this test to upload a file
func TestUploadFileHandlerWithPath(t *testing.T) {
	var httpPostRequest = "http://localhost:8080/upload/path"
	// upload a file as request body
	var httpPostBody = "file"

	response, err := http.Post(httpPostRequest, "application/json", bytes.NewBufferString(httpPostBody))

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// check if response body contains "Jupyter initialized with token, test."
	assert.Equal(t, "Unable to parse form\n", string(body), "Response body contains ERR_DIR_NOT_FOUND")
}
