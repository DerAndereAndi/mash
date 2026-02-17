package runner

import "github.com/mash-protocol/mash-go/internal/testharness/engine"

type mainConnAccessor interface {
	Main() *Connection
	SetMain(conn *Connection)
}

func setMainControlChannel(pool mainConnAccessor, conn *Connection, state *engine.ExecutionState) *Connection {
	if conn == nil {
		conn = &Connection{}
	}
	pool.SetMain(conn)
	if state != nil {
		state.Set(StateConnection, conn)
	}
	return conn
}

func detachMainControlChannel(pool mainConnAccessor, state *engine.ExecutionState) *Connection {
	return setMainControlChannel(pool, &Connection{}, state)
}

func borrowSuiteControlChannelIfAlive(pool mainConnAccessor, suite SuiteSession, state *engine.ExecutionState) bool {
	sc := suite.Conn()
	if sc == nil || !sc.isConnected() {
		return false
	}
	setMainControlChannel(pool, sc, state)
	return true
}

func restoreMainControlChannel(pool mainConnAccessor, prevMain *Connection, state *engine.ExecutionState) {
	if prevMain == nil {
		detachMainControlChannel(pool, state)
		return
	}
	setMainControlChannel(pool, prevMain, state)
}
