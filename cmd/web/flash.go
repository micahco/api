package main

import "net/http"

type FlashMessageType string

const (
	FlashSuccess FlashMessageType = "success"
	FlashInfo    FlashMessageType = "info"
	FlashError   FlashMessageType = "error"
)

type FlashMessage struct {
	Type    FlashMessageType
	Message string
}

func (app *application) flash(r *http.Request, f FlashMessage) {
	app.sessionManager.Put(r.Context(), "flash", f)
}

func (app *application) popFlash(r *http.Request) FlashMessage {
	exists := app.sessionManager.Exists(r.Context(), "flash")

	if exists {
		f, ok := app.sessionManager.Pop(r.Context(), "flash").(FlashMessage)

		if ok {
			return f
		}
	}

	return FlashMessage{}
}
