package subscription

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/codeasashu/HookRelay/internal/cli"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/target"
)

var m *metrics.Metrics

type Subscription struct {
	// lock    *sync.RWMutex
	ID         string
	OwnerId    string         `json:"owner_id" binding:"required" db:"owner_id"`
	Target     *target.Target `json:"target" binding:"required"`
	EventTypes []string       `json:"event_types"`
	Tags       []string       `json:"tags"`

	Status       int `db:"status"`
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

func genSHA(str string) (string, error) {
	h := sha1.New()
	if _, err := h.Write([]byte(str)); err != nil {
		return "", err
	}
	sha1_hash := hex.EncodeToString(h.Sum(nil))
	return sha1_hash, nil
}

func GetID(owner string, t *target.Target) (string, error) {
	if t == nil {
		return "", errors.New("subscription is nil")
	}
	if t.Type == target.TargetHTTP {
		if t.HTTPDetails == nil {
			return "", errors.New("HTTP details are nil")
		}
		return genSHA(owner + ":" + t.HTTPDetails.URL)
	}
	return "", errors.New("target type is invalid")
}

func (s *Subscription) UnmarshalJSON(data []byte) error {
	type Alias Subscription
	temp := &struct {
		*Alias
	}{
		Alias: (*Alias)(s),
	}

	if err := json.Unmarshal(data, temp); err != nil {
		return err
	}
	iid, err := GetID(temp.OwnerId, temp.Target)
	if err != nil {
		slog.Error("failed to get target id", "err", err)
		return err
	}
	temp.ID = iid
	if len(temp.EventTypes) == 0 {
		temp.EventTypes = []string{"*"} // subscribe to all events
	}
	// temp.lock = &sync.RWMutex{}
	temp.CreatedAt = time.Now()
	return nil
}

func Init() {
	m = metrics.GetDPInstance()
}

//	func (e *EventSubscrptions) Add(eventType string, subscription *Subscription) {
//		e.Subscriptions[eventType] = append(es.Subscriptions[eventType], subscription)
//		e.TotalSubscriptions += len(es.Subscriptions[eventType])
//	}
func CreateSubscription(app *cli.App, cs *Subscription) error {
	model := NewSubscriptionModel(app.DB)
	if err := model.CreateSubscription(cs); err != nil {
		return err
	}
	// m.UpdateTotalSubscriptionCount(1)
	return nil
}

// func GetSubscriptionsByEventType(eventType string) []*Subscription {
// 	return es.Subscriptions[eventType]
// }
//
// func GetSubscriptionsByOwner(ownerId string) []*Subscription {
// 	found := make([]*Subscription, 0)
// 	for _, subscriptions := range es.Subscriptions {
// 		for _, subscription := range subscriptions {
// 			if subscription.OwnerId == ownerId {
// 				found = append(found, subscription)
// 			}
// 		}
// 	}
// 	return found
// }
