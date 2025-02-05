package disruptor

import "time"

type DefaultWaitStrategy struct{}

func NewWaitStrategy() DefaultWaitStrategy      { return DefaultWaitStrategy{} }
func (ws DefaultWaitStrategy) Gate(count int64) { time.Sleep(time.Nanosecond) }
func (ws DefaultWaitStrategy) Idle(count int64) { time.Sleep(time.Microsecond * 50) }
