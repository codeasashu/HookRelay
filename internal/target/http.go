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

type HTTPBasicAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type HTTPTargetResponse struct {
	Status TargetStatus `json:"status"`
	Code   int          `json:"code"`
	Body   []byte       `json:"body"`
}

type HTTPDetails struct {
	URL       string            `json:"url" binding:"required"` // Endpoint URL
	Headers   map[string]string `json:"headers,omitempty"`      // Custom headers
	Method    HTTPMethod        `json:"method"`                 // HTTP method (e.g., "GET", "POST")
	BasicAuth HTTPBasicAuth     `json:"auth,omitempty"`         // Basic Auth Username
}

func (t *Target) SetAuth(req *http.Request) *http.Request {
	// Add Basic Authentication if credentials are present
	if t.HTTPDetails.BasicAuth.Username != "" && t.HTTPDetails.BasicAuth.Password != "" {
		req.SetBasicAuth(t.HTTPDetails.BasicAuth.Username, t.HTTPDetails.BasicAuth.Password)
		slog.Info("using basic authentication", "user", t.HTTPDetails.BasicAuth.Username)
	}
	return req
}

func (t *Target) SetHeaders(req *http.Request) *http.Request {
	for key, value := range t.HTTPDetails.Headers {
		req.Header.Set(key, value)
	}

	// Set default content-type if not provided
	if _, ok := t.HTTPDetails.Headers["Content-Type"]; !ok {
		req.Header.Set("Content-Type", "application/json")
	}

	return req
}

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

	req = target.SetHeaders(req)
	req = target.SetAuth(req)

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
	slog.Debug("got http response body", "body", string(body))
	target.HTTPResponse = targetResponse
	return resp.StatusCode, nil
}
