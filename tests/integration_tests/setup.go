package integrationtests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/codeasashu/HookRelay/internal/dispatcher"
	"github.com/codeasashu/HookRelay/pkg/listener"
)

type HTTPEventClient struct {
	client         *http.Client
	baseURL        string
	ResponseBody   []byte
	ResponseStatus []byte
}

func NewHTTPEventClient() *HTTPEventClient {
	return &HTTPEventClient{
		baseURL: "http://localhost:8082/event",
		client: &http.Client{
			Timeout: time.Second * 3,
		},
	}
}

func (e *HTTPEventClient) SendEvent(payload interface{}) (*http.Response, error) {
	if payload == nil {
		return nil, errors.New("payload is nil")
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, e.baseURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	e.ResponseBody = body
	return resp, nil
}

func RunHTTPListener(ctx context.Context, wg *sync.WaitGroup, disp *dispatcher.Dispatcher) *listener.HTTPListener {
	httpListenerServer := listener.NewHTTPListener("localhost:8082", disp)
	wg.Add(1)
	go func() {
		defer wg.Done()
		httpListenerServer.StartAndReceive()
	}()

	return httpListenerServer
}

// func RunMockHTTPServer(ctx context.Context, wg *sync.WaitGroup, disp *dispatcher.Dispatcher) *listener.HTTPListener {
// 	mockServer := mock.VaryingResponseServer(9092, 100*time.Millisecond, 500*time.Millisecond)
// }
