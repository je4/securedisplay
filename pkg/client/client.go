package client

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"emperror.dev/errors"
	"github.com/je4/securedisplay/pkg/browser"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/sahmad98/go-ringbuffer"
)

type BrowserClient struct {
	status        string
	log           zLogger.ZLogger
	instance      string
	httpServerExt *http.Server
	browser       *browser.Browser
	wsGroup       map[string]*ClientWebsocket
	browserLog    *ringbuffer.RingBuffer
}

func NewClient(instanceName string, log zLogger.ZLogger) *BrowserClient {
	client := &BrowserClient{log: log,
		instance:   instanceName,
		wsGroup:    make(map[string]*ClientWebsocket),
		browserLog: ringbuffer.NewRingBuffer(100),
	}

	return client
}

func (client *BrowserClient) writeBrowserLog(format string, a ...interface{}) {
	client.browserLog.Write(fmt.Sprintf(format, a...))
}

func (client *BrowserClient) getBrowserLog() []string {
	result := []string{}
	client.browserLog.Reader = client.browserLog.Writer
	var i int32
	for ; i < client.browserLog.Size; i++ {
		elem := client.browserLog.Read()
		str, ok := elem.(string)
		if !ok {
			continue
		}
		result = append(result, str)
	}
	return result
}

func (client *BrowserClient) SetBrowser(browser *browser.Browser) error {
	if client.browser != nil {
		return errors.New("browser already exists")
	}
	client.browser = browser
	return nil
}

func (client *BrowserClient) SetGroupWebsocket(group string, ws *ClientWebsocket) {
	client.wsGroup[group] = ws
}

func (client *BrowserClient) DeleteGroupWebsocket(group string) {
	delete(client.wsGroup, group)
}

func (client *BrowserClient) GetGroupWebsocket(group string) (*ClientWebsocket, error) {
	ws, ok := client.wsGroup[group]
	if !ok {
		return nil, errors.New(fmt.Sprintf("no websocket connection for group %v", group))
	}
	return ws, nil

}

func (client *BrowserClient) SendGroupWebsocket(group string, message []byte) error {
	ws, err := client.GetGroupWebsocket(group)
	if err != nil {
		return errors.Wrapf(err, "cannot send to group %v", group)
	}
	ws.send <- message
	return nil

}

func (client *BrowserClient) GetBrowser() (*browser.Browser, error) {
	if client.browser == nil {
		return nil, errors.New("browser not initialized")
	}
	return client.browser, nil
}

func (client *BrowserClient) SetStatus(status string) {
	client.status = status
}

func (client *BrowserClient) GetStatus() string {
	if client.status != "" {
		if client.browser == nil {
			client.status = ""
		} else {
			if !client.browser.IsRunning() {
				client.status = ""
			}
		}
	}
	return client.status
}

func (client *BrowserClient) GetInstance() string {
	return client.instance
}

func (client *BrowserClient) ShutdownBrowser() error {
	if client.browser == nil {
		return errors.New("no browser available")
	}
	client.browser.Close()
	client.browser = nil
	return nil
}

func (client *BrowserClient) browserClick() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client.log.Info().Msg("browserClick()")
		xstr := r.FormValue("x")
		ystr := r.FormValue("y")
		x, err := strconv.ParseInt(xstr, 10, 64)
		if err != nil {
			client.log.Error().Msgf("cannot parse x %v: %v", xstr, err)
			http.Error(w, fmt.Sprintf("cannot parse x %v: %v", xstr, err), http.StatusBadRequest)
		}
		y, err := strconv.ParseInt(ystr, 10, 64)
		if err != nil {
			client.log.Error().Msgf("cannot parse x %v: %v", ystr, err)
			http.Error(w, fmt.Sprintf("cannot parse y %v: %v", ystr, err), http.StatusBadRequest)
		}
		if err := client.browser.MouseClick("", x, y, "", 2*time.Second); err != nil {
			client.log.Error().Msgf("cannot click %v/%v: %v", x, y, err)
			http.Error(w, fmt.Sprintf("cannot click %v/%v: %v", x, y, err), http.StatusInternalServerError)
		}

		w.Header().Add("Content-Type", "application/json")
		io.WriteString(w, `"status":"ok"`)
	}
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

func (client *BrowserClient) screenshot(width int, height int, sigma float64) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client.log.Info().Msg("screenshot()")

		if client.browser == nil {
			client.log.Error().Msg("cannot create screenshot: no browser")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("cannot create screenshot: no browser")))
			return
		}
		buf, mime, err := client.browser.Screenshot(width, height, sigma)
		if err != nil {
			client.log.Error().Msgf("cannot create screenshot: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("cannot create screenshot: %v", err)))
			return
		}
		w.Header().Add("Content-Type", mime)
		if _, err := w.Write(buf); err != nil {
			client.log.Error().Err(err).Msg("cannot write image data")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func (client *BrowserClient) Shutdown() error {
	return nil
}
