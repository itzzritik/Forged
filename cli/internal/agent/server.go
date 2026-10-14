package agent

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/itzzritik/forged/cli/internal/platform"
	"golang.org/x/crypto/ssh/agent"
)

type Server struct {
	socketPath      string
	agent           *ForgedAgent
	lifecycleMu     sync.Mutex
	listener        net.Listener
	connections     map[net.Conn]struct{}
	stopping        bool
	stopCtx         context.Context
	stopCancel      context.CancelFunc
	logger          *slog.Logger
	wg              sync.WaitGroup
	onAcceptFailure func(error)
}

func NewServer(socketPath string, a *ForgedAgent, logger *slog.Logger) *Server {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &Server{
		socketPath:  socketPath,
		agent:       a,
		logger:      logger,
		connections: make(map[net.Conn]struct{}),
		stopCtx:     stopCtx,
		stopCancel:  stopCancel,
	}
}

func (s *Server) Start() error {
	s.lifecycleMu.Lock()
	if s.stopping {
		s.lifecycleMu.Unlock()
		return errors.New("starting SSH agent server: already stopping")
	}
	if s.listener != nil {
		s.lifecycleMu.Unlock()
		return errors.New("starting SSH agent server: already started")
	}
	ln, err := platform.Listen(s.socketPath)
	if err != nil {
		s.lifecycleMu.Unlock()
		return err
	}
	s.listener = ln
	s.wg.Add(1)
	s.lifecycleMu.Unlock()

	go func() {
		defer s.wg.Done()
		s.acceptLoop(ln)
	}()

	return nil
}

func (s *Server) BeginStop() {
	s.lifecycleMu.Lock()
	if s.stopping {
		s.lifecycleMu.Unlock()
		return
	}
	s.stopping = true
	listener := s.listener
	connections := make([]net.Conn, 0, len(s.connections))
	for conn := range s.connections {
		connections = append(connections, conn)
	}
	s.lifecycleMu.Unlock()

	s.stopCancel()
	if listener != nil {
		_ = listener.Close()
	}
	for _, conn := range connections {
		_ = conn.Close()
	}
}

func (s *Server) Wait() {
	s.wg.Wait()
}

func (s *Server) Stop() {
	s.BeginStop()
	s.Wait()
}

const maxAcceptFailures = 60

// SetAcceptFailureHandler is called once when accepting fails for good.
func (s *Server) SetAcceptFailureHandler(fn func(error)) {
	s.onAcceptFailure = fn
}

func (s *Server) acceptLoop(listener net.Listener) {
	var retryDelay time.Duration
	failures := 0
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			failures++
			if failures > maxAcceptFailures {
				s.logger.Error("SSH agent accept loop stopped", "error", err)
				if s.onAcceptFailure != nil {
					s.onAcceptFailure(err)
				}
				return
			}
			if retryDelay == 0 {
				retryDelay = 5 * time.Millisecond
			} else {
				retryDelay = min(2*retryDelay, time.Second)
			}
			s.logger.Warn("SSH agent accept failure", "error", err, "retry_in", retryDelay)
			time.Sleep(retryDelay)
			continue
		}
		retryDelay, failures = 0, 0
		if platform.ControlPeerCredentialsAvailable() {
			if err := platform.VerifyCurrentUserPeer(conn); err != nil {
				// Readiness probes connect and close before admission.
				if !errors.Is(err, syscall.ENOTCONN) {
					s.logger.Warn("rejecting SSH agent peer", "error", err)
				}
				_ = conn.Close()
				continue
			}
		}
		ctx, cancel, ok := s.admit(conn)
		if !ok {
			_ = conn.Close()
			return
		}
		go func(conn net.Conn, ctx context.Context, cancel context.CancelFunc) {
			defer s.wg.Done()
			defer s.release(conn)
			defer cancel()
			defer conn.Close()
			defer s.recoverConn()

			var scoped agent.ExtendedAgent
			release := func() {}
			defer func() { release() }()
			if !platform.SSHRoutingSupported() {
				scoped = s.agent.ForContext(ctx)
			} else if pid, err := platform.AgentPeerPID(conn); err != nil {
				s.logger.Warn("rejecting unverified SSH route client", "error", err)
				scoped = s.agent.ForDeniedClientContext(ctx, 0)
			} else if process, err := platform.ProcessInfoForPID(pid); err != nil {
				s.logger.Warn("rejecting unknown SSH route client", "pid", pid, "error", err)
				scoped = s.agent.ForDeniedClientContext(ctx, pid)
			} else {
				scoped, release = s.agent.ForClientProcessSessionContext(ctx, process.Instance)
			}

			err := agent.ServeAgent(scoped, writeDeadlineConn{conn})
			s.logger.Debug("agent connection closed", "error", err)
		}(conn, ctx, cancel)
	}
}

// A panic in one client connection must not take down the agent for every other SSH session.
func (s *Server) recoverConn() {
	if r := recover(); r != nil {
		s.logger.Error("ssh agent connection panicked", "panic", r, "stack", string(debug.Stack()))
	}
}

func (s *Server) admit(conn net.Conn) (context.Context, context.CancelFunc, bool) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.stopping {
		return nil, nil, false
	}
	ctx, cancel := context.WithCancel(s.stopCtx)
	s.connections[conn] = struct{}{}
	s.wg.Add(1)
	return ctx, cancel, true
}

func (s *Server) release(conn net.Conn) {
	s.lifecycleMu.Lock()
	delete(s.connections, conn)
	s.lifecycleMu.Unlock()
}

const agentWriteTimeout = 30 * time.Second

// writeDeadlineConn bounds each reply: a client that stops reading would otherwise pin its
// connection and route lease until shutdown. Reads stay unbounded; idle agent connections are normal.
type writeDeadlineConn struct{ net.Conn }

func (c writeDeadlineConn) Write(p []byte) (int, error) {
	_ = c.Conn.SetWriteDeadline(time.Now().Add(agentWriteTimeout))
	return c.Conn.Write(p)
}
