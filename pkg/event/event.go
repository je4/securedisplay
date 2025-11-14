package event

import (
	"encoding/json"
	"fmt"
	"strings"

	"emperror.dev/errors"
)

type DataInterface interface {
	String() string
	Type() string
}

type Message string

func (m Message) String() string {
	return string(m)
}

var _ DataInterface = Message("")

func (m Message) Type() string {
	return "message"
}

func NewEvent(data DataInterface, target string, token string) (*Event, error) {
	jsonStr, err := json.Marshal(data)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal event data: %v", data)
	}
	return &Event{
		Type:   data.Type(),
		Target: target,
		Token:  token,
		Data:   jsonStr,
	}, nil
}

type Event struct {
	Type   string          `json:"type"`
	Source string          `json:"source"`
	Target string          `json:"target"`
	Token  string          `json:"token"`
	Data   json.RawMessage `json:"data"`
}

func (e *Event) String() string {
	return fmt.Sprintf("%s -> %s", e.Type, e.Target)

}

func (e *Event) Decode() (DataInterface, error) {
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
