package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var (
	interrupt = make(chan os.Signal, 1)
	wg        sync.WaitGroup
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(os.Stdout)
}

// define kernel and session
type Kernel struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	LastActivity   string `json:"last_activity"`
	ExecutionState string `json:"execution_state"`
	connections    int    `json:"connections"`
}

type Session struct {
	ID       string   `json:"id"`
	Path     string   `json:"path"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Kernel   Kernel   `json:"kernel"`
	Notebook Notebook `json:"notebook"`
}

type Notebook struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type ExecutionRequest struct {
	Code string `json:"code"`
}

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

// JupyterMessage represents a Jupyter message structure.
type JupyterMessage struct {
	Header       map[string]interface{} `json:"header"`
	Metadata     map[string]interface{} `json:"metadata"`
	Content      map[string]interface{} `json:"content"`
	ParentHeader map[string]interface{} `json:"parent_header"`
	Channel      string                 `json:"channel"`
	BufferPaths  []string               `json:"buffer_paths"`
}

type FileMetadata struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Filename    string    `json:"filename"` // remove this after CP change since we have name
	Size        int64     `json:"size"`
	LastModTime time.Time `json:"last_modified_time"`
	MIMEType    string    `json:"mime_type"` // remove this after CP change since we have type
}

const (
	jupyterURL               = "http://localhost:8888"
	timeout                  = 60 * time.Second
	dirPath                  = "/mnt/data"
	fileType                 = "file"
	dirType                  = "directory"
	ErrCodeFileNotFound      = "ERR_FILE_NOT_FOUND"
	ErrCodeFileAccess        = "ERR_FILE_ACCESS"
	ErrCodeSymlinkNotAllowed = "ERR_SYMLINK_NOT_ALLOWED"
)

var token = "test"
var lastCodeHealthCheck bool

func main() {
	r := mux.NewRouter()

	log.Info().Msgf("Starting Jupyter API server with token: %s", token)

	// Define your routes
	r.HandleFunc("/", initializeJupyter).Methods("GET")
	r.HandleFunc("/execute", execute).Methods("POST")

	// health check
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/listfiles", listFilesHandler).Methods("GET")
	r.HandleFunc("/listfiles/{path:.*}", listFilesHandler).Methods("GET")
	r.HandleFunc("/upload", uploadFileHandler).Methods("POST")
	r.HandleFunc("/upload/{path:.*}", uploadFileHandler).Methods("POST")
	r.HandleFunc("/download/{filename}", downloadFileHandler).Methods("GET")
	r.HandleFunc("/download/{path:.*}/{filename}", downloadFileHandler).Methods("GET")
	r.HandleFunc("/delete/{filename}", deleteFileHandler).Methods("DELETE")
	r.HandleFunc("/get/{filename}", getFileHandler).Methods("GET")

	fmt.Println("Server listening on :8080")

	// Run health check in the background
	go periodicCodeExecution()

	http.ListenAndServe(":8080", r)
}

// func to take token from the environment variable
func getToken() string {
	token = os.Getenv("JUPYTER_TOKEN")
	if token == "" {
		token = "test"
		log.Info().Msg("Token not found in environment variable, using default token %s" + token)
	}
	return token
}

// func to initialize jupyter
func initializeJupyter(w http.ResponseWriter, r *http.Request) {
	// get token from the environment variable
	token = getToken()
	checkKernels("")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Jupyter initialized with token, ` + token + `."}`))

}

// check if there are any available kernels running and if so create a new session
// return the kernelId and sessionId
func checkKernels(kernelId string) (string, string) {
	fmt.Println("Checking for available kernels...")

	url := fmt.Sprintf("%s/api/kernels?token=%s", jupyterURL, token)
	response, err := http.Get(url)
	if err != nil {
		log.Err(err).Msg("Error getting kernels")
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Err(err).Msg("Error reading response body")
	}

	var kernels []Kernel
	err = json.Unmarshal(body, &kernels)
	if err != nil {
		log.Err(err).Msg("Error unmarshaling JSON")
	}

	fmt.Println(kernels)

	var sessionId string
	// if kernel exists, respond with kernel Id
	if len(kernels) > 0 {
		fmt.Printf("Kernel ID: %s\n", kernels[0].ID)
		sessions := getSessions()

		// return the first session or the session related to the passed kernelId
		if len(sessions) > 0 {
			if kernelId != "" {
				for _, session := range sessions {
					kernelInfo := session.Kernel
					if kernelInfo.ID == kernelId {
						sessionId = session.ID
						// kernelId = kernelId --> not required since we already have the kernelId
						fmt.Printf("Session ID: %s\n", session.ID)
						break
					}
				}
			} else {
				sessionId = sessions[0].ID
				kernelId = sessions[0].Kernel.ID
			}
		}
	} else {
		newSession := createSession()
		fmt.Printf("Session ID: %s\n", newSession.ID)
		sessionId = newSession.ID
		kernelId = newSession.Kernel.ID
	}

	return kernelId, sessionId
}

// get sessions and return json object
func getSessions() []Session {
	fmt.Println("Listing available sessions:")

	url := fmt.Sprintf("%s/api/sessions?token=%s", jupyterURL, token)
	response, err := http.Get(url)
	if err != nil {
		log.Err(err).Msg("Error getting sessions")
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Err(err).Msg("Error reading response body")
	}

	var sessions []Session
	err = json.Unmarshal(body, &sessions)
	if err != nil {
		log.Err(err).Msg("Error unmarshaling JSON")
	}

	fmt.Println(sessions)

	return sessions
}

func createSession() Session {
	fmt.Println("Creating a new session...")

	// payload for POST request to create session as io.Reader value
	payload := bytes.NewBuffer([]byte(`{"path": "", "type": "notebook", "kernel": {"name": "python3"}}`))

	url := fmt.Sprintf("%s/api/sessions?token=%s", jupyterURL, token)
	response, err := http.Post(url, "application/json", payload)
	if err != nil {
		log.Err(err).Msg("Error creating session")
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Err(err).Msg("Error reading response body")
	}

	var sessionInfo Session
	err = json.Unmarshal(body, &sessionInfo)
	if err != nil {
		log.Err(err).Msg("Error unmarshaling JSON")
	}

	return sessionInfo
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if !lastCodeHealthCheck {
		http.Error(w, "Unhealthy code exec failed", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "Healthy")
}

func periodicCodeExecution() {
	time.Sleep(60 * time.Second)
	ticker := time.NewTicker(50 * time.Second)
	defer ticker.Stop()

	sampleCode := "1+1"
	for {
		select {
		case <-ticker.C:
			kernelId, sessionId := checkKernels("")
			response := executeCode(kernelId, sessionId, sampleCode)
			if response.ErrorName == "" || response.Stderr == "" {
				lastCodeHealthCheck = true
				log.Info().Msg("Periodic code execution successful")
			} else {
				lastCodeHealthCheck = false
				log.Error().Msg("Failed to execute code")
				panic("Health Ping Failed")
			}
		}
	}
}

func execute(w http.ResponseWriter, r *http.Request) {
	// read code from the request body
	code, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Err(err).Msg("Error reading request body")
	}

	// get the kernelId
	kernelId, sessionId := checkKernels("")

	// This is just for testing purposes
	// if code == nil {
	// 	// Example: Execute Python code in the created session
	// 	sampleCode := "print('Hello, Earth!')" //"import matplotlib.pyplot as plt \nimport numpy as np \nx = np.linspace(-2*np.pi, 2*np.pi, 1000) \ny = np.tan(x) \nplt.plot(x, y) \nplt.ylim(-10, 10) \nplt.title('Tangent Curve') \nplt.xlabel('x') \nplt.ylabel('tan(x)') \nplt.grid(True) \nplt.show()" //"1+3" //"print('Hello, Jupyter!')"
	// 	code = []byte(sampleCode)
	// }

	// conver the byte array to JSON and read the value for code
	var codeString ExecutionRequest
	err = json.Unmarshal(code, &codeString)
	if err != nil {
		log.Err(err).Msg("Error unmarshaling JSON")
	}

	// execute the code
	response := executeCode(kernelId, sessionId, codeString.Code)

	// return the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// convert the response to JSON and return
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		log.Err(err).Msg("Error marshaling JSON")
	}
	w.Write(jsonResponse)

}

func executeCode(kernelId, sessionId, code string) ExecutionResponse {
	fmt.Println("Executing code in the session using WebSocket:")

	responseChan := connectWebSocket(kernelId, sessionId, code)

	// Wait for response or timeout
	select {
	case response := <-responseChan:
		fmt.Println("Received response:", response)
		return response
	case <-time.After(60 * time.Second): // Timeout after 10 seconds
		fmt.Println("Timeout: No response received.")
		return ExecutionResponse{
			Hresult:      1,
			Result:       "",
			ErrorName:    "Timeout",
			ErrorMessage: "No response received",
			Stdout:       "",
			Stderr:       "",
		}
	}
}

func onMessage(message []byte) map[string]interface{} {
	fmt.Printf("Received message: %s\n", message)
	var msg map[string]interface{}
	err := json.Unmarshal(message, &msg)
	if err != nil {
		log.Err(err).Msg("Error unmarshaling JSON")
	}
	header := msg["header"].(map[string]interface{})
	msgType := header["msg_type"].(string)

	switch msgType {
	case "stream":
		content := msg["content"].(map[string]interface{})
		if content["name"].(string) == "stdout" {
			fmt.Printf("\n\nSTDOUT: %s\n", content["text"])
			// return content["text"].(string)
		} else if content["name"].(string) == "stderr" {
			fmt.Printf("\n\nSTDERR: %s\n", content["text"])
		}
		return content
	case "execute_result":
		content := msg["content"].(map[string]interface{})
		return content
	case "display_data":
		content := msg["content"].(map[string]interface{})
		return content
	case "execute_reply":
		content := msg["content"].(map[string]interface{})
		return content
	}

	return nil
}

func onError(err error) {
	log.Err(err).Msg("Error reading message")
}

func onClose() {
	log.Info().Msg("Closing WebSocket...")
	// TODO: Commenting for now, since the Wg.Done() is adding negative value to the counter even when there are no goroutines running
	// wg.Done()
}

func onOpen(ws *websocket.Conn, sessionId string, code string) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		header := createHeader("execute_request", sessionId)
		parentHeader := make(map[string]interface{})
		metadata := make(map[string]interface{})
		content := map[string]interface{}{
			"code":             code,
			"silent":           false,
			"store_history":    true,
			"user_expressions": make(map[string]interface{}),
			"allow_stdin":      false,
		}
		secret := token // Replace with the actual key
		signature := signMessage(header, parentHeader, metadata, content, secret)

		message := map[string]interface{}{
			"header":        header,
			"parent_header": parentHeader,
			"metadata":      metadata,
			"content":       content,
			"buffers":       []interface{}{},
			"signature":     signature,
		}

		err := ws.WriteJSON(message)
		if err != nil {
			log.Err(err).Msg("Error writing message")
		}
	}()

	wg.Wait()
}

// connect via websocket and execute code and return the result
func connectWebSocket(kernelID string, sessionID string, code string) <-chan ExecutionResponse {
	responseChan := make(chan ExecutionResponse)

	interruptSignal := make(chan os.Signal, 1)
	signal.Notify(interruptSignal, os.Interrupt, syscall.SIGTERM)

	u := url.URL{Scheme: "ws", Host: "localhost:8888", Path: "/api/kernels/" + kernelID + "/channels", RawQuery: "token=" + token}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Err(err).Msg("Error dialing WebSocket")
		close(responseChan)
		return responseChan
	}

	c.SetCloseHandler(func(code int, text string) error {
		log.Printf("WebSocket closed with code %d: %s\n", code, text)
		log.Info().Msgf("WebSocket closed with code %d: %s\n", code, text)
		onClose()
		return nil
	})

	c.SetPingHandler(func(appData string) error {
		log.Info().Msgf("Received ping: %s\n", appData)
		return nil
	})

	c.SetPongHandler(func(appData string) error {
		log.Info().Msgf("Received pong: %s\n", appData)
		return nil
	})

	go func() {
		defer close(responseChan)

		startTime := time.Now()
		for {
			_, message, err := c.ReadMessage()

			// print elapsed time since the start of this loop
			log.Info().Msgf("Elapsed time: %s\n", time.Since(startTime))

			// close the connection if we wait for more than timeout
			if time.Since(startTime) > timeout {
				log.Info().Msg("Timeout: No response received.")
				c.Close()
				return
			}
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					log.Err(err).Msg("Error reading message")
				}
				return
			}
			response := onMessage(message)
			if response != nil {
				result := convertToExecutionResult(response, startTime)
				responseChan <- result
				break
			}
		}
	}()

	onOpen(c, sessionID, code)

	go func() {
		select {
		case <-interruptSignal:
			log.Info().Msg("Interrupt signal received. Closing WebSocket...")
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Err(err).Msg("Error writing close message")
			}
		}
	}()

	return responseChan
}

// func to convert response to ExecutionResult
func convertToExecutionResult(response map[string]interface{}, startTime time.Time) ExecutionResponse {
	var result ExecutionResponse
	/* if response is in the format - "data": {
		"text/plain": "25"
		},
		"metadata": {},
		"execution_count": 3
	}*/
	// then Result should be "text/plain": "25"
	/* if response is in the format - {
						"name": "stdout",
	    				"text": "Hello Earth"
					}*/
	// the Stdout should be "Hello Earth" and Result should be stdout
	if response["name"] == "stdout" {
		result.Stdout = response["text"].(string)
		result.Result = "stdout"
	}
	if response["status"] == "error" {
		//result.Result = response["traceback"].([]interface{})[0].(string)
		result.ErrorName = response["ename"].(string)
		result.ErrorMessage = response["evalue"].(string)
	}
	if response["data"] != nil {
		// iterate over the data and get the value of different types of data and keep adding to the result
		for _, value := range response["data"].(map[string]interface{}) {
			result.Result += value.(string) + "; "
		}
	}

	result.DiagnosticInfo.ExecutionDuration = int(time.Since(startTime).Seconds())

	return result
}

func createHeader(msgType string, sessionId string) map[string]interface{} {
	msgID, err := uuid.NewV4()
	if err != nil {
		log.Err(err).Msg("Error generating UUID")
	}

	sessionID, err := uuid.NewV4()
	if err != nil {
		log.Err(err).Msg("Error generating UUID")
	}

	return map[string]interface{}{
		"msg_id":   msgID.String(),
		"username": "username",
		"session":  sessionID.String(),
		"msg_type": msgType,
		"version":  "5.3", // or other protocol version as needed
	}
}

func signMessage(header, parentHeader, metadata, content map[string]interface{}, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	for _, part := range []map[string]interface{}{header, parentHeader, metadata, content} {
		data, err := json.Marshal(part)
		if err != nil {
			log.Err(err).Msg("Error marshaling JSON")
		}
		h.Write(data)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func sendMessage(conn *websocket.Conn, message JupyterMessage) {
	if err := conn.WriteJSON(message); err != nil {
		log.Err(err).Msg("Error writing message")
	}
}

func receiveMessage(conn *websocket.Conn) JupyterMessage {
	var response JupyterMessage
	if err := conn.ReadJSON(&response); err != nil {
		log.Err(err).Msg("Error reading message")
	}

	return response
}

func listFilesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetPath := dirPath

	// supports both listFiles and listFiles/{path}
	if customPath, ok := vars["path"]; ok && customPath != "" {
		// clean the path to prevent directory traversal attacks
		customPath = filepath.Clean("/" + customPath)
		targetPath = filepath.Join(dirPath, customPath)
	}

	files, err := os.ReadDir(targetPath)
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

		fullPath := filepath.Join(targetPath, f.Name())
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			log.Error().Err(err).Str("file", f.Name()).Msg("Unable to get file info")
			continue
		}

		mimeType := mime.TypeByExtension(filepath.Ext(f.Name()))
		if mimeType == "" {
			mimeType = "application/octet-stream" // default MIME type
		}

		if fileInfo.IsDir() {
			metadataList = append(metadataList, FileMetadata{
				Name:        f.Name(),
				Type:        dirType,
				Filename:    f.Name(), // remove this after CP change since we have Name
				Size:        fileInfo.Size(),
				LastModTime: fileInfo.ModTime(),
				MIMEType:    mimeType, // remove this after CP change since we have type
			})
		} else {
			metadataList = append(metadataList, FileMetadata{
				Name:        f.Name(),
				Type:        fileType,
				Filename:    f.Name(), // remove this after CP change since we have Name
				Size:        fileInfo.Size(),
				LastModTime: fileInfo.ModTime(),
				MIMEType:    mimeType, // remove this after CP change since we have type
			})
		}
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

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	// get custom path from URL
	vars := mux.Vars(r)
	targetPath := dirPath

	// supports both uploadFile and uploadFile/{path}
	if customPath, ok := vars["path"]; ok && customPath != "" {
		// clean the path to prevent directory traversal attacks
		customPath = filepath.Clean("/" + customPath)
		targetPath = filepath.Join(dirPath, customPath)
	}

	err := r.ParseMultipartForm(250 << 20) // 250MB limit
	if err != nil {
		log.Error().Err(err).Msg("Unable to parse form")
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["file"]
	var metadataList []FileMetadata

	for _, file := range files {
		if err := processFile(file, &metadataList, targetPath); err != nil {
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
func processFile(file *multipart.FileHeader, metadataList *[]FileMetadata, path string) error {
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

	// create the directory if it doesn't exist
	os.MkdirAll(path, os.ModePerm)

	dstPath := filepath.Join(path, filepath.Base(file.Filename))
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

	targetPath := dirPath
	// supports both dowloadFile and dowloadFile/{path}/{fileName}
	if customPath, ok := vars["path"]; ok && customPath != "" {
		// clean the path to prevent directory traversal attacks
		customPath = filepath.Clean("/" + customPath)
		targetPath = filepath.Join(dirPath, customPath)
	}

	filePath := filepath.Join(targetPath, filename)

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

func logAndRespond(w http.ResponseWriter, statusCode int, errCode, errMsg string) {
	log.Error().Str("error_code", errCode).Msg(errMsg)
	http.Error(w, fmt.Sprintf("%s: %s", errCode, errMsg), statusCode)
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

	// Check if the file exists
	_, err = os.Lstat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			logAndRespond(w, http.StatusNotFound, ErrCodeFileNotFound, "File not found")
		} else {
			logAndRespond(w, http.StatusInternalServerError, ErrCodeFileAccess, "Error accessing file")
		}
		return
	}

	// File exists, proceed with deletion
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

	// if file exists, retrieve file information using os.Stat
	fileInfo, err := os.Lstat(filePath)
	// handle not found or other errors
	if err != nil {
		if os.IsNotExist(err) {
			logAndRespond(w, http.StatusNotFound, ErrCodeFileNotFound, "File not found")
		} else {
			logAndRespond(w, http.StatusInternalServerError, ErrCodeFileAccess, "Error accessing file")
		}
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
