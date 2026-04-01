package agent

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"

	"golang.org/x/crypto/ssh/agent"
)

type Server struct {
	socketPath string
	agent      *ForgedAgent
	listener   net.Listener
	logger     *slog.Logger
	wg         sync.WaitGroup
}

func NewServer(socketPath string, a *ForgedAgent, logger *slog.Logger) *Server {
	return &Server{
		socketPath: socketPath,
		agent:      a,
		logger:     logger,
	}
}

func (s *Server) Start() error {
	os.Remove(s.socketPath)

	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.socketPath, err)
	}

	if err := os.Chmod(s.socketPath, 0600); err != nil {
		ln.Close()
		return fmt.Errorf("setting socket permissions: %w", err)
	}

	s.listener = ln

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.acceptLoop()
	}()

	return nil
}

func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer conn.Close()
			if err := agent.ServeAgent(s.agent, conn); err != nil {
				s.logger.Debug("agent connection closed", "error", err)
			}
		}()
	}
}
