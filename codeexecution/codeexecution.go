// Copyright 2023 Microsoft Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package codeexecution

import (
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/microsoft/jupyterpython/util"
	"github.com/rs/zerolog/log"
)

var (
	interrupt    = make(chan os.Signal, 1)
	ws           *websocket.Conn
	requestMsgID string
)

type ExecutionRequest struct {
	Code string `json:"code"`
}

// struct to convert GenericMessage to ExecutionPlainTextResult
type ExecutePlainTextResult struct {
	Success                       bool                            `json:"success"`
	ErrorCode                     ExecutePlainTextResultErrorCode `json:"errorCode"`
	TextPlain                     string                          `json:"textPlain,omitempty"`
	TextOfficePy                  string                          `json:"textOfficePy,omitempty"`
	ImageBase64Data               string                          `json:"imageBase64Data,omitempty"`
	ErrorName                     string                          `json:"errorName,omitempty"`
	ErrorMessage                  string                          `json:"errorMessage,omitempty"`
	ErrorTraceback                string                          `json:"errorTraceback,omitempty"`
	Stderr                        string                          `json:"stderr,omitempty"`
	Stdout                        string                          `json:"stdout,omitempty"`
	ExecutionDurationMilliseconds int                             `json:"executionDurationMilliseconds"`
}

// Final result of the execution to be returned
type ExecutionResponse struct {
	HResult         int                       `json:"hresult"`
	Result          *json.RawMessage          `json:"result"`
	ErrorName       string                    `json:"error_name"`
	ErrorMessage    string                    `json:"error_message"`
	ErrorStackTrace string                    `json:"error_stack_trace"`
	Stdout          string                    `json:"stdout"`
	Stderr          string                    `json:"stderr"`
	DiagnosticInfo  ExecuteCodeDiagnosticInfo `json:"diagnosticInfo"`
	//ServiceData     *json.RawMessage          `json:"serviceData"`
	ApproximateSize int `json:"-"`
}

type ExecuteCodeDiagnosticInfo struct {
	ExecutionDuration int `json:"executionDuration"`
	//MessageId         string `json:"messageId"`
}

var lock sync.Mutex

func Execute(w http.ResponseWriter, r *http.Request) {
	// read code from the request body
	lock.Lock()
	defer lock.Unlock()
	code, err := io.ReadAll(r.Body)
	if err != nil {
		log.Err(err).Msg("Error reading request body")
		util.SendHTTPResponse(w, http.StatusInternalServerError, "error reading request body"+err.Error(), true)
	}

	// get the kernelId
	kernelId, sessionId, err := jupyterservices.CheckKernels("")
	if err != nil {
		log.Err(err).Msg("Error checking kernels")
		util.SendHTTPResponse(w, http.StatusInternalServerError, "error checking kernels"+err.Error(), true)
	}

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
		util.SendHTTPResponse(w, http.StatusInternalServerError, "error unmarshaling JSON"+err.Error(), true)
	}

	// execute the code
	response := ExecuteCode(kernelId, sessionId, codeString.Code)

	// convert the response to JSON and return
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		log.Err(err).Msg("Error marshaling JSON")
		util.SendHTTPResponse(w, http.StatusInternalServerError, "error marshaling JSON"+err.Error(), true)
	}
	util.SendHTTPResponse(w, http.StatusOK, string(jsonResponse), false)
}

func ExecuteCode(kernelId, sessionId, code string) ExecutionResponse {
	fmt.Println("Executing code in the session using WebSocket:")

	responseChan := connectWebSocket(kernelId, sessionId, code)

	// select to timeout if no response is received in 60 seconds, else return the response
	select {
	case <-time.After(jupyterservices.Timeout):
		fmt.Println("Timeout: No response received.")
		return ExecutionResponse{
			HResult:      1,
			Result:       nil,
			ErrorName:    "Timeout",
			ErrorMessage: "No response received",
			Stdout:       "",
			Stderr:       "",
		}
	case response := <-responseChan:
		fmt.Println("Received response:", response)
		return response
	}
}

func onMessage(message []byte) *ExecutePlainTextResult {
	var msg map[string]interface{}
	err := json.Unmarshal(message, &msg)
	if err != nil {
		log.Err(err).Msg("Error unmarshaling JSON")
	}

	// get parent_header from the message we got over websocket
	parentHeader := msg["parent_header"].(map[string]interface{})
	msgID := parentHeader["msg_id"].(string)

	if msgID != requestMsgID {
		return nil
	}

	// if current message Id matches the Id of the meessage we sent then process the message
	m := HandleAndProcessMessage(message, msgID)

	// if ExecuteResultAlreadySet is false then return nil
	if !m.ExecuteResultAlreadySet {
		return nil
	}

	return &m.ExecuteResult
}

func onClose() {
	log.Info().Msg("Closing WebSocket...")
	if ws != nil {
		ws.Close()
		ws = nil
	}
}

func onOpen(ws *websocket.Conn, sessionId string, code string) []byte {
	var jsonMessage []byte

	go func() {
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

		message := map[string]interface{}{
			"header":        header,
			"parent_header": parentHeader,
			"metadata":      metadata,
			"content":       content,
			"buffers":       []interface{}{},
		}

		// print the message in JSON format
		jsonMessage, _ = json.Marshal(message)
		fmt.Printf("JSON message: %s\n", jsonMessage)

		err := ws.WriteJSON(message)
		if err != nil {
			log.Err(err).Msg("Error writing message")
		}
	}()

	return jsonMessage
}

// connect via websocket and execute code and return the result
func connectWebSocket(kernelID string, sessionID string, code string) <-chan ExecutionResponse {
	responseChan := make(chan ExecutionResponse)

	interruptSignal := make(chan os.Signal)
	signal.Notify(interruptSignal, os.Interrupt, syscall.SIGTERM)

	u := url.URL{Scheme: "ws", Host: "localhost:8888", Path: "/api/kernels/" + kernelID + "/channels", RawQuery: "token=" + jupyterservices.Token}
	err := error(nil)
	if ws == nil {
		ws, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			log.Err(err).Msg("Error dialing WebSocket")
			close(responseChan)
			return responseChan
		}
		fmt.Printf("Connected to WebSocket %s\n", ws.RemoteAddr())
	}

	ws.SetCloseHandler(func(code int, text string) error {
		fmt.Println("WebSocket closed with code", code, text)
		log.Info().Msgf("WebSocket closed with code %d: %s\n", code, text)
		onClose()
		return err
	})

	ws.SetPingHandler(func(appData string) error {
		log.Info().Msgf("Received ping: %s\n", appData)
		return ws.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
	})

	ws.SetPongHandler(func(appData string) error {
		log.Info().Msgf("Received pong: %s\n", appData)
		return nil
	})

	go func() {
		defer close(responseChan)

		startTime := time.Now()
		for {
			// ws is nil if the connection is closed
			if ws == nil {
				return
			}

			_, message, err := ws.ReadMessage()
			if err != nil {
				fmt.Println("Error reading message:", err)
				log.Err(err).Msg("Error reading message")
				ws = nil
				return
			}
			parsedMessage := make(map[string]interface{})
			json.Unmarshal(message, &parsedMessage)

			// print elapsed time since the start of this loop
			log.Info().Msgf("Elapsed time: %s\n", time.Since(startTime))

			// close the connection if we wait for more than timeout
			if time.Since(startTime) > jupyterservices.Timeout {
				log.Info().Msg("Timeout: No response received.")
				if ws != nil {
					ws.Close()
					ws = nil
				}
				break
			}
			response := onMessage(message)
			if response != nil {
				result := ConvertJupyterPlainResultToExecuteCodeResult(*response, startTime)
				responseChan <- result
				break
			}
		}
	}()

	onOpen(ws, sessionID, code)

	go func() {
		select {
		case <-interruptSignal:
			log.Info().Msg("Interrupt signal received. Closing WebSocket...")
			err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Err(err).Msg("Error writing close message")
			}
		}
	}()

	return responseChan
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

	requestMsgID = msgID.String()
	return map[string]interface{}{
		"msg_id":   msgID.String(),
		"username": "username",
		"session":  sessionID.String(),
		"msg_type": msgType,
		"version":  "5.3", // or other protocol version as needed
	}
}
