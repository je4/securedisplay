package proxy

import (
	"context"
	"crypto/tls"
	"html/template"
	"io/fs"
	"net/http"
	"slices"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/je4/utils/v2/pkg/zLogger"

	"github.com/gorilla/websocket"
)

func NewSocketServer(addr string, numWorkers int, ntpServer string, staticFS fs.FS, templateFS fs.FS, debug bool, logger zLogger.ZLogger) (*SocketServer, error) {
	ss := &SocketServer{
		Addr:              addr,
		upgrader:          websocket.Upgrader{},
		logger:            logger,
		templates:         make(map[string]*template.Template),
		echoConns:         make([]*websocket.Conn, 0),
		echoConnsMu:       sync.Mutex{},
		wsConns:           make(map[string]*connection),
		wsConnsMu:         sync.Mutex{},
		debug:             debug,
		groups:            make(map[string][]string),
		groupsMu:          sync.RWMutex{},
		connectionManager: newConnectionManager(debug, logger),
		numWorkers:        numWorkers,
		ntpServer:         ntpServer,
		ntpFunc:           NewNTPConnection(ntpServer, "", "", "", 0, 0),
		templateFS:        templateFS,
		staticFS:          staticFS,
	}
	return ss, nil
}

type SocketServer struct {
	Addr              string
	upgrader          websocket.Upgrader
	srv               *http.Server
	logger            zLogger.ZLogger
	wg                sync.WaitGroup
	templates         map[string]*template.Template
	echoConns         []*websocket.Conn
	echoConnsMu       sync.Mutex
	wsConns           map[string]*connection
	wsConnsMu         sync.Mutex
	debug             bool
	groups            map[string][]string
	groupsMu          sync.RWMutex
	connectionManager *connectionManager
	numWorkers        int
	ntpServer         string
	ntpFunc           func(data []byte) ([]byte, error)
	templateFS        fs.FS
	staticFS          fs.FS
}

func (ss *SocketServer) getTemplate(name string) (*template.Template, error) {
	if tmpl, ok := ss.templates[name]; ok {
		return tmpl, nil
	}
	tmpl, err := template.New(name).ParseFS(ss.templateFS, name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse template %s", name)
	}
	if !ss.debug {
		ss.templates[name] = tmpl
	}
	return tmpl, nil
}

func (srv *SocketServer) Start(tlsConfig *tls.Config) error {
	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowHeaders:     []string{"*"},
		AllowCredentials: true,
		AllowWebSockets:  true,
	}))
	srv.connectionManager.start(srv.numWorkers)
	router.StaticFS("/static", http.FS(srv.staticFS))
	router.GET("/control/:name", func(c *gin.Context) {
		var name = c.Param("name")
		if name == "" {
			name = "noname"
		}
		controlTemplate, err := srv.getTemplate("control.gohtml")
		if err != nil {
			srv.logger.Error().Err(err).Msgf("Failed to get template control.gohtml")
			return
		}
		if err := controlTemplate.Execute(c.Writer, struct{ Addr, Name string }{
			Addr: "ws://" + c.Request.Host + "/ws/" + name,
			Name: name}); err != nil {
			srv.logger.Error().Err(err).Msg("Failed to execute template")
		}
	})
	router.GET("/roundaudio/:name", func(c *gin.Context) {
		var name = c.Param("name")
		if name == "" {
			name = "noname"
		}
		roundaudioTemplate, err := srv.getTemplate("roundaudio.gohtml")
		if err != nil {
			srv.logger.Error().Err(err).Msgf("Failed to get template roundaudio.gohtml")
			return
		}
		if err := roundaudioTemplate.Execute(c.Writer, struct{ Addr, Name string }{
			Addr: "ws://" + c.Request.Host + "/ws/" + name,
			Name: name}); err != nil {
			srv.logger.Error().Err(err).Msg("Failed to execute template")
		}
	})
	router.GET("/test/:name", func(c *gin.Context) {
		var name = c.Param("name")
		if name == "" {
			name = "noname"
		}
		testTemplate, err := srv.getTemplate("echo.gohtml")
		if err != nil {
			srv.logger.Error().Err(err).Msgf("Failed to get template %s", name)
			return
		}
		if err := testTemplate.Execute(c.Writer, struct{ Addr, Name string }{
			Addr: "ws://" + c.Request.Host + "/ws/" + name,
			Name: name}); err != nil {
			srv.logger.Error().Err(err).Msg("Failed to execute template")
		}
	})
	router.GET("/echo", srv.echo)
	router.GET("/ws/:name", srv.ws)
	srv.srv = &http.Server{
		Addr:      srv.Addr,
		Handler:   router,
		TLSConfig: tlsConfig,
	}
	go func() {
		srv.wg.Add(1)
		defer srv.wg.Done()
		if tlsConfig == nil {
			srv.logger.Info().Msgf("Starting server on http://%s", srv.Addr)
			if err := srv.srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				srv.logger.Error().Err(err).Msg("Server error")
			} else {
				srv.logger.Info().Msg("Server closed")
			}
		} else {
			srv.logger.Info().Msgf("Starting server on https://%s", srv.Addr)
			if err := srv.srv.ListenAndServeTLS("", ""); !errors.Is(err, http.ErrServerClosed) {
				srv.logger.Error().Err(err).Msg("Server error")
			} else {
				srv.logger.Info().Msg("Server closed")
			}
		}
	}()
	return nil
}

func (srv *SocketServer) Stop() error {
	srv.echoConnsMu.Lock()
	defer srv.echoConnsMu.Unlock()

	if srv.srv == nil {
		return errors.New("server not started")
	}
	srv.connectionManager.close()
	srv.logger.Info().Msg("Stopping server")
	for _, conn := range srv.echoConns {
		srv.logger.Info().Msgf("Closing connection %v", conn.RemoteAddr())
		if err := conn.Close(); err != nil {
			srv.logger.Error().Err(err).Msg("Failed to close connection")
		}
	}
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	if err := srv.srv.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "failed to shutdown server")
	}
	srv.wg.Wait()
	return nil
}

func (srv *SocketServer) addEchoConn(c *websocket.Conn) {
	srv.echoConnsMu.Lock()
	defer srv.echoConnsMu.Unlock()
	srv.echoConns = append(srv.echoConns, c)
}

func (srv *SocketServer) removeEchoConn(c *websocket.Conn) {
	srv.echoConnsMu.Lock()
	defer srv.echoConnsMu.Unlock()
	for i, conn := range srv.echoConns {
		if conn == c {
			srv.echoConns = append(srv.echoConns[:i], srv.echoConns[i+1:]...)
			break
		}
	}
}

func (srv *SocketServer) closeEchoConn(c *websocket.Conn) {
	srv.echoConnsMu.Lock()
	defer srv.echoConnsMu.Unlock()
	for i, conn := range srv.echoConns {
		if conn == c {
			srv.echoConns = append(srv.echoConns[:i], srv.echoConns[i+1:]...)
			if err := c.Close(); err != nil {
				srv.logger.Error().Err(err).Msg("Failed to close connection")
			}
			break
		}
	}
}

func (srv *SocketServer) _addWSConn(c *connection) error {
	name := c.Name
	if conn, ok := srv._getWSConn(name); ok {
		if conn.Secure && !c.Secure {
			return errors.Errorf("cannot replace secure connection %s with an insecure connection", name)
		}
		srv._closeWSConn(conn)
		srv.logger.Warn().Msgf("replacing connection %s", name)
		//return errors.Errorf("cannot add connection %s, already have connectin %s", name, conn.Name)
	}
	srv.wsConnsMu.Lock()
	defer srv.wsConnsMu.Unlock()
	srv.logger.Debug().Msgf("Adding connection %s", name)
	srv.wsConns[name] = c
	return nil
}

func (srv *SocketServer) _getWSConn(name string) (*connection, bool) {
	srv.wsConnsMu.Lock()
	defer srv.wsConnsMu.Unlock()
	srv.logger.Debug().Msgf("Getting connection %s", name)
	conn, ok := srv.wsConns[name]
	return conn, ok
}

func (srv *SocketServer) _removeWSConn(name string) {
	srv.wsConnsMu.Lock()
	defer srv.wsConnsMu.Unlock()
	srv.logger.Debug().Msgf("Removing connection %s", name)
	delete(srv.wsConns, name)
}

func (srv *SocketServer) _closeWSConn(wsConn *connection) {
	srv.wsConnsMu.Lock()
	defer srv.wsConnsMu.Unlock()
	if conn, ok := srv.wsConns[wsConn.Name]; ok {
		if conn.Conn.RemoteAddr() != wsConn.Conn.RemoteAddr() {
			srv.logger.Debug().Msgf("connection %s[%s] already closed.", wsConn.Name, wsConn.Conn.RemoteAddr())
			return
		}
		srv.logger.Debug().Msgf("Closing connection %s[%s]", wsConn.Name, wsConn.Conn.RemoteAddr())
		if err := conn.Close(); err != nil {
			srv.logger.Error().Err(err).Msg("Failed to close connection")
		}
		delete(srv.wsConns, wsConn.Name)
	}
}

/*
func (srv *SocketServer) ping(ctx *gin.Context) {
	conn, err := srv.upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		srv.logger.Error().Err(err).Msg("Failed to upgrade connection")
		return
	}
	srv.addEchoConn(conn)
	defer srv.closeEchoConn(conn)
	srv.logger.Debug().Msg("Ping connection established")
}
*/

func (srv *SocketServer) upgrade(ctx *gin.Context, name string, pingInterval time.Duration) (*websocket.Conn, error) {
	conn, err := srv.upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to upgrade connection")
	}
	if err := func(connectionCTX *gin.Context, connectionName string) error {
		conn.SetCloseHandler(func(code int, text string) error {
			srv.logger.Debug().Msgf("connection closed from remote %s[%s]: %d %s", connectionName, connectionCTX.Request.RemoteAddr, code, text)
			srv.closeEchoConn(conn)
			return nil
		})

		// Set ping handler
		conn.SetPingHandler(func(appData string) error {
			srv.logger.Debug().Msgf("Received ping from client %s[%s]: %s", connectionName, connectionCTX.Request.RemoteAddr, appData)
			return nil
		})

		// Set pong handler
		conn.SetPongHandler(func(appData string) error {
			srv.logger.Debug().Msgf("Received pong from client %s[%s]: %s", connectionName, connectionCTX.Request.RemoteAddr, appData)
			return nil
		})
		go func() {
			for {
				if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second)); err != nil {
					srv.logger.Error().Err(err).Msg("Failed to send ping")
					break
				}
				select {
				case <-time.After(pingInterval):
				case <-ctx.Request.Context().Done():
					srv.logger.Debug().Msg("Context done, stopping ping")
					return
				}
				srv.logger.Debug().Msgf("Sent ping to client %s[%s]", connectionName, connectionCTX.Request.RemoteAddr)
			}
		}()
		return nil
	}(ctx, name); err != nil {
		return nil, errors.Wrap(err, "failed to set connection handlers")
	}

	return conn, nil
}

func (srv *SocketServer) echo(ctx *gin.Context) {
	conn, err := srv.upgrade(ctx, "", 10*time.Second)
	if err != nil {
		srv.logger.Error().Err(err).Msg("Failed to upgrade connection")
		return
	}
	srv.addEchoConn(conn)
	defer srv.closeEchoConn(conn)

	for {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(errors.Cause(err), websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				srv.logger.Debug().Err(err).Msg("connection closed by client")
			} else {
				srv.logger.Error().Err(err).Msg("Failed to read echo message")
			}
			break
		}
		srv.logger.Debug().Msgf("Received message: %s", message)
		if err = conn.WriteMessage(mt, message); err != nil {
			srv.logger.Error().Err(err).Msg("Failed to write message")
			break
		}
	}
}

func (srv *SocketServer) _AddToGroup(name string, group string) {
	srv.groupsMu.Lock()
	defer srv.groupsMu.Unlock()
	if _, ok := srv.groups[group]; !ok {
		srv.groups[group] = []string{}
	}
	if !slices.Contains(srv.groups[group], name) {
		srv.groups[group] = append(srv.groups[group], name)
	}
}

func (srv *SocketServer) _RemoveFromGroup(name string, group string) {
	srv.groupsMu.Lock()
	defer srv.groupsMu.Unlock()
	if _, ok := srv.groups[group]; !ok {
		return
	}
	slices.DeleteFunc(srv.groups[group], func(s string) bool {
		return s == name
	})
}

func (srv *SocketServer) _RemoveFromGroups(name string) {
	srv.groupsMu.Lock()
	defer srv.groupsMu.Unlock()
	for group, _ := range srv.groups {
		slices.DeleteFunc(srv.groups[group], func(s string) bool {
			return s == name
		})
	}
}
