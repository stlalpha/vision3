package session

import (
	"testing"
	"time"
)

func TestRegistryRegisterAndList(t *testing.T) {
	r := NewSessionRegistry()

	s1 := &BbsSession{NodeID: 1, StartTime: time.Now(), CurrentMenu: "MAIN"}
	s2 := &BbsSession{NodeID: 3, StartTime: time.Now(), CurrentMenu: "LOGIN"}

	r.Register(s1)
	r.Register(s2)

	active := r.ListActive()
	if len(active) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(active))
	}
	if active[0].NodeID != 1 || active[1].NodeID != 3 {
		t.Errorf("expected sorted by NodeID [1,3], got [%d,%d]", active[0].NodeID, active[1].NodeID)
	}
}

func TestRegistryUnregister(t *testing.T) {
	r := NewSessionRegistry()

	s1 := &BbsSession{NodeID: 1, StartTime: time.Now()}
	r.Register(s1)
	r.Unregister(1)

	active := r.ListActive()
	if len(active) != 0 {
		t.Fatalf("expected 0 sessions after unregister, got %d", len(active))
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewSessionRegistry()

	s1 := &BbsSession{NodeID: 2, StartTime: time.Now()}
	r.Register(s1)

	got := r.Get(2)
	if got == nil || got.NodeID != 2 {
		t.Errorf("expected session with NodeID 2, got %v", got)
	}

	if r.Get(99) != nil {
		t.Error("expected nil for nonexistent node")
	}
}
