package main

import (
	"errors"
	"github.com/dradtke/gotk3/gtk"
	"github.com/dradtke/wetsuit/config"
	"github.com/dradtke/wetsuit/gui"
	"github.com/dradtke/wetsuit/server"
	"github.com/dradtke/wetsuit/server/mopidy"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sync"
)

type Application struct {
	//Mopidy *MopidyProc
	Server       *server.Instance
	ServerConfig *config.Properties
	Gui          *gui.Gui

	Errors       chan error // channel of errors to be displayed
	ShowingError bool

	Work chan func() // channel of functions to be run in the main thread

	Running    bool
	StatusLock sync.Mutex
}

// Program entry point.
func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	gtk.Init(nil)

	app := new(Application)
	app.Errors = make(chan error)
	app.Work = make(chan func())
	app.Running = true

	srv := mopidy.New()

	var userConfigPath string

	// find the user's configuration
	usr, err := user.Current()
	if err == nil {
		userConfigPath = filepath.Join(usr.HomeDir, ".config", "wetsuit", srv.Name()+".conf")
	} else {
		// no user =/
		app.Fatal(err)
	}

	// load configuration
	if p, err := config.Load(userConfigPath); err == nil {
		app.ServerConfig = p
	} else {
		app.Fatal(err)
	}

	// create the window
<<<<<<< HEAD
	app.Gui, err = gui.Init(app.Config, app.Callbacks())
=======
	app.Gui, err = InitGUI(app.ServerConfig)
>>>>>>> 5029df1033df414601b62c79f1ab3b288998077e
	if err != nil {
		app.Fatal(err)
	}

	app.Gui.MainWindow.ShowAll()

	// in a separate goroutine, attempt to start and connect to it
	go func() {
		err := srv.Start(userConfigPath)
		if err != nil {
			app.Errors <- err
		}
		hostname, _ := app.ServerConfig.Get("mpd/hostname")
		port, _ := app.ServerConfig.Get("mpd/port")
		stop := make(chan bool, 1)
		ok, err := srv.Connect(hostname, port, stop)
		if err != nil {
			app.Errors <- err
		}
		if !ok {
			// how to react to this?
		}
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

// Do() runs a function in the main thread, waiting until it finishes.
func (app *Application) Do(f func()) {
	done := make(chan bool, 1)
	app.Work <- func() {
		f()
		done <- true
	}
	<-done
}

// SetStatus() updates the Gui's status based on the value of the provided enum.
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

// Quit() quits the application.
func (app *Application) Quit() {
	if app.Mopidy.Cmd.Process != nil {
		app.Mopidy.Cmd.Process.Kill()
	}
	app.Running = false
}

// Callbacks() returns a map from widget name and signal to callback function.
// It's used during Gui initialization to make all the necessary connections.
func (app *Application) Callbacks() (cb map[string]map[string]gui.Callback) {
	cb = make(map[string]map[string]gui.Callback)
	cb["main-window"] = map[string]gui.Callback{"destroy": app.QuitCallback}
	cb["menu-quit"] = map[string]gui.Callback{"activate": app.QuitCallback}
	cb["menu-server-output"] = map[string]gui.Callback{"activate": app.OutputWindowCallback}
	cb["menu-server-start"] = map[string]gui.Callback{"activate": app.StartMopidyCallback}
	cb["menu-server-stop"] = map[string]gui.Callback{"activate": app.StopMopidyCallback}
	cb["menu-server-restart"] = map[string]gui.Callback{"activate": app.RestartMopidyCallback}
	cb["menu-sources"] = map[string]gui.Callback{"activate": app.SourcesCallback}
	cb["output-window"] = map[string]gui.Callback{"delete-event": app.OutputWindowDeleteCallback}
	return
}
