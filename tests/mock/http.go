package mock

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type MockHTTPServer struct {
	mutex     *sync.Mutex
	Server    *http.Server
	Hits      int
	Responses int
	Errors    int
}

func InitMockHTTPServer(port int) *MockHTTPServer {
	return &MockHTTPServer{
		Server: &http.Server{
			Addr: fmt.Sprintf(":%d", port),
		},
		Hits:      0,
		Responses: 0,
		Errors:    0,
		mutex:     &sync.Mutex{},
	}
}

func (s *MockHTTPServer) StartFixedDurationServer(delay time.Duration) {
	log.Println("Starting fixed duration server...")
	s.Server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// s.mutex.Lock()
		// s.Hits += 1
		// s.mutex.Unlock()
		time.Sleep(delay) // Simulate processing time
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Mock response")
	})
	go func() {
		if err := s.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
}

func (s *MockHTTPServer) StartVariableDurationServer(minDelay time.Duration, maxDelay time.Duration) {
	s.Server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// s.mutex.Lock()
		// s.Hits += 1
		// s.mutex.Unlock()
		delay := time.Duration(rand.Int63n(int64(maxDelay-minDelay))) + minDelay
		time.Sleep(delay) // Simulate processing time
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Mock response")
	})
	go func() {
		if err := s.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
}
