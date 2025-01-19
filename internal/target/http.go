package target

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
)

type HTTPMethod string

const (
	GETMethod  HTTPMethod = "GET"
	POSTMethod HTTPMethod = "POST"
)

const DefaultMethod = GETMethod

const (
	TargetHTTP TargetType = "http"
	// TargetWebSocket TargetType = "websocket"
)

type HTTPTargetResponse struct {
	Status TargetStatus `json:"status"`
	Code   int          `json:"code"`
	Body   []byte       `json:"body"`
}

type HTTPDetails struct {
	URL     string            `json:"url" binding:"required` // Endpoint URL
	Headers map[string]string `json:"headers,omitempty"`     // Custom headers
	Method  HTTPMethod        `json:"method"`                // HTTP method (e.g., "GET", "POST")
}

func Run(t *Target) (string, error) {
	if t == nil {
		return "", errors.New("target is nil")
	}
	if t.Type == TargetHTTP {
		if t.HTTPDetails == nil {
			return "", errors.New("HTTP details are nil")
		}
		return genSHA(t.HTTPDetails.URL)
	}
	return "", errors.New("target type is invalid")
}

func (target *Target) ProcessTarget(payload interface{}) error {
	if target.Type != TargetHTTP {
		log.Printf("unsupported target type: %s \n", target.Type)
		return errors.New("unsupported target type")
	}

	if target.HTTPDetails == nil {
		log.Print("missing HTTP details in target")
		return errors.New("missing HTTP details in target")
	}

	if target.HTTPDetails.Method != "POST" {
		log.Printf("unsupported HTTP method: %v", target.HTTPDetails.Method)
		return errors.New("unsupported HTTP Method")
	}

	// Marshal payload into JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("failed to marshal payload: %s", err)
		return err
	}

	// Create HTTP request
	req, err := http.NewRequest(string(target.HTTPDetails.Method), target.HTTPDetails.URL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Printf("failed to create HTTP request: %s", err)
		return err
	}

	// Add headers
	for key, value := range target.HTTPDetails.Headers {
		req.Header.Set(key, value)
	}

	// Set default content-type if not provided
	if _, ok := target.HTTPDetails.Headers["Content-Type"]; !ok {
		req.Header.Set("Content-Type", "application/json")
	}

	// Send HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("failed to send HTTP request: %s", err)
		return err
	}
	defer resp.Body.Close()

	// Read and log the response (for debugging purposes)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to read HTTP response: %s", err)
		return err
	}

	targetResponse := &HTTPTargetResponse{
		Status: TargetStatus(StatusOK),
		Code:   resp.StatusCode,
		Body:   body,
	}
	log.Printf("HTTP Response: %s\n", body)
	log.Printf("HTTP Status: %s\n", resp.Status)
	if resp.StatusCode != 200 {
		targetResponse.Status = TargetStatus(StatusErr)
		log.Printf("HTTP Status: %s\n", resp.Status)
		return errors.New("Non-200 Status: " + resp.Status)
	}

	target.HTTPResponse = targetResponse
	return nil
}
