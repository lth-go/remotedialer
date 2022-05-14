package remotedialer

import (
	"context"
	"io"
	"net"
	"sync"
	"time"
)

// 客户端发起需要代理的请求
func clientDial(ctx context.Context, conn *connection, message *Message) {
	defer conn.Close()

	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Minute))

	d := net.Dialer{}
	netConn, err := d.DialContext(ctx, message.Proto, message.Address)
	cancel()

	if err != nil {
		conn.tunnelClose(err)
		return
	}
	defer netConn.Close()

	pipe(conn, netConn)
}

func pipe(client *connection, server net.Conn) {
	wg := sync.WaitGroup{}
	wg.Add(1)

	closeFunc := func(err error) error {
		if err == nil {
			err = io.EOF
		}
		client.doTunnelClose(err)
		server.Close()
		return err
	}

	go func() {
		defer wg.Done()

		_, err := io.Copy(server, client) // 发送请求
		closeFunc(err)
	}()

	_, err := io.Copy(client, server) // 接收相应
	err = closeFunc(err)
	wg.Wait()

	// Write tunnel error after no more I/O is happening, just incase messages get out of order
	client.writeErr(err)
}
