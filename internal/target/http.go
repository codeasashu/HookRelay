package target

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/codeasashu/HookRelay/internal/config"
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

func (target *Target) ProcessTarget(payload []byte, traceId string) (int, error) {
	if target.Type != TargetHTTP {
		slog.Error("unsupported target type", "type", target.Type)
		return 0, errors.New("unsupported target type")
	}

	if target.HTTPDetails == nil {
		slog.Info("missing HTTP details in target")
		return 0, errors.New("missing HTTP details in target")
	}

	// conisder DefaultMethod if no method is provided
	if target.HTTPDetails.Method == "" {
		target.HTTPDetails.Method = DefaultMethod
	}

	logAttrs := []any{
		"method", string(target.HTTPDetails.Method),
		"url", string(target.HTTPDetails.URL),
	}

	var req *http.Request
	var err error
	var bodyReader io.Reader = http.NoBody

	if traceId != "" {
		target.HTTPDetails.Headers[config.TraceIDHeaderName] = traceId
		logAttrs = append(logAttrs, "trace_id", traceId)
	}

	// Handle POST request with different Content-Types
	if target.HTTPDetails.Method == POSTMethod {
		// Ensure Content-Type is set
		contentType := target.HTTPDetails.Headers["Content-Type"]
		if contentType == "" {
			slog.Debug("No Content-Type header found, assuming it as: application/json")
			contentType = "application/json" // Default Content-Type
			target.HTTPDetails.Headers["Content-Type"] = contentType
		}

		// Handle different Content-Types
		switch contentType {
		case "application/json":
			bodyReader = bytes.NewBuffer(payload)
		case "application/x-www-form-urlencoded":
			data := url.Values{}
			data.Set("payload", string(payload))
			bodyReader = strings.NewReader(data.Encode())
		case "multipart/form-data":
			var buffer bytes.Buffer
			writer := multipart.NewWriter(&buffer)
			writer.WriteField("payload", string(payload))
			writer.Close()
			bodyReader = &buffer
			target.HTTPDetails.Headers["Content-Type"] = writer.FormDataContentType()
		default:
			slog.Error("unsupported Content-Type found in headers", "content-type", contentType)
			return 0, errors.New("unsupported Content-Type: " + contentType)
		}
	}

	// Create HTTP request
	req, err = http.NewRequest(string(target.HTTPDetails.Method), target.HTTPDetails.URL, bodyReader)
	if err != nil {
		slog.Error("failed to create HTTP request", "err", err)
		return 0, err
	}

	// Set headers and authentication AFTER creating the request
	req = target.SetHeaders(req)
	req = target.SetAuth(req)

	// Log HTTP request details safely
	sanitizedHeaders := make(map[string]string)
	for key, value := range target.HTTPDetails.Headers {
		// remove auth token for logging
		if strings.ToLower(key) == "authorization" {
			sanitizedHeaders[key] = "REDACTED"
		} else {
			sanitizedHeaders[key] = value
		}
	}

	logAttrs = append(logAttrs, "headers", fmt.Sprintf("%+v", sanitizedHeaders))

	// Log BasicAuth username if provided
	if target.HTTPDetails.BasicAuth.Username != "" {
		logAttrs = append(logAttrs, "basic_auth_username", target.HTTPDetails.BasicAuth.Username)
	}

	slog.Info("sending HTTP request", logAttrs...)

	// Send HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("failed to send HTTP request", "trace_id", traceId, "err", err)
		return 0, err
	}
	defer resp.Body.Close()

	// Read 250 bytes and log the response
	body, err := io.ReadAll(io.LimitReader(resp.Body, 250))
	if err != nil {
		slog.Error("failed to read HTTP response", "trace_id", traceId, "err", err)
		return 0, err
	}

	targetResponse := &HTTPTargetResponse{
		Status: TargetStatus(StatusOK),
		Code:   resp.StatusCode,
		Body:   body,
	}

	if resp.StatusCode != http.StatusOK {
		targetResponse.Status = TargetStatus(StatusErr)
		slog.Error("invalid HTTP status", "trace_id", traceId, "status", resp.Status)
		return resp.StatusCode, errors.New("Non-200 Status: " + resp.Status)
	}

	slog.Debug("got full HTTP response body", "trace_id", traceId, "body", string(body))
	slog.Info("got HTTP response body (truncated)", "trace_id", traceId, "body", string(body[:min(100, len(body))]))

	target.HTTPResponse = targetResponse
	return resp.StatusCode, nil
}
