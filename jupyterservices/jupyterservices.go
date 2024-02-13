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

package jupyterservices

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/microsoft/jupyterpython/util"
)

// define kernel and session
type Kernel struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	LastActivity   string `json:"last_activity"`
	ExecutionState string `json:"execution_state"`
	Connections    int    `json:"connections"`
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

const (
	jupyterURL = "http://localhost:8888"
	Timeout    = 60 * time.Second
)

var Token = ""

// check if there are any available kernels running and if so create a new session
// return the kernelId and sessionId
func CheckKernels(kernelId string) (string, string, error) {
	fmt.Println("Checking for available kernels... with token: ", Token)

	url := fmt.Sprintf("%s/api/kernels?token=%s", jupyterURL, Token)
	client := util.HTTPClient()
	response, err := client.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("error getting kernels: %v", err)
	}

	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", "", fmt.Errorf("error reading response body: %v", err)
	}

	var kernels []Kernel
	err = json.Unmarshal(body, &kernels)
	if err != nil {
		return "", "", fmt.Errorf("error unmarshaling JSON: %v", err)
	}

	fmt.Println(kernels)

	var sessionId string
	// if kernel exists, respond with kernel Id
	if len(kernels) > 0 {
		fmt.Printf("Kernel ID: %s\n", kernels[0].ID)
		sessions, err := getSessions(client)
		if err != nil {
			return "", "", fmt.Errorf("error getting sessions: %v", err)
		}

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
		newSession, err := createSession()
		if err != nil {
			return "", "", fmt.Errorf("error creating new session: %v", err)
		}
		fmt.Printf("Session ID: %s\n", newSession.ID)
		sessionId = newSession.ID
		kernelId = newSession.Kernel.ID
	}

	return kernelId, sessionId, nil
}

// get sessions and return json object
func getSessions(client *http.Client) ([]Session, error) {
	fmt.Println("Listing available sessions:")

	url := fmt.Sprintf("%s/api/sessions?token=%s", jupyterURL, Token)
	response, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error getting sessions: %v", err)
	}

	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	var sessions []Session
	err = json.Unmarshal(body, &sessions)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON: %v", err)
	}

	fmt.Println(sessions)

	return sessions, nil
}

func createSession() (*Session, error) {
	fmt.Println("Creating a new session...")

	// payload for POST request to create session as io.Reader value
	payload := bytes.NewBuffer([]byte(`{"path": "", "type": "notebook", "kernel": {"name": "python3"}}`))

	url := fmt.Sprintf("%s/api/sessions?token=%s", jupyterURL, Token)
	response, err := http.Post(url, "application/json", payload)
	if err != nil {
		return nil, fmt.Errorf("error creating session: %v", err)
	}

	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	sessionInfo := &Session{}
	err = json.Unmarshal(body, sessionInfo)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON: %v", err)
	}

	return sessionInfo, nil
}
