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
	ID         string
	OwnerId    string          `json:"owner_id" binding:"required" db:"owner_id"`
	Target     *target.Target  `json:"target"`
	EventTypes []string        `json:"event_types"`
	Filters    json.RawMessage `json:"filters,omitempty" db:"filters"`
	Tags       []string        `json:"tags"`
	Status     int             `json:"status" db:"status"`
	CreatedAt  time.Time       `json:"created_at"`
}

type ReadSubscription struct {
	Target *target.HTTPDetails `json:"target" binding:"required"`
	*Subscription
}

func genSHA(str string) (string, error) {
	h := sha1.New()
	if _, err := h.Write([]byte(str)); err != nil {
		return "", err
	}
	sha1_hash := hex.EncodeToString(h.Sum(nil))
	return sha1_hash, nil
}

func (s *ReadSubscription) UnmarshalJSON(data []byte) error {
	type Alias ReadSubscription

	temp := &struct {
		*Alias
	}{
		Alias: (*Alias)(s),
	}

	if err := json.Unmarshal(data, temp); err != nil {
		return err
	}

	if temp.OwnerId == "" {
		return errors.New("owner_id is required")
	}

	iid, err := genSHA(temp.OwnerId + ":" + temp.Target.URL)
	if err != nil {
		slog.Error("failed to get target id", "err", err)
		return err
	}
	temp.ID = iid
	if len(temp.EventTypes) == 0 {
		temp.EventTypes = []string{"*"} // subscribe to all events
	}
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
