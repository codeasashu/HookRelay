package target

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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
	URL     string            `json:"url" binding:"required"` // Endpoint URL
	Headers map[string]string `json:"headers,omitempty"`      // Custom headers
	Method  HTTPMethod        `json:"method"`                 // HTTP method (e.g., "GET", "POST")
}

//	func Run(t *Target) (string, error) {
//		if t == nil {
//			return "", errors.New("target is nil")
//		}
//		if t.Type == TargetHTTP {
//			if t.HTTPDetails == nil {
//				return "", errors.New("HTTP details are nil")
//			}
//			return genSHA(t.HTTPDetails.URL)
//		}
//		return "", errors.New("target type is invalid")
//	}
func (target *Target) ProcessTarget(payload interface{}) (int, error) {
	if target.Type != TargetHTTP {
		slog.Error("unsupported target type", "type", target.Type)
		return 0, errors.New("unsupported target type")
	}

	if target.HTTPDetails == nil {
		slog.Info("missing HTTP details in target")
		return 0, errors.New("missing HTTP details in target")
	}

	// Marshal payload into JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal payload", "err", err)
		return 0, err
	}

	// Create HTTP request
	if target.HTTPDetails.Method == "" {
		target.HTTPDetails.Method = DefaultMethod
	}

	var req *http.Request
	if target.HTTPDetails.Method == GETMethod {
		// No body in GET Method
		req, err = http.NewRequest(string(target.HTTPDetails.Method), target.HTTPDetails.URL, http.NoBody)
	} else {
		req, err = http.NewRequest(string(target.HTTPDetails.Method), target.HTTPDetails.URL, bytes.NewBuffer(payloadBytes))
	}

	if err != nil {
		slog.Error("failed to create HTTP request", "err", err)
		return 0, err
	}
	// Add headers
	for key, value := range target.HTTPDetails.Headers {
		req.Header.Set(key, value)
	}

	// Set default content-type if not provided
	if _, ok := target.HTTPDetails.Headers["Content-Type"]; !ok {
		req.Header.Set("Content-Type", "application/json")
	}

	slog.Info("sending HTTP request", "req", req.URL.String())
	// Send HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("failed to send HTTP request", "err", err)
		return 0, err
	}
	defer resp.Body.Close()

	// Read and log the response (for debugging purposes)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Info("failed to read HTTP response", "err", err)
		return 0, err
	}

	targetResponse := &HTTPTargetResponse{
		Status: TargetStatus(StatusOK),
		Code:   resp.StatusCode,
		Body:   body,
	}
	if resp.StatusCode != 200 {
		targetResponse.Status = TargetStatus(StatusErr)
		slog.Error("invalid http status", "status", resp.Status)
		return resp.StatusCode, errors.New("Non-200 Status: " + resp.Status)
	}

	slog.Info("got http reply", "status", resp.Status)
	target.HTTPResponse = targetResponse
	return resp.StatusCode, nil
}
