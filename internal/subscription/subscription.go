package subscription

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/codeasashu/HookRelay/internal/database"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/target"
	"github.com/gin-gonic/gin"
)

var m *metrics.Metrics

type Subscription struct {
	router *gin.Engine
	db     database.Database
}

type SubscriptionHandler interface {
	FindSubscribers(eventType, ownerID string, isLegacy bool) ([]Subscriber, error)
}

func NewSubscription(f *app.HookRelayApp) (*Subscription, error) {
	return &Subscription{db: f.SubscriptionDb, router: f.Router}, nil
}

type Subscriber struct {
	ID         string
	OwnerId    string          `json:"owner_id" binding:"required" db:"owner_id"`
	Target     *target.Target  `json:"target"`
	EventTypes []string        `json:"event_types"`
	Filters    json.RawMessage `json:"filters,omitempty" db:"filters"`
	Tags       []string        `json:"tags"`
	Status     int             `json:"status" db:"status"`
	CreatedAt  time.Time       `json:"created_at"`

	db database.Database
}

type ReadSubscriber struct {
	Target *target.HTTPDetails `json:"target" binding:"required"`
	*Subscriber
}

func genSHA(str string) (string, error) {
	h := sha1.New()
	if _, err := h.Write([]byte(str)); err != nil {
		return "", err
	}
	sha1_hash := hex.EncodeToString(h.Sum(nil))
	return sha1_hash, nil
}

func (s *ReadSubscriber) UnmarshalJSON(data []byte) error {
	type Alias ReadSubscriber

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

// func CreateSubscriber(app *cli.App, cs *Subscriber) error {
// 	model := NewSubscriptionModel(app.DB)
// 	if err := model.CreateSubscriber(cs); err != nil {
// 		return err
// 	}
// 	return nil
// }
