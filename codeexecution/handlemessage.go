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
	"strings"
	"time"
	"unicode/utf8"
)

type ExecutePlainTextResultErrorCode int

const (
	Success ExecutePlainTextResultErrorCode = iota
	Generic
	KernelRestarted
	ExecutionAborted
)

type MessageHeader struct {
	MsgId   string `json:"msg_id"`
	MsgType string `json:"msg_type"`
	Version string `json:"version"`
}

type ExecuteMessageContent struct {
	Code            string                 `json:"code"`
	Silent          bool                   `json:"silent"`
	StoreHistory    bool                   `json:"store_history"`
	AllowStdin      bool                   `json:"allow_stdin"`
	StopOnError     bool                   `json:"stop_on_error"`
	UserExpressions map[string]interface{} `json:"user_expressions"`
}

type ExecuteMessage struct {
	Header       MessageHeader          `json:"header"`
	MsgId        string                 `json:"msg_id"`
	MsgType      string                 `json:"msg_type"`
	Metadata     map[string]interface{} `json:"metadata"`
	ParentHeader MessageHeader          `json:"parent_header"`
	Content      ExecuteMessageContent  `json:"content"`
	Channel      string                 `json:"channel"`
}

type GenericMessageContentData struct {
	TextPlain    string `json:"text/plain"`
	TextOfficePy string `json:"text/officepy"`
	ImagePng     string `json:"image/png"`
}

type GenericMessageContent struct {
	ErrorName      string                    `json:"ename"`
	ErrorValue     string                    `json:"evalue"`
	Traceback      []string                  `json:"traceback"`
	Name           string                    `json:"name"`
	Text           string                    `json:"text"`
	Status         string                    `json:"status"`
	Data           GenericMessageContentData `json:"data"`
	ExecutionState string                    `json:"execution_state"`
}

type GenericMessage struct {
	Header       MessageHeader          `json:"header"`
	MsgId        string                 `json:"msg_id"`
	MsgType      string                 `json:"msg_type"`
	Metadata     map[string]interface{} `json:"metadata"`
	ParentHeader MessageHeader          `json:"parent_header"`
	Content      *GenericMessageContent `json:"content"`
	Channel      string                 `json:"channel"`
}

type ExecuteResultAndTaskCompleteSource struct {
	ExecuteResult           ExecutePlainTextResult
	TaskCompletionSource    *ExecutePlainTextResult
	ExecuteResultAlreadySet bool
}

type NotebookClientOptions struct {
	Url                    string
	Token                  string
	MaxStdoutMessageLength int
	IdleTimeout            time.Duration
}

func NewNotebookClientOptions() *NotebookClientOptions {
	return &NotebookClientOptions{
		Url:                    "http://localhost",
		Token:                  "",
		MaxStdoutMessageLength: 1024,
		IdleTimeout:            time.Minute * 30,
	}
}

func NewExecuteResultAndTaskCompleteSource() *ExecuteResultAndTaskCompleteSource {
	return &ExecuteResultAndTaskCompleteSource{
		ExecuteResult:        ExecutePlainTextResult{},
		TaskCompletionSource: &ExecutePlainTextResult{},
	}
}

// map to store the execute task complete source with message id
var m_executeTaskCompleteSourceDict = make(map[string]*ExecuteResultAndTaskCompleteSource)

// handle messages from jupyter and pass the executeTaskCompleteSource to the task completion source
func HandleAndProcessMessage(jsonMessage []byte, msg_id string) *ExecuteResultAndTaskCompleteSource {
	var message GenericMessage
	var returnMessage *ExecuteResultAndTaskCompleteSource

	err := json.Unmarshal(jsonMessage, &message)
	if err != nil {
		fmt.Println("Error unmarshalling message: ", err)
		return nil
	}

	// add to the dictionary
	if _, ok := m_executeTaskCompleteSourceDict[msg_id]; !ok {
		m_executeTaskCompleteSourceDict[message.ParentHeader.MsgId] = NewExecuteResultAndTaskCompleteSource()
	}

	ConvertToExecutionResponse(message)

	// remove the dictionary entry if the task is already set
	if executeResultAndTaskCompleteSource, ok := m_executeTaskCompleteSourceDict[msg_id]; ok {
		returnMessage = executeResultAndTaskCompleteSource
		if executeResultAndTaskCompleteSource.ExecuteResultAlreadySet {
			delete(m_executeTaskCompleteSourceDict, msg_id)
		}
	}

	return returnMessage
}

// function to take generic message and convert to ExecutionResponse based on message type
// Cases:
// - execute_request
// - execute_reply
// - execute_result
// - display_data
// - error
// - status
// - stream
func ConvertToExecutionResponse(message GenericMessage) {
	fmt.Println("Message Type: ", message.MsgType)
	switch message.MsgType {
	case "execute_reply":
		HandleMessage_ExecuteReply(message)
	case "execute_result":
		handleExecuteResult(message)
	case "display_data":
		handleDisplayData(message)
	case "error":
		HandleMessage_Error(message)
	case "status":
		handleStatus(message)
	case "stream":
		handleStream(message)
	}
}

// handle execute_reply
func HandleMessage_ExecuteReply(message GenericMessage) {
	msgId := message.ParentHeader.MsgId
	if msgId == "" {
		return
	}

	if message.Content == nil {
		return
	}

	status := message.Content.Status
	if status == "" {
		return
	}

	if status == "aborted" {
		if executeResultAndTaskCompleteSource, ok := m_executeTaskCompleteSourceDict[msgId]; ok {
			executeResultAndTaskCompleteSource.ExecuteResult.Success = false
			executeResultAndTaskCompleteSource.ExecuteResult.ErrorCode = ExecutionAborted
			SetExecuteTaskComplete(executeResultAndTaskCompleteSource)
		}
	}
}

// handle execute_result
func handleExecuteResult(message GenericMessage) {
	msgId := message.ParentHeader.MsgId
	if msgId == "" {
		return
	}

	executeResultAndTaskCompleteSource, ok := m_executeTaskCompleteSourceDict[msgId]
	if !ok {
		return
	}

	if message.Content != nil {
		data := message.Content.Data
		if data != (GenericMessageContentData{}) {
			executeResultAndTaskCompleteSource.ExecuteResult.TextOfficePy = data.TextOfficePy
			executeResultAndTaskCompleteSource.ExecuteResult.TextPlain = data.TextPlain
		}
	}

	executeResultAndTaskCompleteSource.ExecuteResult.Success = true

	// Do not call SetExecuteTaskComplete as there could be DisplayData message that contains image data
}

// handle display_data
func handleDisplayData(message GenericMessage) {
	msgId := message.ParentHeader.MsgId
	if msgId == "" {
		return
	}

	executeResultAndTaskCompleteSource, ok := m_executeTaskCompleteSourceDict[msgId]
	if !ok {
		return
	}

	if message.Content != nil {
		data := message.Content.Data
		if data != (GenericMessageContentData{}) {
			imagePng := data.ImagePng
			if imagePng != "" {
				if strings.HasSuffix(imagePng, "\n") {
					imagePng = imagePng[:len(imagePng)-1]
				}

				executeResultAndTaskCompleteSource.ExecuteResult.TextOfficePy = BuildOfficePyResultForImage(imagePng)
				executeResultAndTaskCompleteSource.ExecuteResult.ImageBase64Data = imagePng
			}
		}
	}
}

// handle error
func HandleMessage_Error(message GenericMessage) {
	msgId := message.ParentHeader.MsgId
	if msgId == "" {
		return
	}

	executeResultAndTaskCompleteSource, ok := m_executeTaskCompleteSourceDict[msgId]
	if !ok {
		return
	}

	executeResultAndTaskCompleteSource.ExecuteResult.Success = false
	if message.Content != nil {
		strErrorName := message.Content.ErrorName
		executeResultAndTaskCompleteSource.ExecuteResult.ErrorName = strErrorName

		strErrorValue := message.Content.ErrorValue
		executeResultAndTaskCompleteSource.ExecuteResult.ErrorMessage = strErrorValue

		errorTraceback := message.Content.Traceback
		if errorTraceback != nil {
			var sb strings.Builder
			for _, elem := range errorTraceback {
				if elem != "" {
					sb.WriteString(elem)
					sb.WriteString("\n")
				}
			}
			executeResultAndTaskCompleteSource.ExecuteResult.ErrorTraceback = sb.String()
		}
	}

	//delete(m_executeTaskCompleteSourceDict, msgId) <-- to be implemented if required
	SetExecuteTaskComplete(executeResultAndTaskCompleteSource)
}

// handle kernel_info_request <-- To be implemented if required

// handle status
func handleStatus(message GenericMessage) {
	if message.Content == nil {
		return
	}

	executeStateValue := message.Content.ExecutionState
	if executeStateValue == "" {
		return
	}

	if executeStateValue == "restarting" {
		// kernelRestarted = true --> TODO: Implement this if required
		for _, item := range m_executeTaskCompleteSourceDict {
			item.ExecuteResult.Success = false
			item.ExecuteResult.ErrorCode = KernelRestarted
			SetExecuteTaskComplete(item)
		}

		m_executeTaskCompleteSourceDict = make(map[string]*ExecuteResultAndTaskCompleteSource)

		return
	}

	msgId := message.ParentHeader.MsgId
	if msgId == "" {
		return
	}

	if executeResultAndTaskCompleteSource, ok := m_executeTaskCompleteSourceDict[msgId]; ok {
		if executeStateValue == "idle" {
			//delete(m_executeTaskCompleteSourceDict, msgId) <-- to be implemented if required
			executeResultAndTaskCompleteSource.ExecuteResult.Success = true
			SetExecuteTaskComplete(executeResultAndTaskCompleteSource)
		}
	}
}

func SetExecuteTaskComplete(executeResultAndTaskCompleteSource *ExecuteResultAndTaskCompleteSource) {
	TransferOutputMessageToExecuteResult(&executeResultAndTaskCompleteSource.ExecuteResult)
	// executeResultAndTaskCompleteSource.ExecuteResult.ExecutionDurationMilliseconds += int(time.Since(startTime).Milliseconds())
	executeResultAndTaskCompleteSource.TaskCompletionSource = &executeResultAndTaskCompleteSource.ExecuteResult
	executeResultAndTaskCompleteSource.ExecuteResultAlreadySet = true
}

var m_stdout strings.Builder
var m_stderr strings.Builder

func TransferOutputMessageToExecuteResult(result *ExecutePlainTextResult) {
	TrimAndAppendEllipses(&m_stderr, NewNotebookClientOptions().MaxStdoutMessageLength)
	result.Stderr = m_stderr.String()
	TrimAndAppendEllipses(&m_stdout, NewNotebookClientOptions().MaxStdoutMessageLength)
	result.Stdout = m_stdout.String()

	// clear
	m_stderr.Reset()
	m_stdout.Reset()
}

func AppendOutputMessage(sb *strings.Builder, text string) {
	if NewNotebookClientOptions().MaxStdoutMessageLength <= 0 {
		// no output is allowed
		return
	}

	// Add one more so that we know whether to use "..." at the end.
	capacityLeft := NewNotebookClientOptions().MaxStdoutMessageLength + 1 - sb.Len()

	if capacityLeft <= 0 {
		return
	}

	if len(text) <= capacityLeft {
		sb.WriteString(text)
	} else {
		sb.WriteString(text[:capacityLeft])
	}
}

func TrimAndAppendEllipses(sb *strings.Builder, maxLength int) {
	if maxLength < 0 {
		panic("maxLength must be non-negative")
	}

	if maxLength == 0 {
		sb.Reset()
		return
	}

	if sb.Len() > 0 && sb.Len() > maxLength {
		appendEllipses := false
		len := maxLength
		if len >= 3 {
			len = len - 3
			appendEllipses = true
		}

		if len > 0 && utf8.RuneStart(sb.String()[len-1]) {
			len = len - 1
		}

		sb.Reset()
		sb.WriteString(sb.String()[:len])
		if appendEllipses {
			sb.WriteString("...")
		}
	}
}

func BuildOfficePyResultForImage(imageBase64Data string) string {
	var result strings.Builder
	writer := json.NewEncoder(&result)

	writer.Encode(struct {
		OfficePyResult struct {
			Type       string `json:"type"`
			Format     string `json:"format"`
			Base64Data string `json:"base64_data"`
		} `json:"officepy_result"`
	}{
		OfficePyResult: struct {
			Type       string `json:"type"`
			Format     string `json:"format"`
			Base64Data string `json:"base64_data"`
		}{
			Type:       "image",
			Format:     "png",
			Base64Data: imageBase64Data,
		},
	})

	return result.String()
}

func ConvertJupyterPlainResultToExecuteCodeResult(plainResult ExecutePlainTextResult, startTime time.Time) ExecutionResponse {
	result := ExecutionResponse{}

	if plainResult.Success {
		if plainResult.TextOfficePy != "" {
			j, _ := json.RawMessage(plainResult.TextOfficePy).MarshalJSON()
			rawMessage := json.RawMessage(j)
			result.Result = &rawMessage
		} else if outVal, retVal := TryParsePythonLiteralBool(plainResult.TextPlain); retVal == true {
			outVal, _ := json.Marshal(outVal)
			outVal_rawJson := json.RawMessage(outVal)
			result.Result = &outVal_rawJson
		} else if intValue, retVal := TryParsePythonLiteralInteger(plainResult.TextPlain); retVal == true {
			outVal, _ := json.Marshal(intValue)
			outVal_rawJson := json.RawMessage(outVal)
			result.Result = &outVal_rawJson
		} else if doubleValue, retVal := TryParsePythonLiteralDouble(plainResult.TextPlain); retVal == true {
			outVal, _ := json.Marshal(doubleValue)
			outVal_rawJson := json.RawMessage(outVal)
			result.Result = &outVal_rawJson
		} else if strValue, retVal := TryParsePythonLiteralString(plainResult.TextPlain); retVal == true {
			outVal, _ := json.Marshal(strValue)
			outVal_rawJson := json.RawMessage(outVal)
			result.Result = &outVal_rawJson
		} else {
			defaultJson, _ := json.Marshal(plainResult.TextPlain)
			defaultJson_rawJson := json.RawMessage(defaultJson)
			result.Result = &defaultJson_rawJson
		}
	} else {
		// value defined in %SRCROOT%\officepy\publicapi\public\error.h
		switch plainResult.ErrorCode {
		case KernelRestarted:
			result.HResult = -2147205111
			fmt.Println("Kernel restarted")
		case ExecutionAborted:
			result.HResult = -2147205113
			fmt.Println("Execution aborted")
		default:
			if plainResult.ErrorName == "KeyboardInterrupt" {
				result.HResult = -2147205110
			} else {
				if plainResult.ErrorName == "" {
					result.HResult = -2147205117
				} else {
					result.HResult = -2147205116
				}
			}
			fmt.Println("Error: ", plainResult.ErrorName, " - ", plainResult.ErrorMessage, " - ", plainResult.ErrorTraceback)
		}

		result.ErrorMessage = plainResult.ErrorMessage
		result.ErrorName = plainResult.ErrorName
		result.ErrorStackTrace = plainResult.ErrorTraceback

		if result.ErrorName == "SyntaxError" {
			result.ErrorMessage = RemoveFileNameFromSyntaxErrorMessage(result.ErrorMessage)
		} else if result.ErrorName == "KeyboardInterrupt" {
			result.ErrorName = "Timeout"
			result.ErrorMessage = "Timeout"
		} else if result.ErrorName == "ProxyError" {
			fmt.Println("Proxy error")
		}
	}

	result.Stdout = plainResult.Stdout
	result.Stderr = plainResult.Stderr
	result.DiagnosticInfo.ExecutionDuration = int(time.Since(startTime).Milliseconds())

	result.ApproximateSize = StringLength(&plainResult.TextOfficePy) + StringLength(&plainResult.TextPlain) + StringLength(&plainResult.Stdout) + StringLength(&plainResult.Stderr)

	return result
}
