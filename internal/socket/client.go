package socket

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
)

const errorResponsePrefix = "[Error] > "

func SendCommand(socketPath string, command string) (result string, retErr error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		if isDaemonUnavailable(err) {
			return "", fmt.Errorf("fw-fanctrld is not running. Start the service with: systemctl start fw-fanctrld")
		}

		return "", fmt.Errorf("failed to connect to daemon socket %q: %w", socketPath, err)
	}
	defer func() {
		if err := conn.Close(); err != nil && retErr == nil {
			retErr = fmt.Errorf("failed to close connection: %w", err)
		}
	}()

	if _, err := conn.Write([]byte(command)); err != nil {
		return "", fmt.Errorf("failed to send command to daemon: %w", err)
	}

	response, err := readFullResponse(conn)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(response, errorResponsePrefix) {
		return "", errors.New(response)
	}

	return response, nil
}

func readFullResponse(conn net.Conn) (string, error) {
	buffer := make([]byte, 1024)
	var response strings.Builder

	for {
		n, err := conn.Read(buffer)
		if n > 0 {
			response.Write(buffer[:n])
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return "", fmt.Errorf("failed to read daemon response: %w", err)
		}
	}

	if response.Len() == 0 {
		return "", fmt.Errorf("received empty response from daemon")
	}

	return response.String(), nil
}

func isDaemonUnavailable(err error) bool {
	return errors.Is(err, os.ErrNotExist) ||
		errors.Is(err, syscall.ENOENT) ||
		errors.Is(err, syscall.ECONNREFUSED)
}
