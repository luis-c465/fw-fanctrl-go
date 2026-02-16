package socket

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const maxCommandSize = 4096

type Server struct {
	socketPath     string
	listener       net.Listener
	commandHandler func(rawCommand string) (string, error)
	mu             sync.Mutex
}

func NewServer(socketPath string, handler func(string) (string, error)) *Server {
	return &Server{
		socketPath:     socketPath,
		commandHandler: handler,
	}
}

func (s *Server) Start() error {
	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0o755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	if err := os.Remove(s.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove stale socket file: %w", err)
	}

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket: %w", err)
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	if err := os.Chmod(s.socketPath, 0o777); err != nil {
		_ = listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("failed to accept connection: %w", err)
		}

		s.handleConnection(conn)
	}
}

func (s *Server) Stop() error {
	s.mu.Lock()
	listener := s.listener
	s.listener = nil
	s.mu.Unlock()

	var stopErr error
	if listener != nil {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			stopErr = fmt.Errorf("failed to close socket listener: %w", err)
		}
	}

	if err := os.Remove(s.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) && stopErr == nil {
		stopErr = fmt.Errorf("failed to remove socket file: %w", err)
	}

	return stopErr
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	buffer := make([]byte, maxCommandSize)
	n, err := conn.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		s.writeResponse(conn, fmt.Sprintf("[Error] > Failed to read command: %v", err))
		return
	}

	rawCommand := strings.TrimSpace(string(buffer[:n]))
	if rawCommand == "" {
		s.writeResponse(conn, "[Error] > Empty command")
		return
	}

	response, handlerErr := s.commandHandler(rawCommand)
	if handlerErr != nil {
		response = fmt.Sprintf("[Error] > %v", handlerErr)
	}

	s.writeResponse(conn, response)
}

func (s *Server) writeResponse(conn net.Conn, response string) {
	_, _ = conn.Write([]byte(response))

	if unixConn, ok := conn.(*net.UnixConn); ok {
		_ = unixConn.CloseWrite()
	}
}
