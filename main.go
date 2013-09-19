package main

import (
	"github.com/dradtke/go-freedesktop/freedesktop"
	"github.com/dradtke/go-mpd/mpd"
	"github.com/dradtke/gotk3/gtk"
	"github.com/dradtke/wetsuit/gui"
	"os"
	"runtime"
	"sync"
)

type Application struct {
	Conn *mpd.Conn
	//ServerConfig *config.Properties
	Gui *gui.Gui

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

	// TODO: take address as a parameter
	conn, err := mpd.Connect("[::]:6600")
	if err == nil {
		app.Conn = conn
	} else {
		app.Fatal(err)
	}

	/*
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
	*/
	musicDir, err := freedesktop.GetUserDir("music")
	if err != nil {
		app.Fatal(err)
	}

	// create the window
	app.Gui, err = gui.Init(app.Callbacks())
	if err != nil {
		app.Fatal(err)
	}

	app.Gui.Show()

	// load music
	go app.Scan(musicDir)

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

// Quit() quits the application.
func (app *Application) Quit() {
	/*
		if app.Mopidy.Cmd.Process != nil {
			app.Mopidy.Cmd.Process.Kill()
		}
	*/
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
