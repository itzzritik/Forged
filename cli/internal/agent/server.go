package agent

import (
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/itzzritik/forged/cli/internal/platform"
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
	ln, err := platform.Listen(s.socketPath)
	if err != nil {
		return err
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
	var retryDelay time.Duration
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			//lint:ignore SA1019 Listener implementations use Temporary to classify retriable accept failures.
			if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
				if retryDelay == 0 {
					retryDelay = 5 * time.Millisecond
				} else {
					retryDelay = min(2*retryDelay, time.Second)
				}
				s.logger.Warn("temporary SSH agent accept failure", "error", err, "retry_in", retryDelay)
				time.Sleep(retryDelay)
				continue
			}
			s.logger.Error("SSH agent accept loop stopped", "error", err)
			return
		}
		retryDelay = 0
		s.wg.Add(1)
		go func(conn net.Conn) {
			defer s.wg.Done()
			defer conn.Close()

			var scoped agent.ExtendedAgent = s.agent
			if pid, err := platform.AgentPeerPID(conn); err == nil {
				scoped = s.agent.ForClientPID(pid)
			}

			if err := agent.ServeAgent(scoped, conn); err != nil {
				s.logger.Debug("agent connection closed", "error", err)
			}
		}(conn)
	}
}
