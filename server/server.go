package server

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"time"
)

type Instance interface {
	Name() string                                                // read-only string for the server's name, e.g. "mopidy"
	Connect(hostname, port string, stop chan bool) (bool, error) // connect to it
	Errors() <-chan error                                        // read-only channel of errors that the server reports
}

type Status int

const (
	Connecting Status = iota
	Connected
	Failed
)

type baseServer struct {
	cmd    *exec.Cmd
	errors chan error
	conn   net.Conn
}

func (srv *baseServer) Errors() <-chan error {
	return srv.errors
}

func New(name string) (Instance, error) {
	base := new(baseServer)
	base.errors = make(chan error)
	switch name {
	case "mopidy":
		return &mopidyServer{*base}, nil
	default:
		return nil, fmt.Errorf("unrecognized server name '%s'", name)
	}
}

/* -- Mopidy -- */

type mopidyServer struct {
	baseServer
}

func (srv *mopidyServer) Name() string {
	return "mopidy"
}

func (srv *mopidyServer) Connect(hostname, port string, stop chan bool) (bool, error) {
	println("connecting...")
	if hostname == "" {
		hostname = "127.0.0.1"
	}
	if port == "" {
		port = "6600"
	}
	var (
		failedAttempts = 0
		maxAttempts    = 10
	)

	for failedAttempts < maxAttempts {
		select {
		case <-stop:
			return false, nil
		case <-time.After(500 * time.Millisecond):
			conn, err := net.Dial("tcp", hostname+":"+port)
			if err == nil {
				srv.conn = conn
				// read the first line before returning
				scanner := bufio.NewScanner(conn)
				scanner.Split(bufio.ScanLines)
				if ok := scanner.Scan(); !ok {
					return false, scanner.Err()
				}
				fmt.Println(scanner.Text())
				return true, nil
			}
			failedAttempts++
		}
	}
	return false, fmt.Errorf("failed to connect to mopidy after %d tries", maxAttempts)
}
