package remotedialer

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
)

type sessionManager struct {
	sync.Mutex
	clients map[string][]*Session
}

func newSessionManager() *sessionManager {
	return &sessionManager{
		clients: map[string][]*Session{},
	}
}

func toDialer(s *Session, prefix string) Dialer {
	return func(ctx context.Context, proto, address string) (net.Conn, error) {
		if prefix == "" {
			return s.serverConnectContext(ctx, proto, address)
		}
		return s.serverConnectContext(ctx, prefix+"::"+proto, address)
	}
}

func (sm *sessionManager) getDialer(clientKey string) (Dialer, error) {
	sm.Lock()
	defer sm.Unlock()

	sessions := sm.clients[clientKey]
	if len(sessions) > 0 {
		return toDialer(sessions[0], ""), nil
	}

	return nil, fmt.Errorf("failed to find Session for client %s", clientKey)
}

func (sm *sessionManager) add(clientKey string, conn Tunnel) *Session {
	sessionKey := rand.Int63()
	session := newSession(sessionKey, clientKey, conn)

	sm.Lock()
	defer sm.Unlock()

	sm.clients[clientKey] = append(sm.clients[clientKey], session)

	return session
}

func (sm *sessionManager) remove(s *Session) {
	sm.Lock()
	defer sm.Unlock()

	store := sm.clients

	var newSessions []*Session

	for _, v := range store[s.clientKey] {
		if v.sessionKey == s.sessionKey {
			continue
		}
		newSessions = append(newSessions, v)
	}

	if len(newSessions) == 0 {
		delete(store, s.clientKey)
	} else {
		store[s.clientKey] = newSessions
	}

	s.Close()
}
