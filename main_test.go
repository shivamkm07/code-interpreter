package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/jupyterpython/fileservices"
	"github.com/microsoft/jupyterpython/jupyterservices"
)

func TestListFilesHandler(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create some test files and directories
	testFiles := []string{"file1.txt", "file2.txt", "file3.txt"}
	testDirs := []string{"dir1", "dir2", "dir3"}

	for _, file := range testFiles {
		filePath := filepath.Join(tempDir, file)
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	for _, dir := range testDirs {
		dirPath := filepath.Join(tempDir, dir)
		err := os.Mkdir(dirPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
	}

	// Create a mock HTTP request and response
	req := httptest.NewRequest("GET", "/listFiles", nil)
	w := httptest.NewRecorder()

	// Call the handler function
	fileservices.ListFilesHandler(w, req)

	// Check the response status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// Parse the response body
	var metadataList []fileservices.FileMetadata
	err := json.Unmarshal(w.Body.Bytes(), &metadataList)
	if err != nil {
		t.Fatalf("Failed to parse response body: %v", err)
	}

	// Check the number of files and directories in the response
	expectedNumFiles := len(testFiles)
	expectedNumDirs := len(testDirs)
	if len(metadataList) != expectedNumFiles+expectedNumDirs {
		t.Errorf("Expected %d files and %d directories, got %d items", expectedNumFiles, expectedNumDirs, len(metadataList))
	}

	// Check the file metadata in the response
	for _, file := range testFiles {
		found := false
		for _, metadata := range metadataList {
			if metadata.Name == file && metadata.Type == fileservices.FileType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected file %s not found in response", file)
		}
	}

	// Check the directory metadata in the response
	for _, dir := range testDirs {
		found := false
		for _, metadata := range metadataList {
			if metadata.Name == dir && metadata.Type == fileservices.DirType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected directory %s not found in response", dir)
		}
	}
}

func TestCheckKernels(t *testing.T) {
	// Mock the HTTP response
	mockResponse := `[
		{
			"id": "kernel1",
			"name": "Python 3",
			"language": "python",
			"execution_state": "idle"
		},
		{
			"id": "kernel2",
			"name": "Go",
			"language": "go",
			"execution_state": "busy"
		}
	]`
	mockURL := "http://example.com/api/kernels?token=12345"
	mockTransportVar := &mockTransport{response: mockResponse}
	// http.DefaultClient.Transport = mockTransportVar
	originalTransport := http.DefaultTransport
	defer func() { http.DefaultTransport = originalTransport }()

	http.DefaultTransport = mockTransportVar

	// Call the function under test
	kernelId, sessionId, err := jupyterservices.CheckKernels("kernel1")
	if err != nil {
		t.Fatalf("CheckKernels returned an error: %v", err)
	}

	// Check the returned values
	expectedKernelId := "kernel1"
	expectedSessionId := "session1"
	if kernelId != expectedKernelId {
		t.Errorf("Expected kernel ID %s, got %s", expectedKernelId, kernelId)
	}
	if sessionId != expectedSessionId {
		t.Errorf("Expected session ID %s, got %s", expectedSessionId, sessionId)
	}

	// Check the HTTP request
	if mockTransportVar.requestURL != mockURL {
		t.Errorf("Expected request URL %s, got %s", mockURL, mockTransportVar.requestURL)
	}
}

// MockTransport is a custom http.RoundTripper implementation for mocking HTTP requests
type mockTransport struct {
	response    string
	requestURL  string
	requestBody []byte
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.requestURL = req.URL.String()
	if req.Body != nil {
		t.requestBody, _ = io.ReadAll(req.Body)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(t.response)),
	}, nil
}
