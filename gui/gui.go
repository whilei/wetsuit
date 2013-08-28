package gui

import (
	"fmt"
	"github.com/dradtke/gotk3/glib"
	"github.com/dradtke/gotk3/gtk"
	"github.com/dradtke/wetsuit/config"
	"reflect"
)

type Gui struct {
	// TODO: move all of these to a private struct
	MainWindow   *gtk.Window    `build:"main-window"`
	OutputWindow *gtk.Window    `build:"output-window"`
	Status       *gtk.Statusbar `build:"status"`
	Notebook     *gtk.Notebook  `build:"notebook"`
	Output       *gtk.TextView  `build:"output"`

	Menu struct {
		Quit    *gtk.ImageMenuItem `build:"menu-quit"`
		Sources *gtk.MenuItem      `build:"menu-sources"`
		Output  *gtk.MenuItem      `build:"menu-server-output"`
		Start   *gtk.ImageMenuItem `build:"menu-server-start"`
		Stop    *gtk.ImageMenuItem `build:"menu-server-stop"`
		Restart *gtk.ImageMenuItem `build:"menu-server-restart"`
	} `build:"..."`

	DialogSources struct {
		Window *gtk.Dialog `build:"dialog-sources"`
		Ok     *gtk.Button `build:"dialog-sources-ok"`
		Apply  *gtk.Button `build:"dialog-sources-apply"`
		Cancel *gtk.Button `build:"dialog-sources-cancel"`
	} `build:"..."`

	statusMessageArea *gtk.Box
	statusMessageIcon *gtk.Image
	statusMessageText *gtk.Label
	disabledTabs      []struct {
		Label *gtk.Widget
		Page  *gtk.Widget
	}
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
	g.statusMessageIcon.SetFromStock(icon, gtk.ICON_SIZE_MENU)
	g.statusMessageText.SetText(msg)
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

func Init(cfg *config.Properties, callbacks map[string]map[string]Callback) (g *Gui, err error) {
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

	g = new(Gui)

	err = LoadWidgets(reflect.ValueOf(g).Elem(), builder, callbacks)
	if err != nil {
		return nil, err
	}

	// set up the status bar
	area, err := g.Status.GetMessageArea()
	if err != nil {
		return nil, err
	}
	g.statusMessageArea = area
	icon, err := gtk.ImageNewFromStock("", gtk.ICON_SIZE_MENU)
	if err != nil {
		return nil, err
	}
	g.statusMessageIcon = icon
	label, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	g.statusMessageText = label
	g.statusMessageArea.PackStart(g.statusMessageIcon, false, false, 0)
	g.statusMessageArea.PackStart(g.statusMessageText, false, false, 0)

	// disable tabs
	// TODO: default this to enabled if the key isn't found
	if enabled, err := cfg.GetBool("local/enabled"); !enabled || err != nil {
		g.DisableTab("local")
	}
	if enabled, err := cfg.GetBool("spotify/enabled"); !enabled || err != nil {
		g.DisableTab("spotify")
	}

	return g, nil
}

// Removes the tab whose buildable name is equal to "tab-${name}" and adds its label and page
// to the gui struct's disabledTabs field, in case we want to re-enable it later.
func (g *Gui) DisableTab(name string) error {
	// tab-${name}
	for i := 0; i < g.Notebook.GetNPages(); i++ {
		page, err := g.Notebook.GetNthPage(i)
		if err != nil {
			return err
		}

		label, err := g.Notebook.GetTabLabel(page)
		if err != nil {
			return err
		}

		if label.GetBuildableName() == "tab-"+name {
			g.Notebook.RemovePage(i)
			g.disabledTabs = append(g.disabledTabs, struct {
				Label *gtk.Widget
				Page  *gtk.Widget
			}{label, page})
			return nil
		}
	}

	return fmt.Errorf("tab '%s' not found", name)
}

func (g *Gui) DisableAllTabs() error {
	for i := 0; i < g.Notebook.GetNPages(); i++ {
		page, err := g.Notebook.GetNthPage(i)
		if err != nil {
			return err
		}

		label, err := g.Notebook.GetTabLabel(page)
		if err != nil {
			return err
		}

		g.Notebook.RemovePage(i)
		g.disabledTabs = append(g.disabledTabs, struct {
			Label *gtk.Widget
			Page  *gtk.Widget
		}{label, page})
	}

	return nil
}
