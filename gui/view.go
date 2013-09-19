package gui

import (
	"fmt"
	"github.com/dradtke/gotk3/glib"
	"github.com/dradtke/gotk3/gtk"
)

type viewColumn struct {
	name     string
	title    string
	gtype    glib.Type
	renderer gtk.ICellRenderer
}

var definedColumns = []viewColumn{
	viewColumn{name: "title", title: "Title", gtype: glib.TYPE_STRING},
	viewColumn{name: "artist", title: "Artist", gtype: glib.TYPE_STRING},
}

type ViewRow struct {
	Title  string
	Artist string
}

func (g *Gui) initViewModel() error {
	typeMap := make(map[string]glib.Type)
	for _, col := range definedColumns {
		typeMap[col.name] = col.gtype
	}
	model, err := gtk.ListStoreNew(typeMap)
	if err != nil {
		panic(err)
	}
	g.viewModel = model
	g.priv.View.SetModel(model)
	for _, col := range definedColumns {
		attributes := make(map[string]string)
		if col.renderer == nil {
			switch col.gtype {
			case glib.TYPE_STRING:
				renderer, err := gtk.CellRendererTextNew()
				if err != nil {
					return err
				}
				col.renderer = renderer
				attributes["text"] = col.name
			default:
				return fmt.Errorf("unrecognized gtype for column '%s'", col.name)
			}
		}
		treeViewColumn, err := gtk.TreeViewColumnNewWithAttributes(col.title, col.renderer, model, attributes)
		if err != nil {
			return err
		}
		g.priv.View.AppendColumn(treeViewColumn)
	}
	return nil
}

func (g *Gui) AddRow(row *ViewRow) (*gtk.TreeIter, error) {
	return g.viewModel.InsertWithValues(-1, map[string]interface{}{
		"title":  row.Title,
		"artist": row.Artist,
	})
}
