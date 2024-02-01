package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var (
	interrupt = make(chan os.Signal, 1)
	wg        sync.WaitGroup
)

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

type ExecuteRequest struct {
	Code string `json:"code"`
}

const jupyterURL = "http://localhost:8888"
const timeout = 60 * time.Second

var token = ""

// JupyterMessage represents a Jupyter message structure.
type JupyterMessage struct {
	Header       map[string]interface{} `json:"header"`
	Metadata     map[string]interface{} `json:"metadata"`
	Content      map[string]interface{} `json:"content"`
	ParentHeader map[string]interface{} `json:"parent_header"`
	Channel      string                 `json:"channel"`
	BufferPaths  []string               `json:"buffer_paths"`
}

func main() {
	r := mux.NewRouter()

	// Define your routes
	r.HandleFunc("/", initializeJupyter).Methods("GET")
	r.HandleFunc("/execute", execute).Methods("POST")

	// Start the HTTP server
	http.Handle("/", r)
	fmt.Println("Server listening on :8080")
	http.ListenAndServe(":8080", nil)
}

// func to take token from the environment variable
func getToken() string {
	token = os.Getenv("JUPYTER_TOKEN")
	if token == "" {
		log.Fatal("JUPYTER_TOKEN environment variable not set")
	}
	return token
}

// func to initialize jupyter
func initializeJupyter(w http.ResponseWriter, r *http.Request) {
	// get token from the environment variable
	token = getToken()
	checkKernels("")
}

// check if there are any available kernels running and if so create a new session
func checkKernels(kernelId string) (string, string) {
	fmt.Println("Checking for available kernels:")

	url := fmt.Sprintf("%s/api/kernels?token=%s", jupyterURL, token)
	response, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	var kernels []Kernel
	err = json.Unmarshal(body, &kernels)
	if err != nil {
		log.Fatal(err)
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
		fmt.Printf("Session ID: %s\n", sessionId)
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
		log.Fatal(err)
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	var sessions []Session
	err = json.Unmarshal(body, &sessions)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(sessions)

	return sessions
}

func createSession() Session {
	fmt.Println("Creating a new session:")

	// payload for POST request to create session as io.Reader value
	payload := bytes.NewBuffer([]byte(`{"path": "", "type": "notebook", "kernel": {"name": "python3"}}`))

	url := fmt.Sprintf("%s/api/sessions?token=%s", jupyterURL, token)
	response, err := http.Post(url, "application/json", payload)
	if err != nil {
		log.Fatal(err)
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	var sessionInfo Session
	err = json.Unmarshal(body, &sessionInfo)
	if err != nil {
		log.Fatal(err)
	}

	return sessionInfo
}

func execute(w http.ResponseWriter, r *http.Request) {
	// read code from the request body
	code, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
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
	var codeString ExecuteRequest
	err = json.Unmarshal(code, &codeString)
	if err != nil {
		log.Fatal(err)
	}
	// {"code": "print('Hello, Earth!')"}

	// execute the code
	response := executeCode(kernelId, sessionId, codeString.Code)

	// return the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))

}

func executeCode(kernelId, sessionId, code string) string {
	fmt.Println("Executing code in the session using WebSocket:")

	responseChan := connectWebSocket(kernelId, code)

	// Wait for response or timeout
	select {
	case response := <-responseChan:
		fmt.Println("Received response:", response)
		return response
	case <-time.After(60 * time.Second): // Timeout after 10 seconds
		fmt.Println("Timeout: No response received.")
		return "Timeout: No response received."
	}
}

func onMessage(message []byte) string {
	fmt.Printf("Received message: %s\n", message)
	var msg map[string]interface{}
	err := json.Unmarshal(message, &msg)
	if err != nil {
		log.Fatal("Error unmarshaling JSON:", err)
	}
	header := msg["header"].(map[string]interface{})
	msgType := header["msg_type"].(string)

	switch msgType {
	case "stream":
		content := msg["content"].(map[string]interface{})
		if content["name"].(string) == "stdout" {
			fmt.Printf("\n\nSTDOUT: %s\n", content["text"])
			return content["text"].(string)
		} else if content["name"].(string) == "stderr" {
			fmt.Printf("\n\nSTDERR: %s\n", content["text"])
			return content["text"].(string)
		}
	case "execute_result":
		content := msg["content"]
		data := content.(map[string]interface{})["data"]
		text := data.(map[string]interface{})["text/plain"]
		return text.(string)
	}

	return ""
}

func onError(err error) {
	log.Println("Error:", err)
}

func onClose() {
	log.Println("### closed ###")
	// TODO: Commenting for now, since the Wg.Done() is adding negative value to the counter even when there are no goroutines running
	// wg.Done()
}

func onOpen(ws *websocket.Conn, code string) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		header := createHeader("execute_request")
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
			log.Fatal("Error writing JSON:", err)
		}
	}()

	wg.Wait()
}

// connect via websocket and execute code and return the result
func connectWebSocket(kernelID string, code string) <-chan string {
	responseChan := make(chan string)

	interruptSignal := make(chan os.Signal, 1)
	signal.Notify(interruptSignal, os.Interrupt, syscall.SIGTERM)

	u := url.URL{Scheme: "ws", Host: "localhost:8888", Path: "/api/kernels/" + kernelID + "/channels", RawQuery: "token=" + token}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("Error connecting to WebSocket:", err)
		close(responseChan)
		return responseChan
	}

	c.SetCloseHandler(func(code int, text string) error {
		log.Printf("WebSocket closed with code %d: %s\n", code, text)
		onClose()
		return nil
	})

	c.SetPingHandler(func(appData string) error {
		log.Println("Received ping:", appData)
		return nil
	})

	c.SetPongHandler(func(appData string) error {
		log.Println("Received pong:", appData)
		return nil
	})

	go func() {
		defer close(responseChan)

		startTime := time.Now()
		for {
			_, message, err := c.ReadMessage()
			// print elapsed time since the start of this loop
			log.Println("Time waiting for response:", time.Since(startTime))

			// close the connection if we wait for more than timeout
			if time.Since(startTime) > timeout {
				log.Println("Timeout: No response received.")
				c.Close()
				return
			}
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					log.Println("Error reading message:", err)
				}
				return
			}
			response := onMessage(message)
			if response != "" {
				responseChan <- response
				break
			}
		}
	}()

	onOpen(c, code)

	go func() {
		select {
		case <-interruptSignal:
			log.Println("Interrupt signal received, closing WebSocket...")
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("Error sending close message:", err)
			}
		}
	}()

	return responseChan
}

func createHeader(msgType string) map[string]interface{} {
	msgID, err := uuid.NewV4()
	if err != nil {
		log.Fatal("Error generating UUID:", err)
	}

	sessionID, err := uuid.NewV4()
	if err != nil {
		log.Fatal("Error generating UUID:", err)
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
			log.Fatal("Error marshaling JSON:", err)
		}
		h.Write(data)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func sendMessage(conn *websocket.Conn, message JupyterMessage) {
	if err := conn.WriteJSON(message); err != nil {
		log.Fatal(err)
	}
}

func receiveMessage(conn *websocket.Conn) JupyterMessage {
	var response JupyterMessage
	if err := conn.ReadJSON(&response); err != nil {
		log.Fatal(err)
	}

	return response
}
