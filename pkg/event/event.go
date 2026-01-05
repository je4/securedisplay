package event

import (
	"encoding/json"
	"fmt"

	"emperror.dev/errors"
)

type EventType string

type DataInterface interface {
	String() string
	Type() EventType
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
	Type   EventType       `json:"type"`
	Source string          `json:"source"`
	Target string          `json:"target"`
	Token  string          `json:"token"`
	Data   json.RawMessage `json:"data"`
}

func (e *Event) String() string {
	return fmt.Sprintf("%s -> %s", e.Type, e.Target)

}

func (e *Event) GetType() EventType {
	return e.Type
}

func (e *Event) GetSource() string {
	return e.Source
}

func (e *Event) GetTarget() string {
	return e.Target
}

func (e *Event) GetToken() string {
	return e.Token
}

func (e *Event) GetData() (interface{}, error) {
	switch e.Type {
	case TypeStringMessage, TypeAttach, TypeDetach:
		var msg string
		if err := json.Unmarshal(e.Data, &msg); err != nil {
			return nil, errors.Wrapf(err, "cannot unmarshal StringMessage event message: %v", e.Data)
		}
		return msg, nil
	default:
		return nil, errors.Errorf("unknown event type: %v", e.Type)
	}
}
