package remotedialer

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync/atomic"
	"time"
)

const (
	Data MessageType = iota + 1
	Connect
	Error
)

var (
	idCounter      int64
	legacyDeadline = (15 * time.Second).Milliseconds()
)

func init() {
	r := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	idCounter = r.Int63()
}

type MessageType int64

type Message struct {
	ID          int64
	err         error
	ConnID      int64
	MessageType MessageType
	Bytes       []byte
	Proto       string
	Address     string
}

func nextid() int64 {
	return atomic.AddInt64(&idCounter, 1)
}

func newMessage(connID int64, bytes []byte) *Message {
	return &Message{
		ID:          nextid(),
		ConnID:      connID,
		MessageType: Data,
		Bytes:       bytes,
	}
}

func newConnect(connID int64, proto, address string) *Message {
	return &Message{
		ID:          nextid(),
		ConnID:      connID,
		MessageType: Connect,
		Bytes:       []byte(fmt.Sprintf("%s/%s", proto, address)),
		Proto:       proto,
		Address:     address,
	}
}

func newErrorMessage(connID int64, err error) *Message {
	return &Message{
		ID:          nextid(),
		err:         err,
		ConnID:      connID,
		MessageType: Error,
		Bytes:       []byte(err.Error()),
	}
}

func (m *Message) Err() error {
	if m.err != nil {
		return m.err
	}

	str := string(m.Bytes)
	if str == "EOF" {
		m.err = io.EOF
	} else {
		m.err = errors.New(str)
	}
	return m.err
}

func (m *Message) WriteTo(conn Tunnel) (int, error) {
	err := conn.WriteMessage(m)
	return len(m.Bytes), err
}

func (m *Message) String() string {
	switch m.MessageType {
	case Data:
		return fmt.Sprintf("%d DATA         [%d]: %d bytes: %s", m.ID, m.ConnID, len(m.Bytes), string(m.Bytes))
	case Error:
		return fmt.Sprintf("%d ERROR        [%d]: %s", m.ID, m.ConnID, m.Err())
	case Connect:
		return fmt.Sprintf("%d CONNECT      [%d]: %s/%s", m.ID, m.ConnID, m.Proto, m.Address)
	}
	return fmt.Sprintf("%d UNKNOWN[%d]: %d", m.ID, m.ConnID, m.MessageType)
}
