package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"testing"

	"os"

	ce "github.com/microsoft/jupyterpython/codeexecution"
	fs "github.com/microsoft/jupyterpython/fileservices"
	"github.com/stretchr/testify/assert"
)

func TestExecuteSumCode(t *testing.T) {
	var httpPostRequest = "http://localhost:6000/execute"
	var httpPostBody = "{ \"code\": \"1+1\" }"

	response, err := http.Post(httpPostRequest, "application/json", bytes.NewBufferString(httpPostBody))

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// Unmarshal the response body
	var executionResponse ce.ExecutionResponse
	//convert body to string
	err = json.Unmarshal(body, &executionResponse)

	assert.Nil(t, err, "No error")

	var actualResult int
	err = json.Unmarshal(*executionResponse.Result, &actualResult)

	assert.Nil(t, err, "No error")

	// check if executionResponse.Result contains 2
	assert.Equal(t, 2, actualResult, "Result is 2 ")
}

func TestExecuteSleepAndPrintCode(t *testing.T) {
	var httpPostRequest = "http://localhost:6000/execute"
	var httpPostBody = "{ \"code\": \"import time \\ntime.sleep(5) \\nprint(\\\"Done Sleeping\\\")\" }"

	response, err := http.Post(httpPostRequest, "application/json", bytes.NewBufferString(httpPostBody))

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// Unmarshal the response body
	var executionResponse ce.ExecutionResponse
	err = json.Unmarshal(body, &executionResponse)

	assert.Nil(t, err, "No error")

	// check if executionResponse.Result contains 2
	assert.Equal(t, 0, executionResponse.HResult, "Hresult is 0")
}

func TestExecuteHelloEarthCode(t *testing.T) {
	var httpPostRequest = "http://localhost:6000/execute"
	var httpPostBody = "{ \"code\": \"print(\\\"Hello Earth\\\")\" }"

	response, err := http.Post(httpPostRequest, "application/json", bytes.NewBufferString(httpPostBody))

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// Unmarshal the response body
	var executionResponse ce.ExecutionResponse
	err = json.Unmarshal(body, &executionResponse)

	assert.Nil(t, err, "No error")
	assert.Equal(t, 0, executionResponse.HResult, "Hresult is 0")
}

func TestExecuteMatplotlibCode(t *testing.T) {
	var httpPostRequest = "http://localhost:6000/execute"
	var httpPostBody = "{ \"code\": \"import matplotlib.pyplot as plt \\nimport numpy as np \\nx = np.linspace(-2*np.pi, 2*np.pi, 1000) \\ny = np.tan(x) \\nplt.plot(x, y) \\nplt.ylim(-10, 10) \\nplt.title(\\\"Tangent Curve\\\") \\nplt.xlabel(\\\"x\\\") \\nplt.ylabel(\\\"tan(x)\\\") \\nplt.grid(True) \\nplt.show()\" }"

	response, err := http.Post(httpPostRequest, "application/json", bytes.NewBufferString(httpPostBody))

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// Unmarshal the response body
	var executionResponse ce.ExecutionResponse
	err = json.Unmarshal(body, &executionResponse)

	assert.Nil(t, err, "No error")

	// check if executionResponse.Result contains 2
	assert.Equal(t, 0, executionResponse.HResult, "Hresult is 0")
}

// We could also add a generic execute code test case, which reads it from the .py files. That way it would be much simpler
// to add code and test cases.
func TestExecuteCodeFromFile(t *testing.T) {
	var httpPostRequest = "http://localhost:6000/execute"
	// read the python file print_message.py content and pass it as code
	file, err := os.ReadFile("../e2e/files/print_message.py")
	var httpPostBody = "{ \"code\": \"" + string(file) + "\" }"

	response, err := http.Post(httpPostRequest, "application/json", bytes.NewBufferString(httpPostBody))

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	// Unmarshal the response body
	var executionResponse ce.ExecutionResponse
	err = json.Unmarshal(body, &executionResponse)

	assert.Nil(t, err, "No error")

	// check if executionResponse.Result contains 2
	assert.Equal(t, 0, executionResponse.HResult, "Hresult is 0")
}

func TestListFilesHandler(t *testing.T) {
	var httpGetRequest = "http://localhost:6000/listfiles"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	assert.Equal(t, "null", string(body), "Response body contains null")
}

func TestUploadFileHandler(t *testing.T) {
	var httpPostRequest = "http://localhost:6000/upload"
	// Open the file
	file, err := os.Open("../e2e/files/test.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Create a buffer to store our request body
	reqBody := &bytes.Buffer{}

	// Create a multipart writer
	writer := multipart.NewWriter(reqBody)

	// Create a form file writer for our file field
	formFile, err := writer.CreateFormFile("file", file.Name())
	if err != nil {
		log.Fatal(err)
	}

	// Copy the file into the form file writer
	_, err = io.Copy(formFile, file)
	if err != nil {
		log.Fatal(err)
	}

	// Close the multipart writer to finalize the body
	writer.Close()

	response, err := http.Post(httpPostRequest, writer.FormDataContentType(), reqBody)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	var metadataList []fs.FileMetadata
	err = json.Unmarshal(body, &metadataList)

	assert.Equal(t, "test.json", metadataList[0].Filename, "Filename is test.json")
	assert.Greater(t, metadataList[0].Size, int64(0), "Size is greater than 0")
}

func TestDownloadFileHandlerFileNotFound(t *testing.T) {
	var httpGetRequest = "http://localhost:6000/download/file"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	assert.Equal(t, "{\"message\": \"ERR_FILE_NOT_FOUND: File not found\"}", string(body), "Response body contains ERR_FILE_NOT_FOUND")
}

func TestDownloadFileHandlerFileFound(t *testing.T) {
	var httpGetRequest = "http://localhost:6000/download/test.json"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	assert.Equal(t, "{\r\n    \"name\": \"jquery.iframe-transport.js\",\r\n    \"url\": \"https://raw.github.com/blueimp/jQuery-File-Upload/master/js/jquery.iframe-transport.js\"\r\n}", string(body), "Response body contains file content")
}

func TestGetFileHandlerFileNotFound(t *testing.T) {
	var httpGetRequest = "http://localhost:6000/get/file"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	assert.Equal(t, "{\"message\": \"ERR_FILE_NOT_FOUND: File not found\"}", string(body), "Response body contains ERR_FILE_NOT_FOUND")
}

func TestGetFileHandlerFileFound(t *testing.T) {
	var httpGetRequest = "http://localhost:6000/get/test.json"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	var metadataList fs.FileMetadata
	err = json.Unmarshal(body, &metadataList)

	assert.Equal(t, "test.json", metadataList.Filename, "Filename is test.json")
	assert.Greater(t, metadataList.Size, int64(0), "Size is greater than 0")
}

func TestListFilesHandlerWithPathReturnsNoPath(t *testing.T) {
	var httpGetRequest = "http://localhost:6000/listfiles/wrongpath"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	assert.Equal(t, "{\"message\": \"ERR_DIR_NOT_FOUND: File path not found\"}", string(body), "Response body contains ERR_DIR_NOT_FOUND")
}

func TestListFilesHandlerListFiles(t *testing.T) {
	var httpGetRequest = "http://localhost:6000/listfiles"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	var metadataList []fs.FileMetadata
	err = json.Unmarshal(body, &metadataList)

	assert.Equal(t, "test.json", metadataList[0].Filename, "Filename is test.json")
	assert.Greater(t, metadataList[0].Size, int64(0), "Size is greater than 0")
}

func TestUploadFileHandlerWithPath(t *testing.T) {
	var httpPostRequest = "http://localhost:6000/upload/path"
	// Open the file
	file, err := os.Open("../e2e/files/file.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Create a buffer to store our request body
	reqBody := &bytes.Buffer{}

	// Create a multipart writer
	writer := multipart.NewWriter(reqBody)

	// Create a form file writer for our file field
	formFile, err := writer.CreateFormFile("file", file.Name())
	if err != nil {
		log.Fatal(err)
	}

	// Copy the file into the form file writer
	_, err = io.Copy(formFile, file)
	if err != nil {
		log.Fatal(err)
	}

	// Close the multipart writer to finalize the body
	writer.Close()

	response, err := http.Post(httpPostRequest, writer.FormDataContentType(), reqBody)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	var metadataList []fs.FileMetadata
	err = json.Unmarshal(body, &metadataList)

	assert.Equal(t, "file.txt", metadataList[0].Filename, "Filename is test.json")
	assert.Greater(t, metadataList[0].Size, int64(0), "Size is greater than 0")
}

func TestListFilesHandlerWithPath(t *testing.T) {
	var httpGetRequest = "http://localhost:6000/listfiles/path"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	var metadataList []fs.FileMetadata
	err = json.Unmarshal(body, &metadataList)

	assert.Equal(t, "file.txt", metadataList[0].Filename, "Filename is test.json")
	assert.Greater(t, metadataList[0].Size, int64(0), "Size is greater than 0")
}

func TestDownloadFileHandlerWithPath(t *testing.T) {
	var httpGetRequest = "http://localhost:6000/download/path/file.txt"
	response, err := http.Get(httpGetRequest)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	assert.Equal(t, "test", string(body), "Response body contains test")
}

// TODO: Add test to delete a file which is uploaded
func TestDeleteFileHandlerFileNotFound(t *testing.T) {
	var httpDeleteRequest = "http://localhost:6000/delete/test.json"
	request, err := http.NewRequest("DELETE", httpDeleteRequest, nil)
	response, err := http.DefaultClient.Do(request)

	// Assert no error
	assert.Nil(t, err, "No error")

	// Read the response body
	body, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "No error")

	assert.Equal(t, "{\"message\": \"file deleted successfully\"}", string(body), "Response body contains ok")
}
