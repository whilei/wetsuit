package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	//"github.com/dradtke/gotk3/glib"
	"github.com/dradtke/gotk3/gtk"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Application struct {
	Mopidy *MopidyProc
	Config *MopidyConfig
	Gui *GUI
	Errors chan error
	Quit chan bool

	// find a better place to put this?
	OutputLock sync.Mutex
	NewOutput func(string)
}

func Fatal(err error) {
	dialog := gtk.MessageDialogNew(nil, 0, gtk.MESSAGE_ERROR, gtk.BUTTONS_CLOSE, err.Error())
	dialog.Connect("response", func() {
		gtk.MainQuit()
		os.Exit(1)
	})
	dialog.Show()
	if gtk.MainLevel() == 0 {
		gtk.Main()
	}
}

func NonFatal(err error) {
	dialog := gtk.MessageDialogNew(nil, 0, gtk.MESSAGE_ERROR, gtk.BUTTONS_CLOSE, err.Error())
	dialog.Connect("response", func() {
		dialog.Destroy()
	})
	dialog.Show()
}

func main() {
	runtime.GOMAXPROCS(1)
	app := new(Application)
	app.Errors = make(chan error)
	app.Quit = make(chan bool, 1)

	gtk.Init(nil)
	var mopidyCmdPath, userConfigPath string

	// make sure mopidy is installed
	mopidyCmdPath, err := exec.LookPath("mopidy")
	if err != nil {
		Fatal(errors.New("Mopidy is not installed."))
	}

	// find the user's configuration
	usr, err := user.Current()
	if err == nil {
		userConfigPath = filepath.Join(usr.HomeDir, ".config", "wetsuit", "mopidy.conf")
	} else {
		// no user =/
		Fatal(err)
	}

	// load configuration
	app.Config, err = LoadConfig(mopidyCmdPath, userConfigPath)
	if err != nil {
		Fatal(err)
	}

	// create the window
	app.Gui, err = InitGUI(app.Config)
	if err != nil {
		Fatal(err)
	}

	err = InitMopidy(app, mopidyCmdPath)
	if err != nil {
		// TODO: make this a non-fatal error
		Fatal(err)
	}

	app.Gui.SetStatus("", "Connecting...")
	app.ConnectAll()
	app.Gui.MainWindow.ShowAll()
	fmt.Println("entering main...")

	running := true
	for running {
		gtk.MainIteration()
		select {
		case <-app.Quit:
			running = false
		case err := <-app.Errors:
			NonFatal(err)
		default:
			// fall through
		}
	}
}

func Quit(app *Application) {
	if app.Mopidy.Cmd.Process != nil {
		app.Mopidy.Cmd.Process.Kill()
	}
	app.Quit <- true
}

// InitMopidy takes the path to the mopidy executable and a loaded configuration
// and attempts to start it.
func InitMopidy(app *Application, cmd string) error {
	// look up the hostname
	hostname, ok := app.Config.Get("mpd/hostname")
	if !ok {
		return errors.New("mpd/hostname not found in config")
	} else {
		// enclose any IP addresses with a colon in brackets
		// this is important because IPv6 is supported, and mopidy's default is "::"
		if strings.Index(hostname, ":") != -1 {
			hostname = "[" + hostname + "]"
		}
	}

	// look up the port
	port, ok := app.Config.Get("mpd/port")
	if !ok {
		return errors.New("mpd/port not found in config")
	}


	errCh := make(chan error)
	app.Mopidy = &MopidyProc{Status:MopidyConnecting, Errs:errCh, Cmd:exec.Command(cmd, "--config=" + app.Config.path)}
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Panicking!")
			fmt.Println(r)
			Quit(app)
		}
	}()

	outCh := make(chan string)
	app.Mopidy.Output = new(bytes.Buffer)
	app.NewOutput = func(str string) {}

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

	// watch for errors in an idle thread
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
			}
		}
	}()

	// attempt to start mopidy in a new goroutine
	go StartMopidy(app, hostname, port, outCh, errCh)
	return nil
}

func StartMopidy(app *Application, hostname, port string, outCh chan string, errCh chan error) {
	// start mopidy
	stderr, err := app.Mopidy.Cmd.StderrPipe()
	if err != nil {
		errCh <- err
		return
	}
	go ReadOutput(app, stderr, outCh)

	if err := app.Mopidy.Cmd.Start(); err != nil {
		errCh <- errors.New("failed to start mopidy: " + err.Error())
		return
	}

	failedAttempts := 0

	// spin until 1) we get an error, 2) we connect successfully, or
	// 3) we time out with too many attempts
	for app.Mopidy.Status != MopidyFailed {
		// TODO: check to see if this error means the port is taken
		conn, err := net.Dial("tcp", hostname + ":" + port)
		if err == nil {
			app.Mopidy.Conn = conn
			app.Mopidy.Status = MopidyConnected
			app.Gui.SetStatus(gtk.STOCK_CONNECT, "Connected to Mopidy.")
			return
		}
		failedAttempts++
		if failedAttempts == 10 {
			app.Mopidy.Status = MopidyFailed
			errCh <- errors.New("failed to connect to mopidy after 10 tries")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

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

