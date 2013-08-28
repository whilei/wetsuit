package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
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

	Quitting       chan bool
	StopConnecting chan bool
}

type MopidyStatus int

const (
	MopidyConnecting MopidyStatus = iota
	MopidyConnected
	MopidyFailed
)

// InitMopidy() takes the path to the mopidy executable and a loaded configuration and verifies
// that everything is good to go.
func (app *Application) InitMopidy(exec string) (err error) {
	app.Mopidy = new(MopidyProc)
	app.Mopidy.Exec = exec
	app.Mopidy.Errors = make(chan error)
	app.Mopidy.Quitting = make(chan bool, 1)
	app.Mopidy.StopConnecting = make(chan bool, 1)

	// look up the hostname
	app.Mopidy.Hostname, err = app.Config.Get("mpd/hostname")
	if err != nil {
		return
	} else {
		// enclose any IP addresses with a colon in brackets
		// this is important because IPv6 is supported, and mopidy's default is "::"
		if strings.Index(app.Mopidy.Hostname, ":") != -1 {
			app.Mopidy.Hostname = "[" + app.Mopidy.Hostname + "]"
		}
	}

	// look up the port
	app.Mopidy.Port, err = app.Config.Get("mpd/port")
	if err != nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Panicking!")
			fmt.Println(r)
			app.Quit()
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
				//fmt.Fprintln(os.Stderr, err.Error())
				app.Mopidy.StopConnecting <- true
				app.Mopidy.Stop()
				app.Disable()
				app.Errors <- err
			}
		}
	}()

	return nil
}

// Disable() updates the application's state and gui when Mopidy isn't running.
func (app *Application) Disable() {
	app.Gui.Menu.Start.SetSensitive(true)
	app.Gui.Menu.Stop.SetSensitive(false)
	app.Gui.Menu.Restart.SetSensitive(false)
	app.Gui.DisableAllTabs()
	app.SetStatus(MopidyFailed)
}

// Enable() updates the application's state and gui when Mopidy is running
// and connected to.
func (app *Application) Enable() {
	app.SetStatus(MopidyConnected)
	app.Gui.Menu.Stop.SetSensitive(true)
	app.Gui.Menu.Restart.SetSensitive(true)
}

func (app *Application) StartMopidy() {
	app.SetStatus(MopidyConnecting)

	app.Gui.Menu.Start.SetSensitive(false)
	app.Gui.Menu.Stop.SetSensitive(false)
	app.Gui.Menu.Restart.SetSensitive(false)

	err := app.Mopidy.Start(app.Config.Path())
	if err == nil {
		app.Enable()
	} else {
		app.Mopidy.Errors <- err
		app.Disable()
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

