package event

type EventType string

const TypeAttach EventType = "attach"
const TypeDetach EventType = "detach"
const TypeStringMessage EventType = "message"
const TypeNTPQuery EventType = "ntp-query"
const TypeNTPResponse EventType = "ntp-response"
const TypeNTPError EventType = "ntp-error"
const TypeBrowserNavigate EventType = "browser-navigate"
