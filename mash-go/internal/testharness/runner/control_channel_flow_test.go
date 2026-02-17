package runner

import (
	"context"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
)

func TestBorrowSuiteControlChannelIfAlive_BindsMainAndState(t *testing.T) {
	pool := NewConnPool(nil, nil)
	pool.SetMain(&Connection{})
	suite := NewSuiteSession()
	suiteConn := &Connection{state: ConnOperational}
	suite.SetConn(suiteConn)
	state := engine.NewExecutionState(context.Background())

	ok := borrowSuiteControlChannelIfAlive(pool, suite, state)
	if !ok {
		t.Fatal("expected suite control channel borrow to succeed")
	}
	if pool.Main() != suiteConn {
		t.Fatal("expected pool main to point at suite connection")
	}
	if got, _ := state.Get(StateConnection); got != suiteConn {
		t.Fatal("expected state connection to point at suite connection")
	}
}

func TestBorrowSuiteControlChannelIfAlive_DoesNothingWhenDead(t *testing.T) {
	pool := NewConnPool(nil, nil)
	orig := &Connection{state: ConnOperational}
	pool.SetMain(orig)
	suite := NewSuiteSession()
	suite.SetConn(&Connection{state: ConnDisconnected})
	state := engine.NewExecutionState(context.Background())
	state.Set(StateConnection, orig)

	ok := borrowSuiteControlChannelIfAlive(pool, suite, state)
	if ok {
		t.Fatal("expected suite control channel borrow to fail for dead suite conn")
	}
	if pool.Main() != orig {
		t.Fatal("expected pool main to remain unchanged")
	}
	if got, _ := state.Get(StateConnection); got != orig {
		t.Fatal("expected state connection to remain unchanged")
	}
}

func TestRestoreMainControlChannel_DetachesWhenPrevNil(t *testing.T) {
	pool := NewConnPool(nil, nil)
	pool.SetMain(&Connection{state: ConnOperational})
	state := engine.NewExecutionState(context.Background())

	restoreMainControlChannel(pool, nil, state)
	if pool.Main() == nil {
		t.Fatal("expected detached empty main connection")
	}
	if pool.Main().isConnected() {
		t.Fatal("expected detached main connection to be disconnected")
	}
	if got, _ := state.Get(StateConnection); got != pool.Main() {
		t.Fatal("expected state connection to track detached main connection")
	}
}
