package remotedialer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Session struct {
	sync.Mutex
	nextConnID int64
	clientKey  string
	sessionKey int64
	conn       Tunnel
	conns      map[int64]*connection
}

func NewClientSession(conn Tunnel) *Session {
	return &Session{
		clientKey: "client",
		conn:      conn,
		conns:     map[int64]*connection{},
	}
}

func newSession(sessionKey int64, clientKey string, conn Tunnel) *Session {
	return &Session{
		nextConnID: 1,
		clientKey:  clientKey,
		sessionKey: sessionKey,
		conn:       conn,
		conns:      map[int64]*connection{},
	}
}

func (s *Session) Serve(ctx context.Context) error {
	for {
		msg, err := s.conn.ReadMessage()
		if err != nil {
			return err
		}

		err = s.serveMessage(ctx, msg)
		if err != nil {
			return err
		}
	}
}

func (s *Session) serveMessage(ctx context.Context, message *Message) error {
	if message.MessageType == Connect {
		s.clientConnect(ctx, message)
		return nil
	}

	s.Lock()
	conn := s.conns[message.ConnID]
	s.Unlock()

	if conn == nil {
		if message.MessageType == Data {
			err := fmt.Errorf("connection not found %s/%d/%d", s.clientKey, s.sessionKey, message.ConnID)
			newErrorMessage(message.ConnID, err).WriteTo(s.conn)
		}
		return nil
	}

	switch message.MessageType {
	case Data:
		if err := conn.OnData(message); err != nil {
			s.closeConnection(message.ConnID, err)
		}
	case Error:
		s.closeConnection(message.ConnID, message.Err())
	}

	return nil
}

func defaultDeadline() time.Time {
	return time.Now().Add(time.Minute)
}

func (s *Session) closeConnection(connID int64, err error) {
	s.Lock()
	conn := s.conns[connID]
	delete(s.conns, connID)
	s.Unlock()

	if conn != nil {
		conn.tunnelClose(err)
	}
}

func (s *Session) clientConnect(ctx context.Context, message *Message) {
	conn := newConnection(message.ConnID, s, message.Proto, message.Address)

	s.Lock()
	s.conns[message.ConnID] = conn
	s.Unlock()

	go clientDial(ctx, conn, message)
}

type connResult struct {
	conn net.Conn
	err  error
}

func (s *Session) serverConnectContext(ctx context.Context, proto, address string) (net.Conn, error) {
	result := make(chan connResult, 1)
	go func() {
		c, err := s.serverConnect(proto, address)
		result <- connResult{conn: c, err: err}
	}()

	select {
	case <-ctx.Done():
		// We don't want to orphan an open connection so we wait for the result and immediately close it
		go func() {
			r := <-result
			if r.err == nil {
				r.conn.Close()
			}
		}()
		return nil, ctx.Err()
	case r := <-result:
		return r.conn, r.err
	}
}

func (s *Session) serverConnect(proto, address string) (net.Conn, error) {
	connID := atomic.AddInt64(&s.nextConnID, 1)
	conn := newConnection(connID, s, proto, address)

	s.Lock()
	s.conns[connID] = conn
	s.Unlock()

	_, err := s.writeMessage(newConnect(connID, proto, address))
	if err != nil {
		s.closeConnection(connID, err)
		return nil, err
	}

	return conn, err
}

func (s *Session) writeMessage(message *Message) (int, error) {
	return message.WriteTo(s.conn)
}

func (s *Session) Close() {
	s.Lock()
	defer s.Unlock()

	for _, connection := range s.conns {
		connection.tunnelClose(errors.New("tunnel disconnect"))
	}

	s.conns = map[int64]*connection{}
}
