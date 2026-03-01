package ml

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Server struct {
	cmd *exec.Cmd
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Start() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	classifierPath := filepath.Join(filepath.Dir(exePath), "fiona-classifier")

	s.cmd = exec.Command(classifierPath)
	err = s.cmd.Start()
	if err != nil {
		return err
	}

	client := NewClient("")
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
