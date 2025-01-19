package target

import (
	"crypto/sha1"
	"encoding/hex"
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

func genSHA(str string) (string, error) {
	h := sha1.New()
	if _, err := h.Write([]byte(str)); err != nil {
		return "", err
	}
	sha1_hash := hex.EncodeToString(h.Sum(nil))
	return sha1_hash, nil
}

func (t *Target) GetID() (string, error) {
	if t == nil {
		return "", errors.New("target is nil")
	}
	if t.Type == TargetHTTP {
		if t.HTTPDetails == nil {
			return "", errors.New("HTTP details are nil")
		}
		return genSHA(t.HTTPDetails.URL)
	}
	return "", errors.New("target type is invalid")
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
