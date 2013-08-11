package main

import (
	"bytes"
	"net"
	"os/exec"
)

type MopidyProc struct {
	Cmd    *exec.Cmd
	Conn   net.Conn
	Output *bytes.Buffer
	Errs   <-chan error
	Status MopidyStatus
}

type MopidyStatus int

const (
	MopidyConnecting MopidyStatus = iota
	MopidyConnected
	MopidyFailed
)
