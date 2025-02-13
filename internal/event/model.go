package event

import (
	"errors"

	"github.com/codeasashu/HookRelay/internal/database"
)

type EventModel struct {
	db database.Database
}

var ErrEventNotCreated = errors.New("subscription could not be created")

func NewEventModel(db database.Database) *EventModel {
	return &EventModel{db: db}
}
