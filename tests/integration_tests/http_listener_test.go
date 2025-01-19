package integrationtests

import (
	"fmt"
	"testing"
	"time"

	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/target"
	"github.com/codeasashu/HookRelay/pkg/subscription"
)

//	func TestListenerStarts(t *testing.T) {
//		t.Run("ServerIsRunning", func(t *testing.T) {
//			conn, err := net.Dial("tcp", "127.0.0.1:8082")
//			if err != nil {
//				t.Fatalf("Failed to connect to server: %v", err)
//			}
//			conn.Close()
//		})
//	}
//
//	func TestSendEvent(t *testing.T) {
//		ec := NewHTTPEventClient()
//
//		// Test without any subscriptions
//		subscription.Init()
//
//		testCases := []struct {
//			name  string
//			event event.Event
//		}{
//			{name: "test3", event: event.Event{
//				OwnerId:   "owner123",
//				EventType: "webhook.abc",
//				Payload: map[string]string{
//					"a": "b",
//				},
//				LatencyTimestamps: make(map[string]time.Time),
//			}},
//		}
//
//		for _, tc := range testCases {
//			t.Run(tc.name, func(t *testing.T) {
//				resp, err := ec.SendEvent(tc.event)
//				if err != nil {
//					t.Error("Failed to send event:", err)
//				}
//				if resp.StatusCode != 200 {
//					t.Errorf("Expected status code 200, got %d, response=%s", resp.StatusCode, string(ec.ResponseBody))
//				}
//			})
//		}
//	}
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

func BenchmarkSendEvent(b *testing.B) {
	ec := NewHTTPEventClient()

	subscription.Init()
	subscription.CreateSubscription("webhook.abc", &subscription.Subscription{
		ID:      "123",
		OwnerId: "owner123",
		Target: &target.Target{
			Type: "http",
			HTTPDetails: &target.HTTPDetails{
				URL:    "http://localhost:9092/testReceive",
				Method: "POST",
			},
		},
	})

	subscription.CreateSubscription("webhook.abc", &subscription.Subscription{
		ID:      "456",
		OwnerId: "owner123",
		Target: &target.Target{
			Type: "http",
			HTTPDetails: &target.HTTPDetails{
				URL:    "http://localhost:9092/testReceive2",
				Method: "POST",
			},
		},
	})
	b.ResetTimer()

	st := 0
	for n := 0; n < b.N; n++ {
		st += 1
		ev := event.Event{
			OwnerId:   "owner123",
			EventType: "webhook.abc",
			Payload: map[string]string{
				"a": "b",
			},
			LatencyTimestamps: make(map[string]time.Time),
		}
		resp, err := ec.SendEvent(ev)
		if err != nil {
			b.Error("Failed to send event:", err)
		}
		if resp.StatusCode != 200 {
			b.Errorf("Expected status code 200, got %d, response=%s", resp.StatusCode, string(ec.ResponseBody))
		}
	}

	// time.Sleep(4 * time.Second)
	fmt.Printf("Total Hits st: %d\n", st)
}
