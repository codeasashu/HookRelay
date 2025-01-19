package subscription

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/codeasashu/HookRelay/internal/target"
)

type Subscription struct {
	ID      string
	OwnerId string         `json:"owner_id" binding:"required"`
	Target  *target.Target `json:"target" binding:"required"`
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
	return nil
}

type EventSubscrptions struct {
	Subscriptions map[string][]*Subscription // Mapping of eventType and Subsciptions
}

var es *EventSubscrptions

func Init() {
	es = &EventSubscrptions{
		Subscriptions: make(map[string][]*Subscription),
	}
}

func (e *EventSubscrptions) Add(eventType string, subscription *Subscription) {
	// @TODO: Check if subscription already exists, by matching md5(target) for given subscription's owner id
	e.Subscriptions[eventType] = append(es.Subscriptions[eventType], subscription)
}

func CreateSubscription(eventType string, subscription *Subscription) error {
	err := subscription.Validate()
	if err != nil {
		return err
	}
	es.Add(eventType, subscription)
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
