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

func Sum(x int, y int) int {
	return x + y
}

func TestSum(t *testing.T) {
	total := Sum(5, 5)
	if total != 10 {
		t.Errorf("Sum was incorrect, got: %d, want: %d.", total, 10)
	}

	// Assert that the sum of 5 and 5 is 10
	assert.Equal(t, 10, total, "Sum was correct")
}

// curl -v -X 'POST' 'http://localhost:8080/execute'   -H 'Content-Type: application/json' -d '{ "code": "1+1" }'
func TestBasicPythonCode(t *testing.T) {
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
	assert.Contains(t, executionResponse.Result, "2", "Result contains 2")
}
