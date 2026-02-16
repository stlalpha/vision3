package session

import (
	"sort"
	"sync"
)

// SessionRegistry tracks all active BBS sessions.
type SessionRegistry struct {
	mu       sync.RWMutex
	sessions map[int]*BbsSession
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		sessions: make(map[int]*BbsSession),
	}
}

func (r *SessionRegistry) Register(s *BbsSession) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[s.NodeID] = s
}

func (r *SessionRegistry) Unregister(nodeID int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, nodeID)
}

func (r *SessionRegistry) Get(nodeID int) *BbsSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessions[nodeID]
}

func (r *SessionRegistry) ListActive() []*BbsSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*BbsSession, 0, len(r.sessions))
	for _, s := range r.sessions {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].NodeID < result[j].NodeID
	})
	return result
}
