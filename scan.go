package main

import (
	"github.com/dradtke/wetsuit/gui"
)

func (app *Application) Scan(root string) {
	/*
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			filetype := mime.TypeByExtension(filepath.Ext(path))
			if strings.HasPrefix(filetype, "audio/") {
				fmt.Println(path)
				_, err := app.Conn.Send(`add "file://`+path+`"`)
				if err != nil {
					app.Errors <- err
				}
			}
			return nil
		})
	*/
	app.Gui.AddRow(&gui.ViewRow{Title: "Fuck Me Slowly", Artist: "Tenacious D"})
	app.Gui.AddRow(&gui.ViewRow{Title: "Nightlife", Artist: "Green Day"})
}
