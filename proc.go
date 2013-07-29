package main

import (
	"github.com/dradtke/gotk3/gtk"

	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

type MopidyProc struct {
	Cmd      *exec.Cmd
	Conn     net.Conn
	Output   *bytes.Buffer
	Errs     <-chan error
	Status   MopidyStatus
	Hostname string
	Port     string
}

type MopidyStatus int

const (
	MopidyConnecting MopidyStatus = iota
	MopidyConnected
	MopidyFailed
)

// InitMopidy takes the path to the mopidy executable and a loaded configuration and verifies
// that everything is good to go.
func InitMopidy(app *Application, cmd string) error {
	var ok bool
	errCh := make(chan error)
	app.Mopidy = &MopidyProc{Status: MopidyConnecting, Errs: errCh, Cmd: exec.Command(cmd, "--config="+app.Config.path)}

	// look up the hostname
	app.Mopidy.Hostname, ok = app.Config.Get("mpd/hostname")
	if !ok {
		return errors.New("mpd/hostname not found in config")
	} else {
		// enclose any IP addresses with a colon in brackets
		// this is important because IPv6 is supported, and mopidy's default is "::"
		if strings.Index(app.Mopidy.Hostname, ":") != -1 {
			app.Mopidy.Hostname = "[" + app.Mopidy.Hostname + "]"
		}
	}

	// look up the port
	app.Mopidy.Port, ok = app.Config.Get("mpd/port")
	if !ok {
		return errors.New("mpd/port not found in config")
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Panicking!")
			fmt.Println(r)
			Quit(app)
		}
	}()

	outCh := make(chan string)
	app.Mopidy.Output = new(bytes.Buffer)
	app.NewOutput = func(str string) {} // callback for when new data is received

	// watch mopidy's output for errors
	go func() {
		for {
			select {
			case str, ok := <-outCh:
				if !ok {
					return
				}
				app.OutputLock.Lock()
				app.Mopidy.Output.WriteString(str + "\n")
				app.NewOutput(str + "\n")
				app.OutputLock.Unlock()
				if strings.HasPrefix(str, "ERROR") {
					errCh <- errors.New(strings.TrimSpace(str[6:]))
				}
			}
		}
	}()

	// watch for errors
	go func() {
		for {
			select {
			case err, ok := <-app.Mopidy.Errs:
				if !ok {
					return
				}
				fmt.Fprintln(os.Stderr, err.Error())
				app.Mopidy.Cmd.Process.Kill()
				app.Mopidy.Status = MopidyFailed
				app.Gui.SetStatus("", "Not connected.")
				app.Errors <- err
				app.Mopidy.Cmd.Wait()
			}
		}
	}()

	stderr, err := app.Mopidy.Cmd.StderrPipe()
	if err != nil {
		return err
	}
	go ReadOutput(app, stderr, outCh)

	return nil
}

func (app *Application) StartMopidy() error {
	// kill mopidy if it's already running
	if app.Mopidy.Cmd.Process != nil {
		app.Mopidy.Cmd.Process.Kill()
	}

	if err := app.Mopidy.Cmd.Start(); err != nil {
		return errors.New("failed to start mopidy: " + err.Error())
	}

	failedAttempts := 0
	maxAttempts := 10

	// spin until 1) we get an error, 2) we connect successfully, or
	// 3) we time out with too many attempts
	for failedAttempts < maxAttempts {
		time.Sleep(500 * time.Millisecond)
		conn, err := net.Dial("tcp", app.Mopidy.Hostname+":"+app.Mopidy.Port)
		if err == nil {
			// connected
			app.Mopidy.Conn = conn
			app.Mopidy.Status = MopidyConnected
			app.Gui.SetStatus(gtk.STOCK_CONNECT, "Connected to Mopidy.")
			return nil
		}
		// TODO: check to see if this error means the port is taken
		failedAttempts++
	}

	app.Mopidy.Status = MopidyFailed
	return fmt.Errorf("failed to connect to mopidy after %d tries", maxAttempts)
}

// ReadOutput() runs in a separate goroutine and continually reads lines from the stream
// until a certain condition is met.
func ReadOutput(app *Application, stream io.Reader, output chan string) {
	var (
		reader = bufio.NewReader(stream)
		buffer bytes.Buffer
	)
	for app.Mopidy.Status != MopidyFailed {
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			break
		}

		buffer.Write(line)
		if isPrefix {
			continue
		}

		output <- buffer.String()
		buffer.Reset()
	}
	close(output)
}

// EchoStream takes a stream and a prefix and continuously prints lines
// from the stream prefixed by prefix.
func EchoStream(stream io.Reader, prefix string) {
	var (
		reader = bufio.NewReader(stream)
		buffer bytes.Buffer
	)
	for {
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			return
		}

		buffer.Write(line)
		if isPrefix {
			continue
		}

		fmt.Println(prefix, buffer.String())
		buffer.Reset()
	}
}

// WatchForError reads lines from the reader continuously, only returning an error
// if reading a line fails. If a line is encountered that starts with "ERROR", it sends
// the rest of that line along the error channel.
func WatchForErrors(r io.Reader, ch chan string) error {
	var (
		reader = bufio.NewReader(r)
		buffer bytes.Buffer
	)
	for {
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			return err
		}

		buffer.Write(line)
		if isPrefix {
			continue
		}

		outputLine := buffer.String()
		buffer.Reset()
		if outputLine[:5] == "ERROR" {
			ch <- strings.TrimSpace(outputLine[6:])
		}
	}
}
