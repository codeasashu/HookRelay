package target

import (
	"errors"
)

type (
	TargetType   string
	TargetStatus string
)

const (
	StatusOK  TargetStatus = "success"
	StatusErr TargetStatus = "error"
)

// Only HTTP target is supported for now
const DefaultTargetType = TargetHTTP

type Target struct {
	Type         TargetType          `json:"type"`
	HTTPDetails  *HTTPDetails        `json:"http_details,omitempty"`
	HTTPResponse *HTTPTargetResponse `json:"http_response,omitempty"`
	// WebSocketDetails *WebSocketDetails `json:"websocket_details,omitempty"`
}

func NewHTTPTarget(url string, method string) (*Target, error) {
	if url == "" {
		return nil, errors.New("url is empty")
	}
	t := &Target{
		Type: TargetHTTP,
		HTTPDetails: &HTTPDetails{
			URL:    url,
			Method: HTTPMethod(method),
		},
	}
	return t, nil
}

func ValidateTarget(target Target) error {
	switch target.Type {
	case TargetHTTP:
		if target.HTTPDetails == nil {
			return errors.New("HTTP details must be provided for HTTP target")
		}
		// if target.WebSocketDetails != nil {
		// 	return errors.New("only HTTP details should be set for HTTP target")
		// }
	// case TargetWebSocket:
	//     if target.WebSocketDetails == nil {
	//         return errors.New("WebSocket details must be provided for WebSocket target")
	//     }
	//     if target.HTTPDetails != nil || target.PubSubDetails != nil || target.GRPCDetails != nil {
	//         return errors.New("only WebSocket details should be set for WebSocket target")
	//     }
	default:
		return errors.New("unknown target type")
	}
	return nil
}
