package subscription

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/codeasashu/HookRelay/internal/target"
)

type LegacySubscription struct {
	ID           string         `db:"id"`
	CompanyId    string         `db:"company_id"`
	TargetUrl    string         `db:"url"`
	TargetMethod int            `db:"request"`
	HeaderType   int            `db:"simple"`
	Headers      sql.NullString `db:"headers"`
	Auth         int            `db:"auth"`
	Crendentials string         `db:"credentials"`
	IsActive     int            `db:"is_active"`
	ServiceType  int            `db:"service_type"`
	Created      time.Time      `json:"created"`
}

func (l *LegacySubscription) GetEventTypes() []string {
	var eventType string
	if l.ServiceType == 2 {
		eventType = "webhook.incall"
	} else {
		eventType = "webhook.aftercall"
	}
	return []string{eventType}
}

func (l *LegacySubscription) GetHeaders() map[string]string {
	headerMap := make(map[string]string)
	headerMap["Content-Type"] = "application/json"
	if l.Headers.Valid {
		var headers []string
		err := json.Unmarshal([]byte(l.Headers.String), &headers)
		if err != nil {
			return headerMap
		}

		for _, header := range headers {
			kv := strings.Split(header, ":")
			headerMap[kv[0]] = kv[1]
		}

		switch l.HeaderType {
		case 1:
			headerMap["Content-Type"] = "multipart/form-data"
		case 2:
			headerMap["Content-Type"] = "application/json"
		case 3:
			headerMap["Content-Type"] = "application/x-www-form-urlencoded"
		default:
			headerMap["Content-Type"] = "application/json"
		}
		return headerMap
	}
	return headerMap
}

func (l *LegacySubscription) GetMethod() string {
	var method string
	if l.TargetMethod == 2 {
		method = "GET"
	} else {
		method = "POST"
	}
	return method
}

func (l *LegacySubscription) GetAuth() target.HTTPBasicAuth {
	auth := target.HTTPBasicAuth{
		Username: "",
		Password: "",
	}
	if len(l.Crendentials) > 0 {
		err := json.Unmarshal([]byte(l.Crendentials), &auth)
		if err != nil {
			return auth
		}
	}
	return auth
}

func (l *LegacySubscription) GetTarget() (*target.Target, error) {
	return target.NewHTTPTarget(
		l.TargetUrl,
		l.GetMethod(),
		l.GetHeaders(),
		l.GetAuth(),
	)
}

func (l *LegacySubscription) ConvertToSubscription() (*Subscription, error) {
	t, err := l.GetTarget()
	if err != nil {
		return nil, err
	}
	return &Subscription{
		ID:         l.ID,
		OwnerId:    l.CompanyId,
		EventTypes: l.GetEventTypes(),
		Target:     t,
		Status:     l.IsActive,
		CreatedAt:  l.Created,
	}, nil
}
