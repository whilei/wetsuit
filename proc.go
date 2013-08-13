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
	"sync"
	"time"
)

type MopidyProc struct {
	Exec     string        // command to start Mopidy
	Cmd      *exec.Cmd     // command for starting Mopidy
	Conn     net.Conn      // connection to the running Mopidy process
	Output   *bytes.Buffer // buffer of Mopidy's output
	Errors   chan error
	Status   MopidyStatus
	Hostname string
	Port     string

	// find a better place to put this?
	OutputLock sync.Mutex
	NewOutput  func(string)

	Quitting chan bool
}

type MopidyStatus int

const (
	MopidyConnecting MopidyStatus = iota
	MopidyConnected
	MopidyFailed
)

// InitMopidy takes the path to the mopidy executable and a loaded configuration and verifies
// that everything is good to go.
func InitMopidy(app *Application, exec string) error {
	var ok bool

	app.Mopidy = new(MopidyProc)
	app.Mopidy.Exec = exec
	app.Mopidy.Status = MopidyConnecting
	app.Mopidy.Errors = make(chan error)
	app.Mopidy.Quitting = make(chan bool, 1)

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

	app.Mopidy.Output = new(bytes.Buffer)
	app.Mopidy.NewOutput = func(str string) {} // callback for when new data is received

	// if any errors are encountered, stop mopidy and notify the application
	go func() {
		for {
			select {
			case err, ok := <-app.Mopidy.Errors:
				if !ok {
					return
				}
				fmt.Fprintln(os.Stderr, err.Error())
				app.Mopidy.Stop()
				app.Mopidy.Status = MopidyFailed
				app.Gui.SetStatus("", "Not connected.")
				app.Errors <- err
			}
		}
	}()

	return nil
}

func (app *Application) StartMopidy() {
	app.Gui.MenuStart.SetSensitive(false)
	app.Gui.MenuStop.SetSensitive(true)
	app.Gui.MenuRestart.SetSensitive(true)
	err := app.Mopidy.Start(app.Config.Path)
	if err == nil {
		app.Gui.SetStatus(gtk.STOCK_CONNECT, "Connected to Mopidy.")
	} else {
		app.Gui.SetStatus("", "Not connected.")
		app.Errors <- err
	}
}

// ReadOutput() constantly reads lines from Mopidy and writes them to the
// output buffer. Any errors found are also sent on the error channel.
func (m *MopidyProc) ReadOutput(stream io.Reader) {
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
		fmt.Println(str)
		m.OutputLock.Lock()
		m.Output.WriteString(str + "\n")
		m.NewOutput(str + "\n")
		m.OutputLock.Unlock()
		if strings.HasPrefix(str, "ERROR") {
			m.Errors <- errors.New(strings.TrimSpace(str[6:]))
		}
		buffer.Reset()
	}
	m.Quitting <- true
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

func (app *Application) StopMopidy() {
	app.Gui.MenuStart.SetSensitive(true)
	app.Gui.MenuStop.SetSensitive(false)
	app.Gui.MenuRestart.SetSensitive(false)
	app.Gui.SetStatus("", "Not connected.")
	err := app.Mopidy.Stop()
	if err != nil {
		app.Errors <- err
	}
}

func (app *Application) RestartMopidy() {
	app.Mopidy.Output.Reset()
	app.Gui.SetStatus("", "Connecting...")

	if err := app.Mopidy.Stop(); err != nil {
		app.Errors <- err
	}
	if err := app.Mopidy.Start(app.Config.Path); err != nil {
		app.Errors <- err
	}
}

// Mopidy methods

func (m *MopidyProc) Start(configPath string) error {
	m.Cmd = exec.Command(m.Exec, "--config="+configPath)
	stderr, err := m.Cmd.StderrPipe()
	if err != nil {
		return err
	}
	go m.ReadOutput(stderr)

	if err := m.Cmd.Start(); err != nil {
		return errors.New("failed to start mopidy: " + err.Error())
	}

	failedAttempts := 0
	maxAttempts := 10

	// spin until 1) we get an error, 2) we connect successfully, or
	// 3) we time out with too many attempts
	for failedAttempts < maxAttempts {
		time.Sleep(500 * time.Millisecond)
		conn, err := net.Dial("tcp", m.Hostname+":"+m.Port)
		if err == nil {
			// connected
			m.Conn = conn
			m.Status = MopidyConnected
			return nil
		}
		// TODO: check to see if this error means the port is taken
		failedAttempts++
	}

	m.Status = MopidyFailed
	return fmt.Errorf("failed to connect to mopidy after %d tries", maxAttempts)
}

func (m *MopidyProc) Stop() error {
	if err := m.Cmd.Process.Kill(); err != nil {
		return err
	}

	if _, err := m.Cmd.Process.Wait(); err != nil {
		return err
	}

	// wait for associated goroutines to finish
	<-m.Quitting
	m.Output.Reset()
	return nil
}
