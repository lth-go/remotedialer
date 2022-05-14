package remotedialer

import (
	"io"
	"net"
	"time"
)

type connection struct {
	err           error
	writeDeadline time.Time
	buffer        *readBuffer
	addr          addr
	session       *Session
	connID        int64
}

func newConnection(connID int64, session *Session, proto, address string) *connection {
	c := &connection{
		addr: addr{
			proto:   proto,
			address: address,
		},
		connID:  connID,
		session: session,
	}
	c.buffer = newReadBuffer(connID)
	return c
}

func (c *connection) tunnelClose(err error) {
	c.writeErr(err)
	c.doTunnelClose(err)
}

func (c *connection) doTunnelClose(err error) {
	if c.err != nil {
		return
	}

	c.err = err
	if c.err == nil {
		c.err = io.ErrClosedPipe
	}

	c.buffer.Close(c.err)
}

func (c *connection) OnData(m *Message) error {
	return c.buffer.Write(m.Bytes)
}

func (c *connection) Close() error {
	c.session.closeConnection(c.connID, io.EOF)
	return nil
}

func (c *connection) Read(b []byte) (int, error) {
	return c.buffer.Read(b)
}

func (c *connection) Write(b []byte) (int, error) {
	if c.err != nil {
		return 0, io.ErrClosedPipe
	}
	msg := newMessage(c.connID, b)
	return c.session.writeMessage(msg)
}

func (c *connection) writeErr(err error) {
	if err != nil {
		msg := newErrorMessage(c.connID, err)
		c.session.writeMessage(msg)
	}
}

func (c *connection) LocalAddr() net.Addr {
	return c.addr
}

func (c *connection) RemoteAddr() net.Addr {
	return c.addr
}

func (c *connection) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

func (c *connection) SetReadDeadline(t time.Time) error {
	c.buffer.deadline = t
	return nil
}

func (c *connection) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return nil
}

type addr struct {
	proto   string
	address string
}

func (a addr) Network() string {
	return a.proto
}

func (a addr) String() string {
	return a.address
}
