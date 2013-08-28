package mopidy

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/dradtke/wetsuit/server"
	"io"
	"net"
	"os/exec"
	"strings"
	"time"
)

type impl struct {
	cmd    *exec.Cmd
	errors chan error
	conn   net.Conn
}

func New() server.Instance {
	i := new(impl)
	i.errors = make(chan error)
	return i
}

func (i *impl) Name() string {
	return "mopidy"
}

func (i *impl) Start(configPath string) error {
	i.cmd = exec.Command("mopidy", "--config="+configPath)
	stderr, err := i.cmd.StderrPipe()
	if err != nil {
		return err
	}
	go i.watch(stderr)
	if err := i.cmd.Start(); err != nil {
		return err
	}
	return nil
}

func (i *impl) watch(stream io.Reader) {
	var (
		reader = bufio.NewReader(stream)
		buffer bytes.Buffer
	)
	for {
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			break
		}
		buffer.Write(line)
		if isPrefix {
			continue
		}
		str := buffer.String()
		if strings.HasPrefix(str, "ERROR") {
			i.errors <- errors.New(strings.TrimSpace(str[6:]))
		}
		buffer.Reset()
	}
}

func (i *impl) Connect(hostname, port string, stop chan bool) (bool, error) {
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
				i.conn = conn
				return true, nil
			}
			failedAttempts++
		}
	}
	return false, fmt.Errorf("failed to connect to mopidy after %d tries", maxAttempts)
}

func (i *impl) Errors() <-chan error {
	return i.errors
}
