package integrationtests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/codeasashu/HookRelay/internal/delivery"
	"github.com/codeasashu/HookRelay/internal/listener"
	"github.com/codeasashu/HookRelay/internal/subscription"
	"github.com/codeasashu/HookRelay/internal/worker"
	"github.com/codeasashu/HookRelay/tests/mocks"
	"github.com/jmoiron/sqlx"

	"github.com/DATA-DOG/go-sqlmock"
)

func setupMockReceivers(t *testing.T, response_cb func(w http.ResponseWriter, r *http.Request), check_cb func(w http.ResponseWriter, r *http.Request) error) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := check_cb(w, r)
		if err != nil {
			t.Errorf("Error: %s", err)
		}
		response_cb(w, r)
	}))
	return server
	// defer server.Close()
}

func setupMockDb(t *testing.T, rows *sqlmock.Rows) *mocks.MockDatabase {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	// defer db.Close()
	mock.ExpectQuery(`SELECT id, company_id, url, request, simple, headers, auth, credentials, service_type, is_active, created FROM api_pushes WHERE company_id = \? AND is_active = 1 AND service_type = 2`).WithArgs("1").WillReturnRows(rows)

	mockDb := new(mocks.MockDatabase)
	mockDb.On("GetDB").Return(sqlxDB)
	return mockDb
}

func setupMockApp(t *testing.T, mockDb *mocks.MockDatabase) *app.HookRelayApp {
	wg := sync.WaitGroup{}

	ctx := context.Background()

	mainApp, err := app.FromConfig("test_config.toml", ctx, false)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	mainApp.HttpClient = &http.Client{Timeout: 5 * time.Second}
	mainApp.SubscriptionDb = mockDb
	mainApp.DeliveryDb = mockDb

	wp := worker.InitPool(mainApp)
	localWorker := worker.CreateLocalWorker(mainApp, wp, delivery.SaveDeliveries(mainApp))
	wp.SetLocalClient(localWorker)

	deliveryApp, err := delivery.NewHTTPDelivery(mainApp, wp)
	if err != nil {
		t.Fatalf("Failed to create delivery: %v", err)
	}
	deliveryApp.InitApiRoutes()
	subscriptionApp, err := subscription.NewSubscription(mainApp, false)
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}
	subscriptionApp.InitApiRoutes()

	httpListener := listener.NewHTTPListener(mainApp, deliveryApp, subscriptionApp)
	httpListener.InitApiRoutes()

	wg.Add(1)
	go func() {
		defer wg.Done()
		go mainApp.Start(nil)
	}()

	if !isServerReady(mainApp.HttpServer.Addr, 10*time.Second) {
		t.Fatalf("Server did not start within the timeout period")
	}

	return mainApp
}

// isServerReady checks if the server is ready to accept connections
func isServerReady(addr string, timeout time.Duration) bool {
	start := time.Now()
	for time.Since(start) < timeout {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(50 * time.Millisecond) // Wait before retrying
	}
	return false
}

func TestListenerStarts(t *testing.T) {
	t.Skip()
	setupMockApp(t, nil)
	t.Run("ServerIsRunning", func(t *testing.T) {
		conn, err := net.Dial("tcp", "127.0.0.1:8083")
		if err != nil {
			t.Fatalf("Failed to connect to server: %v", err)
		}
		conn.Close()
	})
}

func sendEvent(url string, jsonData string) (*http.Response, error) {
	// Convert the JSON payload to a byte array
	jsonBytes := []byte(jsonData)

	// Create a new POST request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil, err
	}

	// Set the content type to application/json
	req.Header.Set("Content-Type", "application/json")

	// Send the request using the default HTTP client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return nil, err
	}
	return resp, nil
}

func TestSendEvent(t *testing.T) {
	workerChan := make(chan string)
	expectedPayload, _ := json.Marshal(map[string]interface{}{"a": "abc", "b": "def"})
	// Setup webhook receiver mocks
	rcv := setupMockReceivers(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"value":"fixed"}`))
		workerChan <- "ok"
	}, func(w http.ResponseWriter, r *http.Request) error {
		if r.URL.Path != "/recv" {
			return errors.New("bad request")
		}
		// Verify request payload
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return err
		}
		if string(body) != string(expectedPayload) {
			return errors.New("invalid request body. expected=" + string(expectedPayload) + ", actual=" + string(body))
		}
		return nil
	})

	// Mock DB
	rows := sqlmock.NewRows([]string{"id", "company_id", "url", "request", "simple", "headers", "auth", "credentials", "service_type", "is_active", "created"}).
		AddRow(1, "1", rcv.URL+"/recv", 1, 3, "", "0", "", 2, 1, time.Now().UTC())
	db := setupMockDb(t, rows)

	f := setupMockApp(t, db)
	testCases := []struct {
		name  string
		event string
	}{
		{
			name: "positive test", event: `{
		        "owner_id": "1",
		        "event_type": "webhook.incall",
		        "payload": ` + string(expectedPayload) + `
	        }`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := sendEvent("http://localhost"+f.HttpServer.Addr+"/event", tc.event)
			// Resume further whenever we got a response
			select {
			case <-workerChan:
				if err != nil {
					t.Error("Failed to send event:", err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != 200 {
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						t.Error("Failed to send event:", err)
					}
					t.Errorf("Expected status code 200, got %d, response=%s", resp.StatusCode, string(body))
				}
				// Worker completed successfully
				t.Log("Worker completed successfully")
			case <-time.After(5 * time.Second): // Set a reasonable timeout
				t.Error("Test timed out: Worker did not complete within the expected time")
			}
		})
	}
}

// func TestSendEventWithSubscription(t *testing.T) {
// 	ec := NewHTTPEventClient()
//
// 	// Test with 2 subscriptions (fanout)
// 	subscription.Init()
// 	subscription.CreateSubscription("webhook.abc", &subscription.Subscription{
// 		ID:      "123",
// 		OwnerId: "owner123",
// 		Target: &target.Target{
// 			Type: "http",
// 			HTTPDetails: &target.HTTPDetails{
// 				URL:    "http://localhost:9092/testReceive",
// 				Method: "POST",
// 			},
// 		},
// 	})
//
// 	subscription.CreateSubscription("webhook.abc", &subscription.Subscription{
// 		ID:      "456",
// 		OwnerId: "owner123",
// 		Target: &target.Target{
// 			Type: "http",
// 			HTTPDetails: &target.HTTPDetails{
// 				URL:    "http://localhost:9092/testReceive2",
// 				Method: "POST",
// 			},
// 		},
// 	})
//
// 	testCases := []struct {
// 		name  string
// 		event event.Event
// 	}{
// 		{name: "test3", event: event.Event{
// 			OwnerId:   "owner123",
// 			EventType: "webhook.abc",
// 			Payload: map[string]string{
// 				"a": "b",
// 			},
// 			LatencyTimestamps: make(map[string]time.Time),
// 		}},
// 	}
//
// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			resp, err := ec.SendEvent(tc.event)
// 			if err != nil {
// 				t.Error("Failed to send event:", err)
// 			}
// 			if resp.StatusCode != 200 {
// 				t.Errorf("Expected status code 200, got %d, response=%s", resp.StatusCode, string(ec.ResponseBody))
// 			}
// 		})
// 	}
// }

// func BenchmarkSendEvent(b *testing.B) {
// 	ec := NewHTTPEventClient()
//
// 	subscription.Init()
// 	subscription.CreateSubscription("webhook.abc", &subscription.Subscription{
// 		ID:      "123",
// 		OwnerId: "owner123",
// 		Target: &target.Target{
// 			Type: "http",
// 			HTTPDetails: &target.HTTPDetails{
// 				URL:    "http://localhost:9092/testReceive",
// 				Method: "POST",
// 			},
// 		},
// 	})
//
// 	subscription.CreateSubscription("webhook.abc", &subscription.Subscription{
// 		ID:      "456",
// 		OwnerId: "owner123",
// 		Target: &target.Target{
// 			Type: "http",
// 			HTTPDetails: &target.HTTPDetails{
// 				URL:    "http://localhost:9092/testReceive2",
// 				Method: "POST",
// 			},
// 		},
// 	})
// 	b.ResetTimer()
//
// 	st := 0
// 	for n := 0; n < b.N; n++ {
// 		st += 1
// 		ev := event.Event{
// 			OwnerId:   "owner123",
// 			EventType: "webhook.abc",
// 			Payload: map[string]string{
// 				"a": "b",
// 			},
// 		}
// 		resp, err := ec.SendEvent(ev)
// 		if err != nil {
// 			b.Error("Failed to send event:", err)
// 		}
// 		if resp.StatusCode != 200 {
// 			b.Errorf("Expected status code 200, got %d, response=%s", resp.StatusCode, string(ec.ResponseBody))
// 		}
// 	}
//
// 	// time.Sleep(4 * time.Second)
// 	fmt.Printf("Total Hits st: %d\n", st)
// }
