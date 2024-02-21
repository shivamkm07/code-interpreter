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
	"regexp"
	"strconv"
	"strings"
)

// handle stream
func handleStream(message GenericMessage) {
	if message.Content == nil {
		return
	}

	name := message.Content.Name
	text := message.Content.Text

	if name == "" || text == "" {
		return
	}

	if name == "stdout" {
		AppendOutputMessage(&m_stdout, text)
	} else if name == "stderr" {
		AppendOutputMessage(&m_stderr, text)
	}
}

var regexSyntaxErrorMessage = regexp.MustCompile(`(?P<line>\(\d+\))`)

func RemoveFileNameFromSyntaxErrorMessage(message string) string {
	if message == "" {
		return ""
	}

	return regexSyntaxErrorMessage.ReplaceAllString(message, "(${line})")
}

func StringLength(s *string) int {
	if s == nil {
		return 0
	}
	return len(*s)
}

func TryParsePythonLiteralBool(literal string) (bool, bool) {
	if literal == "True" {
		return true, true
	} else if literal == "False" {
		return false, true
	}

	return false, false
}

func TryParsePythonLiteralInteger(literal string) (int, bool) {
	value, err := strconv.Atoi(literal)
	if err != nil {
		return 0, false
	}
	return value, true
}

func TryParsePythonLiteralDouble(literal string) (float64, bool) {
	value, err := strconv.ParseFloat(literal, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func TryParsePythonLiteralString(literal string) (string, bool) {
	value := ""
	if len(literal) < 2 {
		return value, false
	}

	if literal[0] == '\'' && literal[len(literal)-1] == '\'' ||
		literal[0] == '"' && literal[len(literal)-1] == '"' {
		sb := strings.Builder{}
		for i := 1; i < len(literal)-1; i++ {
			if literal[i] != '\\' {
				sb.WriteString(string(literal[i]))
			} else if i+1 < len(literal)-1 {
				i++
				chNext := literal[i]
				switch chNext {
				case '\\':
					sb.WriteString("\\")
				case '\'':
					sb.WriteString("'")
				case '"':
					sb.WriteString("\"")
				case 'a':
					sb.WriteString("\a")
				case 'b':
					sb.WriteString("\b")
				case 'f':
					sb.WriteString("\f")
				case 'n':
					sb.WriteString("\n")
				case 'r':
					sb.WriteString("\r")
				case 't':
					sb.WriteString("\t")
				case 'v':
					sb.WriteString("\v")
				default:
					panic("Invalid escape sequence")
				}
			}
		}
		value = sb.String()
		return value, true
	}

	return value, false
}
