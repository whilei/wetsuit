package gui

import (
	"fmt"
	"github.com/dradtke/gotk3/glib"
	"github.com/dradtke/gotk3/gtk"
	"path/filepath"
	"reflect"
)

type Gui struct {
	priv *gui
	viewModel *gtk.ListStore
}

type gui struct {
	Window *gtk.Window `build:"main-window"`
	View *gtk.TreeView `build:"view"`
}

// Represents a caught signal.
type Event struct {
	Widget  string
	Signal  string
	Context *glib.CallbackContext
	Return  chan interface{}
}

type Callback func(ctx *glib.CallbackContext) interface{}

// RunDialog() encapsulates much of the logic for displaying an application dialog.
// Given the name of the dialog to run, it will check the Gui struct for a
// field of that name. If found, it uses reflection to search for a number of
// particularly-named fields, in particular looking for "Window" to be the *gtk.Dialog
// instance as well as a number of common button names. It hooks up the signal handlers
// for all of them, passing pre-defined response types to the callback provided.
// If that callback returns false, the dialog is hidden; otherwise it sticks around.
func (g *Gui) RunDialog(name string, respond func(gtk.ResponseType) bool) (err error) {
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
	structVal := reflect.ValueOf(g).Elem().FieldByName(name)

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

func (g *Gui) SetStatus(icon gtk.Stock, msg string) {
	panic("not implemented")
}

// LoadWidgets() loads widgets into the struct by checking the "build" tag of each field.
// The special build-tag "..." causes it to be called recursively on that field.
func LoadWidgets(structVal reflect.Value, builder *gtk.Builder, callbacks map[string]map[string]Callback) (err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
		}
	}()

	// load widgets dynamically by tag
	for i := 0; i < structVal.NumField(); i++ {
		field := structVal.Field(i)
		tag := structVal.Type().Field(i).Tag
		widget := tag.Get("build")
		if widget == "" {
			continue
		} else if widget == "..." {
			// recursively load additional structs
			err := LoadWidgets(field, builder, callbacks)
			if err != nil {
				return err
			}
			continue
		}
		obj, err := builder.GetObject(widget)
		if err != nil {
			return err
		}
		if signals, ok := callbacks[widget]; ok {
			for signal, f := range signals {
				obj.ToObject().Connect(signal, f)
			}
		}
		w := reflect.ValueOf(obj).Convert(field.Type())
		field.Set(w)
	}
	return nil
}

// loadUI() loads all the .ui files for the passed in builder.
func loadUI(builder *gtk.Builder) error {
	matches, err := filepath.Glob("src/github.com/dradtke/wetsuit/ui/*.ui")
	if err != nil {
		return err
	}
	for _, f := range matches {
		err := builder.AddFromFile(f)
		if err != nil {
			return err
		}
	}
	return nil
}

func Init(callbacks map[string]map[string]Callback) (g *Gui, err error) {
	builder, err := gtk.BuilderNew()
	if err != nil {
		return nil, err
	}

	if err := loadUI(builder); err != nil {
		return nil, err
	}

	g = new(Gui)
	g.priv = new(gui)

	err = LoadWidgets(reflect.ValueOf(g.priv).Elem(), builder, callbacks)
	if err != nil {
		return nil, err
	}

	err = g.initViewModel()
	if err != nil {
		return nil, err
	}

	// set up the status bar
	/*
	area, err := g.priv.Status.GetMessageArea()
	if err != nil {
		return nil, err
	}
	g.priv.StatusMessageArea = area
	icon, err := gtk.ImageNewFromStock("", gtk.ICON_SIZE_MENU)
	if err != nil {
		return nil, err
	}
	g.priv.StatusMessageIcon = icon
	label, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	g.priv.StatusMessageText = label
	g.priv.StatusMessageArea.PackStart(g.priv.StatusMessageIcon, false, false, 0)
	g.priv.StatusMessageArea.PackStart(g.priv.StatusMessageText, false, false, 0)
	*/

	// TODO: see if we can somehow set the music folder button to the existing music folder
	/*
	mediaDir, err := cfg.Get("local/media_dir")
	if err != nil {
		if mediaDir == "$XDG_MUSIC_DIR" {
			mediaDir = os.ExpandEnv(mediaDir)
		}
		if mediaDir != "" {
			g.priv.DialogSources.MusicFolder.SetCurrentFolder(mediaDir)
		} else {
			cfg.SetBool("local/enabled", false)
		}
	} else {
		cfg.SetBool("local/enabled", false)
	}
	*/

	// disable tabs
	// TODO: default this to enabled if the key isn't found
	/*
	if enabled, err := cfg.GetBool("local/enabled"); !enabled || err != nil {
		g.DisableTab("local")
	}
	if enabled, err := cfg.GetBool("spotify/enabled"); !enabled || err != nil {
		g.DisableTab("spotify")
	}
	*/

	// TODO: if everything is disabled, point users to the Sources... dialog
	return g, nil
}

func (g *Gui) Show() {
	g.priv.Window.ShowAll()
}
