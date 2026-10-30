package main

import (
	"embed"
	"io"
	"log"
	"os"
	"slices"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"wdtt-plus/client/internal/applog"
	"wdtt-plus/client/internal/platform"
	"wdtt-plus/client/internal/profile"
)

type ignoreErrors struct{ w io.Writer }

func (i ignoreErrors) Write(p []byte) (int, error) {
	_, _ = i.w.Write(p)
	return len(p), nil
}

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	dataDir, err := profile.DataDir()
	if err != nil {
		log.Fatal(err)
	}
	logs := applog.Open(dataDir)
	defer logs.Close()
	// A Windows GUI process has no console: writes to stderr fail, and
	// io.MultiWriter would stop before reaching the log.
	log.SetOutput(io.MultiWriter(logs, ignoreErrors{os.Stderr}))
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Printf("[СИСТЕМА] WDTT Plus %s", version)

	app := NewApp(dataDir, logs)
	opts := &options.App{
		Title:     "WDTT Plus",
		Width:     1080,
		Height:    720,
		MinWidth:  860,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 17, G: 19, B: 24, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []any{app},
		Windows: &windows.Options{
			WebviewUserDataPath: dataDir + `\webview`,
			OnSuspend:           app.manager.Suspend,
			OnResume:            app.manager.Resume,
		},
	}
	// An elevated relaunch starts while the previous instance is still exiting,
	// so it must not hand its arguments over to that instance.
	if !slices.Contains(os.Args[1:], platform.ElevatedFlag) {
		opts.SingleInstanceLock = &options.SingleInstanceLock{
			UniqueId: "wdtt-plus-pc-3f6c1a52",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				app.onSecondInstance(data.Args)
			},
		}
	}
	if err := wails.Run(opts); err != nil {
		log.Fatal(err)
	}
}
