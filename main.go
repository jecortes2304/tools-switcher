package main

import (
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"runtime"
	"slices"

	"github.com/jecortes2304/tools-switcher/internal/desktop"
	"github.com/jecortes2304/tools-switcher/internal/manager"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

//go:embed build/tray-template.png
var trayTemplate []byte

var version = "0.1.0"
var updatePublicKeyBase64 string

func main() {
	app, _, err := newApplication()
	if err != nil {
		log.Fatal(err)
	}
	if err = app.Run(); err != nil {
		log.Fatal(err)
	}
}

func newApplication() (*application.App, *application.WebviewWindow, error) {
	publicKey, err := base64.StdEncoding.DecodeString(updatePublicKeyBase64)
	if err != nil {
		return nil, nil, err
	}
	dir, err := desktop.ConfigDirectory()
	if err != nil {
		return nil, nil, err
	}
	m, err := manager.New(dir)
	if err != nil {
		return nil, nil, err
	}
	service := desktop.New(m, version, string(publicKey))
	notifier := notifications.New()
	instanceID := "com.cortestudios.tools-switcher"
	if os.Getenv("TOOLS_SWITCHER_CONFIG_DIR") != "" {
		digest := sha256.Sum256([]byte(dir))
		instanceID += fmt.Sprintf(".%x", digest[:8])
	}
	var window *application.WebviewWindow
	app := application.New(application.Options{
		Name:        "tools-switcher",
		Description: "Gestiona tus herramientas de desarrollo",
		Services: []application.Service{
			application.NewService(service),
			application.NewService(notifier),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: instanceID,
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				if window != nil {
					window.Show()
					window.Focus()
				}
			},
		},
	})
	desktop.Attach(service, app, notifier)
	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "tools-switcher",
		Width:            400,
		Height:           620,
		MinWidth:         360,
		MinHeight:        500,
		BackgroundColour: application.NewRGB(17, 19, 25),
		URL:              "/",
		Hidden:           slices.Contains(os.Args, "--background"),
	})
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if m.Settings().CloseToTray {
			window.Hide()
			event.Cancel()
		} else {
			app.Quit()
		}
	})
	tray := app.SystemTray.New()
	tray.SetIcon(appIcon)
	tray.SetDarkModeIcon(appIcon)
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(trayTemplate)
	}
	tray.SetTooltip("tools-switcher")
	menu := app.Menu.New()
	menu.Add("Abrir tools-switcher").OnClick(func(_ *application.Context) { window.Show(); window.Focus() })
	menu.Add("Salir").OnClick(func(_ *application.Context) { app.Quit() })
	tray.SetMenu(menu)
	tray.OnClick(func() { window.Show(); window.Focus() })
	return app, window, nil
}
