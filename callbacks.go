package main

import (
	"fmt"
	"github.com/dradtke/gotk3/glib"
	"github.com/dradtke/gotk3/gtk"
	"os"
)

func (app *Application) QuitCallback(ctx *glib.CallbackContext) interface{} {
	app.Quit()
	return nil
}

func (app *Application) SourcesCallback(ctx *glib.CallbackContext) interface{} {
	err := app.Gui.RunDialog("DialogSources", func(response gtk.ResponseType) bool {
		// ???: this seems to get called twice on each response, once passing in
		// 0, and once passing in the correct signal. What the hell?
		switch response {
		case gtk.RESPONSE_CLOSE, gtk.RESPONSE_CANCEL:
			return false
		case gtk.RESPONSE_OK:
			return false
		default:
			return true
		}
	})
	if err != nil {
		// how to handle this failure?
		fmt.Fprintln(os.Stderr, "failed to run dialog:", err.Error())
	}
	return nil
}

func (app *Application) OutputWindowDeleteCallback(ctx *glib.CallbackContext) interface{} {
	// app.Mopidy.NewOutput = func(str string) {}
	// return app.Gui.OutputWindow.HideOnDelete()
	return nil
}

func (app *Application) OutputWindowCallback(ctx *glib.CallbackContext) interface{} {
	/*
		app.Mopidy.OutputLock.Lock()
		buffer, err := app.Gui.Output.GetBuffer()
		if err != nil {
			app.Errors <- err
			return nil
		}
		buffer.SetText(app.Mopidy.Output.String())
		iter := buffer.GetIterAtOffset(-1)
		app.Mopidy.NewOutput = func(str string) {
			buffer.Insert(iter, str)
		}
		app.Mopidy.OutputLock.Unlock()
		app.Gui.OutputWindow.ShowAll()
	*/
	return nil
}

func (app *Application) StartMopidyCallback(ctx *glib.CallbackContext) interface{} {
	// go app.StartMopidy()
	return nil
}

func (app *Application) StopMopidyCallback(ctx *glib.CallbackContext) interface{} {
	// go app.StopMopidy()
	return nil
}

func (app *Application) RestartMopidyCallback(ctx *glib.CallbackContext) interface{} {
	// go app.RestartMopidy()
	return nil
}
