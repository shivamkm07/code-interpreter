package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	computeResourceKey = ""
)

const (
	ErrCodeParseForm         = "ERR_PARSE_FORM"
	ErrCodeFileOpen          = "ERR_FILE_OPEN"
	ErrCodeFileCreate        = "ERR_FILE_CREATE"
	ErrCodeFileWrite         = "ERR_FILE_WRITE"
	ErrCodeFileInfo          = "ERR_FILE_INFO"
	ErrCodeMarshalResponse   = "ERR_MARSHAL_RESPONSE"
	ErrCodeFileNotFound      = "ERR_FILE_NOT_FOUND"
	ErrCodeFileAccess        = "ERR_FILE_ACCESS"
	ErrCodeSymlinkNotAllowed = "ERR_SYMLINK_NOT_ALLOWED"
	apiEndpoint              = "http://127.0.0.1:80/api/runtimes"
	maxRetries               = 5
	initialRetryDelay        = 2 * time.Second
	apiEndpointFormat        = "http://localhost:80/api/runtimes/%s/%s"
	executeEndpoint          = "http://127.0.0.1:80/api/runtimes/%v/execute"
	dirPath                  = "/mnt/data"
)

type AcaPoolPythonRuntimesResponse struct {
	Items []AcaPoolPythonRuntimeResponse `json:"items"`
}

type AcaPoolPythonRuntimeResponse struct {
	Id            string `json:"id"`
	EnvironmentId string `json:"environmentId"`
}

type FileMetadata struct {
	Filename    string    `json:"filename"`
	Size        int64     `json:"size"`
	LastModTime time.Time `json:"last_modified_time"`
	MIMEType    string    `json:"mime_type"`
}

type PythonRuntimesResponse struct {
	Items []PythonRuntime `json:"items"`
}

type PythonRuntime struct {
	Id            string `json:"id"`
	EnvironmentId string `json:"environmentId"`
}

var (
	cachedRuntimeID     string
	runtimeMutex        sync.Mutex
	codeExecMutex       sync.Mutex
	lastCodeHealthCheck bool
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(os.Stdout)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	//runtimeID, err := getRuntimeID("/computeresourcekey123", false)
	//if err != nil {
	//	log.Error().Err(err).Msg("Health check failed")
	//	http.Error(w, "Unhealthy", http.StatusInternalServerError)
	//	return
	//}

	if !lastCodeHealthCheck {
		http.Error(w, "Unhealthy code exec failed", http.StatusInternalServerError)
		return
	}

	//log.Info().Str("RuntimeID", runtimeID).Msg("Health check passed")
	fmt.Fprintln(w, "Healthy")

}

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(250 << 20) // 250MB limit
	if err != nil {
		log.Error().Err(err).Msg("Unable to parse form")
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["file"]
	var metadataList []FileMetadata

	for _, file := range files {
		if err := processFile(file, &metadataList); err != nil {
			log.Error().Err(err).Str("filename", file.Filename).Send()
			// choose to continue?
		}
	}

	response, err := json.Marshal(metadataList)
	if err != nil {
		log.Error().Err(err).Msg("Unable to marshal response")
		http.Error(w, "Unable to marshal response", http.StatusInternalServerError)
		return
	}

	log.Info().Msg("Upload files successfully.\n")
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

// processFile handles the processing of each individual file and updates the metadata list.
func processFile(file *multipart.FileHeader, metadataList *[]FileMetadata) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	// url decode filename
	decodedFilename, err := url.QueryUnescape(file.Filename)
	if err != nil {
		log.Error().Err(err).Str("filename", file.Filename).Msg("Error decoding file name")
	}
	file.Filename = decodedFilename

	dstPath := filepath.Join(dirPath, filepath.Base(file.Filename))
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	if fileInfo, err := dst.Stat(); err == nil {
		*metadataList = append(*metadataList, FileMetadata{
			Filename:    file.Filename,
			Size:        fileInfo.Size(),
			LastModTime: fileInfo.ModTime(),
		})
	} else {
		return err
	}

	if err := os.Chmod(dstPath, 0777); err != nil {
		return err
	}

	return nil
}

func downloadFileHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
	encodedFilename := vars["filename"]

	// URL decode the filename
	decodedFilename, err := url.QueryUnescape(encodedFilename)
	if err != nil {
		log.Error().Err(err).Msg("Error decoding file name")
		http.Error(w, "Error decoding file name", http.StatusBadRequest)
		return
	}

	// Use the decoded filename for further processing
	filename := filepath.Base(decodedFilename)
	filePath := filepath.Join(dirPath, filename)

	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			logAndRespond(w, http.StatusNotFound, ErrCodeFileNotFound, "File not found")
		} else {
			logAndRespond(w, http.StatusInternalServerError, ErrCodeFileAccess, "Error accessing file")
		}
		return
	}

	if fileInfo.Mode()&os.ModeSymlink != 0 {
		logAndRespond(w, http.StatusBadRequest, ErrCodeSymlinkNotAllowed, "Symlinks not allowed")
		return
	}

	http.ServeFile(w, r, filePath)
}

func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	encodedFilename := vars["filename"]

	// URL decode the filename
	decodedFilename, err := url.QueryUnescape(encodedFilename)
	if err != nil {
		log.Error().Err(err).Msg("Error decoding file name")
		http.Error(w, "Error decoding file name", http.StatusBadRequest)
		return
	}

	// Use the decoded filename in further processing
	filename := filepath.Base(decodedFilename)
	filePath := filepath.Join(dirPath, filename)

	err = os.Remove(filePath)
	if err != nil {
		log.Error().Err(err).Msg(fmt.Sprintf("Error deleting file %s", filename))
		http.Error(w, "Error deleting file", http.StatusInternalServerError)
		return
	}

	log.Info().Msg(fmt.Sprintf("File %s deleted successfully.\n", filename))
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func getFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	encodedFilename := vars["filename"]

	// URL decode the filename
	decodedFilename, err := url.QueryUnescape(encodedFilename)
	if err != nil {
		log.Error().Err(err).Msg("Error decoding file name")
		http.Error(w, "Error decoding file name", http.StatusBadRequest)
		return
	}

	// Use the decoded filename in further processing
	filename := filepath.Base(decodedFilename)
	filePath := filepath.Join(dirPath, filename)

	// Retrieve file information using os.Stat
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		// If there is an error, send an error response
		log.Error().Err(err).Str("file", filename).Msg("Unable to get file info")
		http.Error(w, fmt.Sprintf("Error getting file information: %v", err), http.StatusInternalServerError)
		return
	}

	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	if mimeType == "" {
		mimeType = "application/octet-stream" // default MIME type
	}

	fileMetadata := FileMetadata{
		Filename:    filename,
		Size:        fileInfo.Size(),
		LastModTime: fileInfo.ModTime(),
		MIMEType:    mimeType,
	}

	response, err := json.Marshal(fileMetadata)
	if err != nil {
		log.Error().Err(err).Msg("Unable to marshal response")
		http.Error(w, "Unable to marshal response", http.StatusInternalServerError)
		return
	}

	log.Info().Msg(fmt.Sprintf("Get file %s successfully.\n", filename))
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func listFilesHandler(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		log.Error().Err(err).Msg("Unable to read directory")
		http.Error(w, "Unable to read directory", http.StatusInternalServerError)
		return
	}

	var metadataList []FileMetadata
	for _, f := range files {
		// Ignore if it is a symlink
		if f.Type()&os.ModeSymlink != 0 {
			continue
		}

		fullPath := filepath.Join(dirPath, f.Name())
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			log.Error().Err(err).Str("file", f.Name()).Msg("Unable to get file info")
			continue
		}

		mimeType := mime.TypeByExtension(filepath.Ext(f.Name()))
		if mimeType == "" {
			mimeType = "application/octet-stream" // default MIME type
		}

		metadataList = append(metadataList, FileMetadata{
			Filename:    f.Name(),
			Size:        fileInfo.Size(),
			LastModTime: fileInfo.ModTime(),
			MIMEType:    mimeType,
		})
	}

	response, err := json.Marshal(metadataList)
	if err != nil {
		log.Error().Err(err).Msg("Unable to marshal response")
		http.Error(w, "Unable to marshal response", http.StatusInternalServerError)
		return
	}

	log.Info().Msg("List files successfully.\n")
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	runtimeID, err := getRuntimeID(computeResourceKey, false)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get runtime ID")
		http.Error(w, "Failed to get runtime ID", http.StatusInternalServerError)
		return
	}

	path := mux.Vars(r)["path"]
	destURL := fmt.Sprintf(apiEndpointFormat, runtimeID, path)
	proxyURL, err := url.Parse(destURL)
	if err != nil {
		log.Error().Err(err).Str("url", destURL).Msg("Failed to parse destination URL")
		http.Error(w, "Failed to parse destination URL", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(proxyURL)
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, e error) {
		log.Error().Err(e).Msg("Proxy error")
		http.Error(rw, "Error in proxying request", http.StatusInternalServerError)
	}

	proxy.Director = func(req *http.Request) {
		req.URL = proxyURL
		req.Header.Add("Authorization", "APIKey "+computeResourceKey)
		req.Host = proxyURL.Host
		log.Info().Str("URL", req.URL.String()).Msg("Proxying request")
	}

	proxy.ServeHTTP(w, r)
}

func ExecuteCode(runtimeId string, code string) error {
	url := fmt.Sprintf(executeEndpoint, runtimeId)

	payload := fmt.Sprintf(`{"code": "%s"}`, code)
	reqBody := bytes.NewBufferString(payload)

	req, err := http.NewRequest("POST", url, reqBody)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create HTTP request")
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Authorization", "APIKey "+computeResourceKey)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send HTTP request")
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read HTTP response")
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		log.Error().Int("StatusCode", resp.StatusCode).Str("ResponseBody", string(body)).Msg("Error response received")
		return fmt.Errorf("received error status code %d: %s", resp.StatusCode, string(body))
	}

	log.Info().Str("ResponseBody", string(body)).Msg("Successfully executed code")
	return nil
}

func periodicCodeExecution(apiKey string) {
	time.Sleep(60 * time.Second)
	ticker := time.NewTicker(50 * time.Second)
	defer ticker.Stop()

	sampleCode := "1+1"
	for {
		select {
		case <-ticker.C:
			runtimeID, err := getRuntimeID(apiKey, false)
			if err != nil {
				log.Error().Err(err).Msg("Failed to get runtime ID for code execution")
				panic("Can't get runtime id")
			}

			err = ExecuteCode(runtimeID, sampleCode)
			if err != nil {
				lastCodeHealthCheck = false
				log.Error().Err(err).Msg("Failed to execute code")
				panic("Health Ping Failed")
			} else {
				log.Info().Msg("Periodic code execution successful")
			}
		}
	}
}

func getRuntimeID(apiKey string, forceRefresh bool) (string, error) {
	log.Info().Msg("GetRuntimeID")
	runtimeMutex.Lock()
	defer runtimeMutex.Unlock()

	if cachedRuntimeID != "" && !forceRefresh {
		return cachedRuntimeID, nil
	}

	return fetchRuntimeID(apiKey)
}

func fetchRuntimeID(apiKey string) (string, error) {
	retryDelay := initialRetryDelay

	for i := 0; i < maxRetries; i++ {
		log.Info().Int("Attempt", i+1).Msg("Fetching Runtime ID")
		runtimeID, err := attemptFetchRuntimeID(apiKey)
		if err == nil {
			return runtimeID, nil
		}

		log.Error().Err(err).Int("Attempt", i+1).Msg("Failed to fetch Runtime ID")
		time.Sleep(retryDelay)
		retryDelay *= 2
	}
	return "", errors.New("Maximum retries reached for getRuntimeID")
}

func attemptFetchRuntimeID(apiKey string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", apiEndpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", fmt.Sprintf("apikey %s", apiKey))

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("non-OK HTTP status: %d", resp.StatusCode)
	}

	var runtimes AcaPoolPythonRuntimesResponse
	if err := json.NewDecoder(resp.Body).Decode(&runtimes); err != nil {
		return "", err
	}

	if len(runtimes.Items) == 0 {
		return createRuntime(apiKey, client)
	}

	cachedRuntimeID = runtimes.Items[0].Id
	return cachedRuntimeID, nil
}

func createRuntime(apiKey string, client *http.Client) (string, error) {
	createRuntimeURL := "http://127.0.0.1:80/api/runtimes"

	requestData := struct{}{}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal request data")
	}

	req, err := http.NewRequest("POST", createRuntimeURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", errors.Wrap(err, "failed to create POST request")
	}

	req.Header.Add("Authorization", fmt.Sprintf("apikey %s", apiKey))
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to send POST request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to create runtime, status code: %d", resp.StatusCode)
	}

	var runtimeResponse AcaPoolPythonRuntimeResponse
	if err := json.NewDecoder(resp.Body).Decode(&runtimeResponse); err != nil {
		return "", errors.Wrap(err, "failed to decode response")
	}

	return runtimeResponse.Id, nil
}

func logAndRespond(w http.ResponseWriter, statusCode int, errCode, errMsg string) {
	log.Error().Str("error_code", errCode).Msg(errMsg)
	http.Error(w, fmt.Sprintf("%s: %s", errCode, errMsg), statusCode)
}

func main() {
	log.Info().Msg("Application starting up")

	computeResourceKey = os.Getenv("OfficePy__ComputeResourceKey")

	if computeResourceKey == "" {
		computeResourceKey = "/acasessions"
	} else {
		computeResourceKey = "/" + computeResourceKey
	}

	log.Info().Str("Compute Resource Key", computeResourceKey).Msg("Logging compute resource key")
	lastCodeHealthCheck = true

	router := mux.NewRouter()

	router.HandleFunc("/healthz", healthHandler).Methods("GET")
	router.HandleFunc("/listfiles", listFilesHandler).Methods("GET")
	router.HandleFunc("/upload", uploadFileHandler).Methods("POST")
	router.HandleFunc("/download/{filename}", downloadFileHandler).Methods("GET")
	router.HandleFunc("/delete/{filename}", deleteFileHandler).Methods("DELETE")
	router.HandleFunc("/get/{filename}", getFileHandler).Methods("GET")
	router.PathPrefix("/{path:.*}").HandlerFunc(proxyHandler)

	go periodicCodeExecution(computeResourceKey)

	log.Info().Msg("Starting server on port :6000")
	http.ListenAndServe(":6000", router)
}
