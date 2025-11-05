package server

import (
	"context"
	"crypto/tls"
	"html/template"
	"net/http"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/je4/utils/v2/pkg/zLogger"

	"github.com/gorilla/websocket"
)

func NewSocketServer(addr string, logger zLogger.ZLogger) (*SocketServer, error) {
	tpl, err := template.New("home").Parse(echoTemplate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse echo home template")
	}
	return &SocketServer{
		Addr:         addr,
		upgrader:     websocket.Upgrader{},
		logger:       logger,
		homeTemplate: tpl,
		echoConns:    make([]*websocket.Conn, 0),
		echoConnsMu:  sync.Mutex{},
		wsConns:      make(map[string]*Connection),
		wsConnsMu:    sync.Mutex{},
	}, nil
}

func NewConnection(conn *websocket.Conn, name string, secure bool) *Connection {
	return &Connection{
		Secure: secure,
		Conn:   conn,
		Name:   name,
	}
}

type Connection struct {
	Secure bool
	Conn   *websocket.Conn
	Name   string
}

func (c *Connection) Close() error {
	if c.Conn != nil {
		return errors.WithStack(c.Conn.Close())
	}
	return nil
}

type SocketServer struct {
	Addr         string
	upgrader     websocket.Upgrader
	srv          *http.Server
	logger       zLogger.ZLogger
	wg           sync.WaitGroup
	homeTemplate *template.Template
	echoConns    []*websocket.Conn
	echoConnsMu  sync.Mutex
	wsConns      map[string]*Connection
	wsConnsMu    sync.Mutex
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
	router.GET("/", func(c *gin.Context) {
		if err := srv.homeTemplate.Execute(c.Writer, "ws://"+c.Request.Host+"/echo"); err != nil {
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
			if err := c.Close(); err != nil {
				srv.logger.Error().Err(err).Msg("Failed to close connection")
			}
			srv.echoConns = append(srv.echoConns[:i], srv.echoConns[i+1:]...)
		}
	}
}

func (srv *SocketServer) addWSConn(c *Connection, name string) error {
	if conn, ok := srv.getWSConn(name); ok {
		if conn.Secure && !c.Secure {
			return errors.Errorf("cannot add insecure connection %s, already have secure connection %s", name, conn.Name)
		}
		srv.closeWSConn(name)
	}
	srv.wsConnsMu.Lock()
	defer srv.wsConnsMu.Unlock()
	srv.logger.Debug().Msgf("Adding connection %s", name)
	srv.wsConns[name] = c
	return nil
}

func (srv *SocketServer) getWSConn(name string) (*Connection, bool) {
	srv.wsConnsMu.Lock()
	defer srv.wsConnsMu.Unlock()
	srv.logger.Debug().Msgf("Getting connection %s", name)
	conn, ok := srv.wsConns[name]
	return conn, ok
}

func (srv *SocketServer) removeWSConn(name string) {
	srv.wsConnsMu.Lock()
	defer srv.wsConnsMu.Unlock()
	srv.logger.Debug().Msgf("Removing connection %s", name)
	delete(srv.wsConns, name)
}

func (srv *SocketServer) closeWSConn(name string) {
	srv.wsConnsMu.Lock()
	defer srv.wsConnsMu.Unlock()
	if conn, ok := srv.wsConns[name]; ok {
		srv.logger.Debug().Msgf("Closing connection %s", name)
		if err := conn.Close(); err != nil {
			srv.logger.Error().Err(err).Msg("Failed to close connection")
		}
		delete(srv.wsConns, name)
	}
}

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

func (srv *SocketServer) upgrade(ctx *gin.Context, pingInterval time.Duration) (*websocket.Conn, error) {
	conn, err := srv.upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to upgrade connection")
	}
	conn.SetCloseHandler(func(code int, text string) error {
		srv.logger.Debug().Msgf("Connection closed from remote: %d %s", code, text)
		srv.closeEchoConn(conn)
		return nil
	})

	// Set ping handler
	conn.SetPingHandler(func(appData string) error {
		srv.logger.Debug().Msgf("Received ping from client: %s", appData)
		return nil
	})

	// Set pong handler
	conn.SetPongHandler(func(appData string) error {
		srv.logger.Debug().Msgf("Received pong from client: %s", appData)
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
			srv.logger.Debug().Msg("Sent ping to client")
		}
	}()

	return conn, nil
}

func (srv *SocketServer) echo(ctx *gin.Context) {
	conn, err := srv.upgrade(ctx, 10*time.Second)
	if err != nil {
		srv.logger.Error().Err(err).Msg("Failed to upgrade connection")
		return
	}
	srv.addEchoConn(conn)
	defer srv.closeEchoConn(conn)

	for {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				srv.logger.Debug().Err(err).Msg("Connection closed by client")
			} else {
				srv.logger.Error().Err(err).Msg("Failed to read message")
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
