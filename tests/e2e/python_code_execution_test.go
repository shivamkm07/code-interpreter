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
	//log message stating the test is running
	fmt.Println("Running TestBasicPythonCode")

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
	//log message stating the test is running
	fmt.Println("Running TestBasicPythonCode")

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

// write unit test to execute '{"code": "import matplotlib.pyplot as plt \nimport numpy as np \nx = np.linspace(-2*np.pi, 2*np.pi, 1000) \ny = np.tan(x) \nplt.plot(x, y) \nplt.ylim(-10, 10) \nplt.title('\”Tangent Curve'\”) \nplt.xlabel('\”x'\”) \nplt.ylabel('\”tan(x)'\”) \nplt.grid(True) \nplt.show()"}'
func TestExecuteMatplotlibCode(t *testing.T) {
	//log message stating the test is running
	fmt.Println("Running TestBasicPythonCode")

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
