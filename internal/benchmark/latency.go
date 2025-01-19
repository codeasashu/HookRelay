package benchmark

import (
	"log"
	"time"

	"github.com/codeasashu/HookRelay/internal/dispatcher"
	"github.com/codeasashu/HookRelay/internal/event"
)

type Latency struct {
	dispatcher          *dispatcher.Dispatcher
	endToEnd            []time.Duration
	endToEndWithoutHttp []time.Duration
	listener            []time.Duration
	worker              []time.Duration
	dispatcherLatency   []time.Duration
}

func NewLatencyBench(disp *dispatcher.Dispatcher) *Latency {
	l := &Latency{
		dispatcher: disp,
		endToEnd:   make([]time.Duration, 0),
		listener:   make([]time.Duration, 0),
		worker:     make([]time.Duration, 0),
	}
	return l
}

func (l *Latency) PrintSummary() {
	for _, result := range l.dispatcher.JobResults {
		l.addEventLatency(result.Job.Event)
	}

	log.Printf("End-to-End Latency: Avg=%v\n", avg(l.endToEnd))
	log.Printf("End-to-End Latency (without HTTP): Avg=%v\n", avg(l.endToEndWithoutHttp))
	log.Printf("Listener Latency: Avg=%v\n", avg(l.listener))
	log.Printf("Dispatcher Latency: Avg=%v\n", avg(l.dispatcherLatency))
	log.Printf("Worker Latency: Avg=%v\n", avg(l.worker))
}

func avg(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func (l *Latency) addEventLatency(event *event.Event) {
	l.endToEnd = append(l.endToEnd, event.LatencyTimestamps["worker_end"].Sub(event.LatencyTimestamps["http_listener_start"]))
	l.endToEndWithoutHttp = append(l.endToEndWithoutHttp, event.LatencyTimestamps["worker_start"].Sub(event.LatencyTimestamps["http_listener_start"]))
	l.listener = append(l.listener, event.LatencyTimestamps["http_listener_end"].Sub(event.LatencyTimestamps["http_listener_start"]))
	l.dispatcherLatency = append(l.dispatcherLatency, event.LatencyTimestamps["dispatcher_end"].Sub(event.LatencyTimestamps["dispatcher_start"]))
	l.worker = append(l.worker, event.LatencyTimestamps["worker_end"].Sub(event.LatencyTimestamps["worker_start"]))
}
