package dummy

import (
	"encoding/json"

	"github.com/rancher/remotedialer"
	"github.com/rancher/remotedialer/api/proxy"
)

type proxyStream interface {
	Send(*proxy.ProxyMessage) error
	Recv() (*proxy.ProxyMessage, error)
}

type GrpcStreamTunnel struct {
	stream proxyStream
}

func NewGrpcStreamTunnel(stream proxyStream) *GrpcStreamTunnel {
	return &GrpcStreamTunnel{
		stream: stream,
	}
}

func (t *GrpcStreamTunnel) ReadMessage() (*remotedialer.Message, error) {
	pbMsg, err := t.stream.Recv()
	if err != nil {
		return nil, err
	}

	var msg remotedialer.Message
	err = json.Unmarshal(pbMsg.Data, &msg)
	if err != nil {
		return nil, err
	}

	return &msg, nil
}

func (t *GrpcStreamTunnel) WriteMessage(msg *remotedialer.Message) error {
	buf, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	err = t.stream.Send(&proxy.ProxyMessage{
		Data: buf,
	})
	if err != nil {
		return err
	}

	return nil
}
