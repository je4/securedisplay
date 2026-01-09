package client0

import (
	"fmt"
	"net/http"

	"emperror.dev/errors"
	"github.com/je4/securedisplay/pkg/player0"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/sahmad98/go-ringbuffer"
)

type PlayerClient struct {
	status        string
	log           zLogger.ZLogger
	instance      string
	httpServerExt *http.Server
	player        player0.Player
	wsGroup       map[string]*ClientWebsocket
	playerLog     *ringbuffer.RingBuffer
}

func NewClient(instanceName string, log zLogger.ZLogger) *PlayerClient {
	client := &PlayerClient{log: log,
		instance:  instanceName,
		wsGroup:   make(map[string]*ClientWebsocket),
		playerLog: ringbuffer.NewRingBuffer(100),
	}

	return client
}

func (client *PlayerClient) writeBrowserLog(format string, a ...interface{}) {
	client.playerLog.Write(fmt.Sprintf(format, a...))
}

func (client *PlayerClient) getBrowserLog() []string {
	result := []string{}
	client.playerLog.Reader = client.playerLog.Writer
	var i int32
	for ; i < client.playerLog.Size; i++ {
		elem := client.playerLog.Read()
		str, ok := elem.(string)
		if !ok {
			continue
		}
		result = append(result, str)
	}
	return result
}

func (client *PlayerClient) SetPlayer(player player0.Player) error {
	if client.player != nil {
		return errors.New("browser already exists")
	}
	client.player = player
	return nil
}

func (client *PlayerClient) SetGroupWebsocket(group string, ws *ClientWebsocket) {
	client.wsGroup[group] = ws
}

func (client *PlayerClient) DeleteGroupWebsocket(group string) {
	delete(client.wsGroup, group)
}

func (client *PlayerClient) GetGroupWebsocket(group string) (*ClientWebsocket, error) {
	ws, ok := client.wsGroup[group]
	if !ok {
		return nil, errors.New(fmt.Sprintf("no websocket connection for group %v", group))
	}
	return ws, nil

}

func (client *PlayerClient) SendGroupWebsocket(group string, message []byte) error {
	ws, err := client.GetGroupWebsocket(group)
	if err != nil {
		return errors.Wrapf(err, "cannot send to group %v", group)
	}
	ws.send <- message
	return nil

}

func (client *PlayerClient) GetBrowser() (player0.Player, error) {
	if client.player == nil {
		return nil, errors.New("browser not initialized")
	}
	return client.player, nil
}

func (client *PlayerClient) SetStatus(status string) {
	client.status = status
}

func (client *PlayerClient) GetStatus() string {
	if client.status != "" {
		if client.player == nil {
			client.status = ""
		} else {
			if !client.player.IsRunning() {
				client.status = ""
			}
		}
	}
	return client.status
}

func (client *PlayerClient) GetInstance() string {
	return client.instance
}

func (client *PlayerClient) ShutdownPlayer() error {
	if client.player == nil {
		return errors.New("no browser available")
	}
	err := client.player.Close()
	client.player = nil
	return errors.Wrap(err, "error closing player")
}

type MyTransport http.Transport

func (transport *MyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Make the request to the server.
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	resp.Header.Set("Access-Control-Allow-Origin", "*")
	return resp, nil
}

func (client *PlayerClient) Shutdown() error {
	return nil
}
