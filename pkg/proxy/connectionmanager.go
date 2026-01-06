package proxy

import (
	"slices"
	"sync"

	"emperror.dev/errors"
	"github.com/je4/securedisplay/pkg/event"
	"github.com/je4/utils/v2/pkg/zLogger"
)

func newConnectionManager(debug bool, logger zLogger.ZLogger) *connectionManager {
	cm := &connectionManager{
		debug:         debug,
		wsConns:       make(map[string]*connection),
		wsConnsMu:     sync.Mutex{},
		groups:        make(map[string][]string),
		groupsMu:      sync.RWMutex{},
		logger:        logger,
		senderChannel: make(chan *job, 100),
		workerWG:      sync.WaitGroup{},
	}
	return cm
}

type job struct {
	evt  *event.Event
	dest string
}

type connectionManager struct {
	wsConns       map[string]*connection
	wsConnsMu     sync.Mutex
	groups        map[string][]string
	groupsMu      sync.RWMutex
	debug         bool
	logger        zLogger.ZLogger
	senderChannel chan *job
	workerWG      sync.WaitGroup
}

func (manager *connectionManager) start(numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		manager.logger.Debug().Msgf("Starting worker #%d", i)
		go manager.worker(i, manager.senderChannel)
	}
}

func (manager *connectionManager) close() {
	close(manager.senderChannel)
	manager.workerWG.Wait()
}
func (manager *connectionManager) worker(id int, jobs <-chan *job) {
	manager.workerWG.Add(1)
	defer manager.workerWG.Done()
	for j := range jobs {
		manager.logger.Debug().Msgf("worker #%d forwarding event %s %s -> %s to %s", id, j.evt.Type, j.evt.GetSource(), j.evt.GetTarget(), j.dest)
		if err := manager.sendWS(j.dest, j.evt); err != nil {
			manager.logger.Error().Err(err).Msgf("worker #%d failed to send event", id)
			continue
		}
		manager.logger.Debug().Msgf("worker #%d event %s %s -> %s to %s forwarded", id, j.evt.Type, j.evt.GetSource(), j.evt.GetTarget(), j.dest)
	}
}

func (manager *connectionManager) send(evt *event.Event) error {
	dests, ok := manager.groups[evt.GetTarget()]
	if !ok {
		manager.senderChannel <- &job{evt: evt, dest: evt.GetTarget()}
		return nil
	}
	for _, dest := range dests {
		manager.senderChannel <- &job{evt: evt, dest: dest}
	}
	return nil
}

func (manager *connectionManager) sendWS(dest string, evt *event.Event) error {
	conn, ok := manager.getWSConn(dest)
	if !ok {
		return errors.Errorf("no connection for destination %s", dest)
	}
	if err := conn.Conn.WriteJSON(evt); err != nil {
		return errors.Wrapf(err, "failed to send event %s to %s->%s", evt.GetType(), evt.GetSource(), evt.GetTarget())
	}
	return nil
}

func (manager *connectionManager) addWSConn(c *connection) error {
	name := c.Name
	if conn, ok := manager.getWSConn(name); ok {
		if conn.Secure && !c.Secure {
			return errors.Errorf("cannot replace secure connection %s with an insecure connection", name)
		}
		manager.closeWSConn(conn)
		manager.logger.Warn().Msgf("replacing connection %s", name)
		//return errors.Errorf("cannot add connection %s, already have connectin %s", name, conn.Name)
	}
	manager.wsConnsMu.Lock()
	defer manager.wsConnsMu.Unlock()
	manager.logger.Debug().Msgf("Adding connection %s", name)
	manager.wsConns[name] = c
	return nil
}

func (manager *connectionManager) getWSConn(name string) (*connection, bool) {
	manager.wsConnsMu.Lock()
	defer manager.wsConnsMu.Unlock()
	manager.logger.Debug().Msgf("Getting connection %s", name)
	conn, ok := manager.wsConns[name]
	return conn, ok
}

func (manager *connectionManager) removeWSConn(name string) {
	manager.wsConnsMu.Lock()
	defer manager.wsConnsMu.Unlock()
	manager.logger.Debug().Msgf("Removing connection %s", name)
	delete(manager.wsConns, name)
}

func (manager *connectionManager) closeWSConn(wsConn *connection) {
	manager.wsConnsMu.Lock()
	defer manager.wsConnsMu.Unlock()
	if conn, ok := manager.wsConns[wsConn.Name]; ok {
		if conn.Conn.RemoteAddr() != wsConn.Conn.RemoteAddr() {
			manager.logger.Debug().Msgf("connection %s[%s] already closed.", wsConn.Name, wsConn.Conn.RemoteAddr())
			return
		}
		manager.logger.Debug().Msgf("Closing connection %s[%s]", wsConn.Name, wsConn.Conn.RemoteAddr())
		if err := conn.Close(); err != nil {
			manager.logger.Error().Err(err).Msg("Failed to close connection")
		}
		delete(manager.wsConns, wsConn.Name)
	}
}

func (manager *connectionManager) AddToGroup(name string, group string) {
	manager.groupsMu.Lock()
	defer manager.groupsMu.Unlock()
	if _, ok := manager.groups[group]; !ok {
		manager.groups[group] = []string{}
	}
	if !slices.Contains(manager.groups[group], name) {
		manager.groups[group] = append(manager.groups[group], name)
	}
}

func (manager *connectionManager) RemoveFromGroup(name string, group string) {
	manager.groupsMu.Lock()
	defer manager.groupsMu.Unlock()
	if _, ok := manager.groups[group]; !ok {
		return
	}
	slices.DeleteFunc(manager.groups[group], func(s string) bool {
		return s == name
	})
}

func (manager *connectionManager) RemoveFromGroups(name string) {
	manager.groupsMu.Lock()
	defer manager.groupsMu.Unlock()
	for group, _ := range manager.groups {
		slices.DeleteFunc(manager.groups[group], func(s string) bool {
			return s == name
		})
	}
}
