package ml

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

type Server struct {
	cmd        *exec.Cmd
	port       int
	configPath string
}

func NewServer(configPath string) *Server {
	return &Server{configPath: configPath}
}

func (s *Server) Start() error {
	port, err := getFreePort()
	if err != nil {
		return fmt.Errorf("could not find a free port: %v", err)
	}
	s.port = port

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	classifierPath := filepath.Join(filepath.Dir(exePath), "fiona-classifier")

	if s.configPath != "" {
		s.cmd = exec.Command(classifierPath, "--config", s.configPath, "--port", strconv.Itoa(port))
	} else {
		s.cmd = exec.Command(classifierPath, "--port", strconv.Itoa(port))
	}
	err = s.cmd.Start()
	if err != nil {
		return err
	}

	client := NewClient(fmt.Sprintf("http://localhost:%d", s.port))
	timeout := time.After(60 * time.Second)
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-timeout:
			return errors.New("ml server did not start in time")
		case <-tick.C:
			if err := client.Health(); err == nil {
				return nil
			}
		}
	}
}

func (s *Server) Stop() error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}
	return s.cmd.Process.Kill()
}

func getFreePort() (int, error) {
	addr, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	defer addr.Close()
	return addr.Addr().(*net.TCPAddr).Port, nil
}
