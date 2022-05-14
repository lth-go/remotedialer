package remotedialer

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

// ClientConnect connect to WS and wait 5 seconds when error
func ClientConnect(ctx context.Context, conn Tunnel) error {
	err := ConnectToProxy(ctx, conn)
	if err != nil {
		logrus.WithError(err).Error("Remotedialer proxy error")
		time.Sleep(time.Duration(5) * time.Second)
		return err
	}
	return nil
}

// ConnectToProxy connect to websocket server
func ConnectToProxy(rootCtx context.Context, conn Tunnel) error {
	logrus.Info("Connecting to proxy")

	result := make(chan error, 2)

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	session := NewClientSession(conn)
	defer session.Close()

	go func() {
		err := session.Serve(ctx)
		result <- err
	}()

	select {
	case <-ctx.Done():
		logrus.WithField("err", ctx.Err()).Info("Proxy done")
		return nil
	case err := <-result:
		return err
	}
}
