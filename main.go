package main

import (
	"github.com/dradtke/gotk3/gtk"
	"github.com/dradtke/wetsuit/config"
	"sync"

	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
)

type Application struct {
	Mopidy *MopidyProc
	Config *config.Properties
	Gui    *GUI

	Errors       chan error // channel of errors to be displayed
	ShowingError bool

	Work chan func() // channel of functions to be run in the main thread

	Running bool
	StatusLock sync.Mutex
}

// Program entry point.
func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	app := new(Application)
	app.Errors = make(chan error)
	app.Work = make(chan func())
	app.Running = true

	gtk.Init(nil)
	var mopidyCmdPath, userConfigPath string

	// make sure mopidy is installed
	mopidyCmdPath, err := exec.LookPath("mopidy")
	if err != nil {
		app.Fatal(errors.New("Mopidy is not installed."))
	}

	// find the user's configuration
	usr, err := user.Current()
	if err == nil {
		userConfigPath = filepath.Join(usr.HomeDir, ".config", "wetsuit", "mopidy.conf")
	} else {
		// no user =/
		app.Fatal(err)
	}

	// load configuration
	if p, err := config.Load(userConfigPath); err == nil {
		app.Config = p
	} else {
		app.Fatal(err)
	}

	// create the window
	app.Gui, err = InitGUI(app.Config)
	if err != nil {
		app.Fatal(err)
	}

	app.ConnectAll()
	app.Gui.MainWindow.ShowAll()

	go func() {
		err := app.InitMopidy(mopidyCmdPath)
		if err != nil {
			app.Errors <- err
		}
		// attempt to start mopidy
		app.StartMopidy()
	}()

	// custom iterator so that we can watch channels
	for app.Running {
		gtk.MainIteration()

		// check for main thread work
		select {
		case f := <-app.Work:
			f()
		default:
			// fall through
		}

		// if no error is currently showing, check for
		// error messages to display
		if !app.ShowingError {
			select {
			case err := <-app.Errors:
				app.ShowingError = true
				app.NonFatal(err)
				app.Disable()
			default:
				// fall through
			}
		}
	}
}

// Fatal() displays an error dialog, then quits the program when it's closed.
func (app *Application) Fatal(err error) {
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

// NonFatal() displays an error dialog, but the program keeps running after it's closed.
// This should not be called from anywhere but main(), since it needs to run on GTK's thread.
func (app *Application) NonFatal(err error) {
	dialog := gtk.MessageDialogNew(nil, 0, gtk.MESSAGE_ERROR, gtk.BUTTONS_CLOSE, err.Error())
	dialog.Connect("response", func() {
		dialog.Destroy()
		app.ShowingError = false
	})
	dialog.Show()
}

// Do() runs a function in the main thread.
func (app *Application) Do(f func()) {
	done := make(chan bool, 1)
	app.Work <- func() {
		f()
		done <- true
	}
	<-done
}

func (app *Application) SetStatus(status MopidyStatus) {
	app.StatusLock.Lock()
	defer app.StatusLock.Unlock()

	app.Mopidy.Status = status
	switch status {
	case MopidyConnecting:
		app.Gui.SetStatus("", "Connecting...")
	case MopidyConnected:
		app.Gui.SetStatus(gtk.STOCK_CONNECT, "Connected to Mopidy.")
	case MopidyFailed:
		app.Gui.SetStatus("", "Not connected.")
	}
}

func Quit(app *Application) {
	if app.Mopidy.Cmd.Process != nil {
		app.Mopidy.Cmd.Process.Kill()
	}
	app.Running = false
}

