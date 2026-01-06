package event

const TypeAttach EventType = "attach"
const TypeDetach EventType = "detach"
const TypeStringMessage EventType = "message"
const TypeNTPQuery EventType = "ntp-query"
const TypeNTPResponse EventType = "ntp-response"
const TypeNTPError EventType = "ntp-error"

func NewGenericStringMessage(t EventType, msg string) DataInterface {
	return &GenericStringMessage{
		type_: t,
		msg:   msg,
	}
}

type GenericStringMessage struct {
	type_ EventType
	msg   string
}

func (m *GenericStringMessage) String() string {
	return m.msg
}
func (m *GenericStringMessage) Type() EventType {
	return m.type_
}

var _ DataInterface = (*GenericStringMessage)(nil)
