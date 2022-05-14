package remotedialer

type Tunnel interface {
	ReadMessage() (*Message, error)
	WriteMessage(*Message) error
}
