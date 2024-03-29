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

package util

import (
	"net/http"
	"time"
)

func HTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * 10,
	}
}

func SendHTTPResponse(w http.ResponseWriter, statusCode int, message string, wrapMessage bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if wrapMessage {
		message = `{"message": "` + message + `"}`
	}
	w.Write([]byte(message))
}
