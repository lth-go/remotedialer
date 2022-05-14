package remotedialer


type Server struct {
	sessionManager *sessionManager
}

func New() *Server {
	return &Server{
		sessionManager: newSessionManager(),
	}
}

func (s *Server) SessionAdd(clientKey string, conn Tunnel) *Session {
	return s.sessionManager.add(clientKey, conn)
}

func (s *Server) SessionRemove(session *Session) {
	s.sessionManager.remove(session)
}
