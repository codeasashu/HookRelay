package subscription

import (
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/target"
)

var m *metrics.Metrics

type Subscription struct {
	// lock    *sync.RWMutex
	ID      string
	OwnerId string         `json:"owner_id" binding:"required"`
	Target  *target.Target `json:"target" binding:"required"`

	CreatedAt    time.Time
	StartedAt    time.Time
	DispatchedAt time.Time
	CompleteAt   time.Time
}

func (e *Subscription) Dispatch() {
	// e.lock.Lock()
	e.DispatchedAt = time.Now()
	// e.lock.Unlock()
}

func (e *Subscription) Complete() {
	// e.lock.Lock()
	e.CompleteAt = time.Now()
	// e.lock.Unlock()
}

func (s *Subscription) Validate() error {
	if err := target.ValidateTarget(*s.Target); err != nil {
		return err
	}

	// Existing Subscription does not exist
	if subs := GetSubscriptionsByOwner(s.OwnerId); len(subs) > 0 {
		for _, sub := range subs {
			if sub.ID == s.ID {
				return errors.New("Subscription already exists")
			}
		}
	}
	return nil
}

func (s *createSubscription) UnmarshalJSON(data []byte) error {
	type Alias createSubscription
	temp := &struct {
		*Alias
	}{
		Alias: (*Alias)(s),
	}

	if err := json.Unmarshal(data, temp); err != nil {
		return err
	}
	iid, err := temp.Target.GetID()
	if err != nil {
		log.Println(err)
		return err
	}
	temp.ID = iid
	// temp.lock = &sync.RWMutex{}
	temp.CreatedAt = time.Now()
	return nil
}

type EventSubscrptions struct {
	Subscriptions      map[string][]*Subscription // Mapping of eventType and Subsciptions
	TotalSubscriptions int
}

var es *EventSubscrptions

func Init() {
	es = &EventSubscrptions{
		Subscriptions: make(map[string][]*Subscription),
	}
	m = metrics.GetDPInstance()
}

func (e *EventSubscrptions) Add(eventType string, subscription *Subscription) {
	e.Subscriptions[eventType] = append(es.Subscriptions[eventType], subscription)
	e.TotalSubscriptions += len(es.Subscriptions[eventType])
}

func CreateSubscription(eventType string, subscription *Subscription) error {
	err := subscription.Validate()
	if err != nil {
		return err
	}
	es.Subscriptions[eventType] = append(es.Subscriptions[eventType], subscription)
	m.UpdateTotalSubscriptionCount(len(es.Subscriptions[eventType]))
	return nil
}

func GetSubscriptionsByEventType(eventType string) []*Subscription {
	return es.Subscriptions[eventType]
}

func GetSubscriptionsByOwner(ownerId string) []*Subscription {
	found := make([]*Subscription, 0)
	for _, subscriptions := range es.Subscriptions {
		for _, subscription := range subscriptions {
			if subscription.OwnerId == ownerId {
				found = append(found, subscription)
			}
		}
	}
	return found
}
