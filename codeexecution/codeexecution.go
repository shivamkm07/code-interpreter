package codeexecution

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	"github.com/microsoft/jupyterpython/jupyterservices"
	"github.com/rs/zerolog/log"
)

var (
	interrupt = make(chan os.Signal, 1)
	wg        sync.WaitGroup
)

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

func Execute(w http.ResponseWriter, r *http.Request) {
	// read code from the request body
	code, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Err(err).Msg("Error reading request body")
	}

	// get the kernelId
	kernelId, sessionId := jupyterservices.CheckKernels("")

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
	response := ExecuteCode(kernelId, sessionId, codeString.Code)

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

func ExecuteCode(kernelId, sessionId, code string) ExecutionResponse {
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
		secret := jupyterservices.Token // Replace with the actual key
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

	u := url.URL{Scheme: "ws", Host: "localhost:8888", Path: "/api/kernels/" + kernelID + "/channels", RawQuery: "token=" + jupyterservices.Token}
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
			if time.Since(startTime) > jupyterservices.Timeout {
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
