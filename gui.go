package main

import (
	"github.com/dradtke/gotk3/glib"
	"github.com/dradtke/gotk3/gtk"
	"github.com/dradtke/wetsuit/config"
	"fmt"
	"os"
	"reflect"
)

type GUI struct {
	MainWindow   *gtk.Window    `build:"main-window"`
	OutputWindow *gtk.Window    `build:"output-window"`
	Status       *gtk.Statusbar `build:"status"`
	Notebook     *gtk.Notebook  `build:"notebook"`
	Output       *gtk.TextView  `build:"output"`

	MenuQuit    *gtk.ImageMenuItem `build:"menu-quit"`
	MenuSources *gtk.MenuItem      `build:"menu-sources"`
	MenuOutput  *gtk.MenuItem      `build:"menu-server-output"`
	MenuStart   *gtk.ImageMenuItem `build:"menu-server-start"`
	MenuStop    *gtk.ImageMenuItem `build:"menu-server-stop"`
	MenuRestart *gtk.ImageMenuItem `build:"menu-server-restart"`

	DialogSources struct {
		Window      *gtk.Dialog            `build:"dialog-sources"`
		Ok          *gtk.Button            `build:"dialog-sources-ok"`
		Apply       *gtk.Button            `build:"dialog-sources-apply"`
		Cancel      *gtk.Button            `build:"dialog-sources-cancel"`
		MusicFolder *gtk.FileChooserButton `build:"dialog-sources-music-folder"`
	} `build:"..."`

	statusMessageArea *gtk.Box
	statusMessageIcon *gtk.Image
	statusMessageText *gtk.Label
	disabledTabs      []struct {
		Label *gtk.Widget
		Page  *gtk.Widget
	}
}

// RunDialog() encapsulates much of the logic for displaying an application dialog.
// Given the name of the dialog to run, it will check the GUI struct for a
// field of that name. If found, it uses reflection to search for a number of
// particularly-named fields, in particular looking for "Window" to be the *gtk.Dialog
// instance as well as a number of common button names. It hooks up the signal handlers
// for all of them, passing pre-defined response types to the callback provided.
// If that callback returns false, the dialog is hidden; otherwise it sticks around.
func (gui *GUI) RunDialog(name string, respond func(gtk.ResponseType) bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	var w *gtk.Dialog = nil
	handlers := make([]struct {
		Id     int
		Object *glib.Object
	}, 1)
	structVal := reflect.ValueOf(gui).Elem().FieldByName(name)

	connect := func(object *glib.Object, signal string, response gtk.ResponseType) {
		h, err := object.Connect(signal, func() { w.Response(response) })
		if err != nil {
			panic(err)
		}
		handlers = append(handlers, struct {
			Id     int
			Object *glib.Object
		}{h, object})
	}

	for i := 0; i < structVal.NumField(); i++ {
		field := structVal.Field(i)
		object := field.Interface()
		switch structVal.Type().Field(i).Name {
		case "Window":
			w = object.(*gtk.Dialog)
			h, err := w.Connect("response", func(ctx *glib.CallbackContext) {
				if !respond(gtk.ResponseType(ctx.Arg(0).Int())) {
					w.Hide()
					for _, handler := range handlers {
						handler.Object.HandlerDisconnect(handler.Id)
					}
				}
			})
			if err != nil {
				return err
			}
			handlers[0] = struct {
				Id     int
				Object *glib.Object
			}{h, w.Object}
		case "Ok":
			connect(object.(*gtk.Button).Object, "clicked", gtk.RESPONSE_OK)
		case "Apply":
			connect(object.(*gtk.Button).Object, "clicked", gtk.RESPONSE_APPLY)
		case "Cancel":
			connect(object.(*gtk.Button).Object, "clicked", gtk.RESPONSE_CANCEL)
		}
	}
	if w != nil {
		w.Run()
		return nil
	} else {
		return fmt.Errorf("window field not found for dialog '%s'", name)
	}
}

func (gui *GUI) SetStatus(icon gtk.Stock, msg string) {
	gui.statusMessageIcon.SetFromStock(icon, gtk.ICON_SIZE_MENU)
	gui.statusMessageText.SetText(msg)
}

// LoadWidgets() loads widgets into the struct by checking the "build" tag of each field.
// The special build-tag "..." causes it to be called recursively on that field.
func LoadWidgets(structVal reflect.Value, builder *gtk.Builder) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	// load widgets dynamically by tag
	for i := 0; i < structVal.NumField(); i++ {
		field := structVal.Field(i)
		widget := structVal.Type().Field(i).Tag.Get("build")
		if widget == "" {
			continue
		} else if widget == "..." {
			// recursively load additional structs
			err := LoadWidgets(field, builder)
			if err != nil {
				return err
			}
			continue
		}
		obj, err := builder.GetObject(widget)
		if err != nil {
			return err
		}
		w := reflect.ValueOf(obj).Convert(field.Type())
		field.Set(w)
	}
	return nil
}

func InitGUI(cfg *config.Properties) (gui *GUI, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	builder, err := gtk.BuilderNew()
	if err != nil {
		return nil, err
	}

	if err := builder.AddFromFile("src/github.com/dradtke/wetsuit/wetsuit.ui"); err != nil {
		return nil, err
	}

	gui = new(GUI)

	err = LoadWidgets(reflect.ValueOf(gui).Elem(), builder)
	if err != nil {
		return nil, err
	}

	// set up the status bar
	area, err := gui.Status.GetMessageArea()
	if err != nil {
		return nil, err
	}
	gui.statusMessageArea = area
	icon, err := gtk.ImageNewFromStock("", gtk.ICON_SIZE_MENU)
	if err != nil {
		return nil, err
	}
	gui.statusMessageIcon = icon
	label, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	gui.statusMessageText = label
	gui.statusMessageArea.PackStart(gui.statusMessageIcon, false, false, 0)
	gui.statusMessageArea.PackStart(gui.statusMessageText, false, false, 0)

	// TODO: see if we can somehow set the music folder button to the existing music folder
	mediaDir, err := cfg.Get("local/media_dir")
	if err != nil {
		if mediaDir == "$XDG_MUSIC_DIR" {
			mediaDir = os.ExpandEnv(mediaDir)
		}
		if mediaDir != "" {
			gui.DialogSources.MusicFolder.SetCurrentFolder(mediaDir)
		} else {
			cfg.SetBool("local/enabled", false)
		}
	} else {
		cfg.SetBool("local/enabled", false)
	}

	// disable tabs
	// TODO: default this to enabled if the key isn't found
	if enabled, err := cfg.GetBool("local/enabled"); !enabled || err != nil {
		gui.DisableTab("local")
	}
	if enabled, err := cfg.GetBool("spotify/enabled"); !enabled || err != nil {
		gui.DisableTab("spotify")
	}

	// TODO: if everything is disabled, point users to the Sources... dialog

	return gui, nil
}

// Removes the tab whose buildable name is equal to "tab-${name}" and adds its label and page
// to the gui struct's disabledTabs field, in case we want to re-enable it later.
func (gui *GUI) DisableTab(name string) error {
	// tab-${name}
	for i := 0; i < gui.Notebook.GetNPages(); i++ {
		page, err := gui.Notebook.GetNthPage(i)
		if err != nil {
			return err
		}

		label, err := gui.Notebook.GetTabLabel(page)
		if err != nil {
			return err
		}

		if label.GetBuildableName() == "tab-"+name {
			gui.Notebook.RemovePage(i)
			gui.disabledTabs = append(gui.disabledTabs, struct {
				Label *gtk.Widget
				Page  *gtk.Widget
			}{label, page})
			return nil
		}
	}

	return fmt.Errorf("tab '%s' not found", name)
}

func (gui *GUI) DisableAllTabs() error {
	for i := 0; i < gui.Notebook.GetNPages(); i++ {
		page, err := gui.Notebook.GetNthPage(i)
		if err != nil {
			return err
		}

		label, err := gui.Notebook.GetTabLabel(page)
		if err != nil {
			return err
		}

		gui.Notebook.RemovePage(i)
		gui.disabledTabs = append(gui.disabledTabs, struct {
			Label *gtk.Widget
			Page  *gtk.Widget
		}{label, page})
	}

	return nil
}

func (app *Application) ConnectAll() {
	app.Gui.MainWindow.Connect("destroy", func() {
		Quit(app)
	})
	app.Gui.MenuQuit.Connect("activate", func() {
		Quit(app)
	})
	app.Gui.MenuSources.Connect("activate", func() {
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
	})
	app.Gui.OutputWindow.Connect("delete-event", func() bool {
		return app.Gui.OutputWindow.HideOnDelete()
	})
	app.Gui.MenuOutput.Connect("activate", func() {
		/*
		buffer, err := app.Gui.Output.GetBuffer()
		if err != nil {
			app.Errors <- err
			return
		}
		iter := buffer.GetIterAtOffset(-1)
		app.Gui.OutputWindow.ShowAll()
		*/
	})
	app.Gui.MenuStart.Connect("activate", func() {
		// go app.StartMopidy()
	})
	app.Gui.MenuStop.Connect("activate", func() {
		// go app.StopMopidy()
	})
	app.Gui.MenuRestart.Connect("activate", func() {
		// go app.RestartMopidy()
	})
}
