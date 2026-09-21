package main

import "context"

type App struct{ ctx context.Context }

type Capabilities struct {
	Product       string `json:"product"`
	VideoCover    bool   `json:"videoCover"`
	StartAtZero   bool   `json:"startAtZero"`
	SafeBatch     bool   `json:"safeBatch"`
	SafeDownloads bool   `json:"safeDownloads"`
}

func NewApp() *App { return &App{} }
func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) Capabilities() Capabilities {
	return Capabilities{
		Product: "TDL Desktop", VideoCover: true, StartAtZero: true,
		SafeBatch: true, SafeDownloads: true,
	}
}
