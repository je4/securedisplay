package event

import (
	"emperror.dev/errors"
	"encoding/json"
	"fmt"
	"strings"
)

type Message string

func (m Message) String() string {
	return string(m)
}

type Event struct {
	Type   string          `json:"type"`
	Target string          `json:"target"`
	Token  string          `json:"token"`
	Data   json.RawMessage `json:"data"`
}

func (e *Event) String() string {
	return fmt.Sprintf("%s -> %s", e.Type, e.Target)

}

func (e *Event) Decode() (any, error) {
	switch strings.ToLower(e.Type) {
	case "message":
		var msg Message
		if err := json.Unmarshal(e.Data, &msg); err != nil {
			return nil, errors.Wrapf(err, "cannot unmarshal event message: %v", e.Data)
		}
		return msg, nil
	default:
		return nil, errors.Errorf("unknown event type: %s", e.Type)
	}
}
