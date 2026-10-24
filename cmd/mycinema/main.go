package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/oneworkspacestudio/my-cinema/internal/app"
	"github.com/oneworkspacestudio/my-cinema/internal/webui"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

var version = "0.1.0-preview.1"

func run() error {
	f := flag.NewFlagSet("cinema", flag.ContinueOnError)
	data := f.String("data", "", "Dedicated protected app data folder")
	listen := f.String("listen", "127.0.0.1:9020", "Loopback listener")
	origin := f.String("origin", "", "Public application URL")
	noBrowser := f.Bool("no-browser", false, "Do not open the browser")
	bootstrap := f.String("bootstrap-email", "", "Standalone first owner; password from WORKSPACE_BOOTSTRAP_PASSWORD")
	showVersion := f.Bool("version", false, "Print version")
	if e := f.Parse(os.Args[1:]); e != nil {
		return e
	}
	if *showVersion {
		fmt.Println(version)
		return nil
	}
	if *data == "" {
		exe, e := os.Executable()
		if e != nil {
			return e
		}
		*data = filepath.Join(filepath.Dir(exe), "data")
	}
	absolute, e := filepath.Abs(*data)
	if e != nil {
		return e
	}
	if *origin == "" {
		*origin = "http://" + strings.Replace(*listen, "127.0.0.1", "localhost", 1)
	}
	cfg := apphost.FromEnv(apphost.Config{ID: "cinema", Name: "My Cinema", Version: version, Data: absolute, Listen: *listen, Origin: *origin})
	host, e := apphost.New(cfg, webui.Files())
	if e != nil {
		return e
	}
	defer host.Close()
	if *bootstrap != "" {
		if e = host.Bootstrap(*bootstrap, os.Getenv("WORKSPACE_BOOTSTRAP_PASSWORD")); e != nil {
			return e
		}
		os.Unsetenv("WORKSPACE_BOOTSTRAP_PASSWORD")
	}
	application, e := app.New(host)
	if e != nil {
		return e
	}
	// The generic apphost snapshot excludes the sidecar. Never silently offer an
	// incomplete media backup through either the public API or a CLI flag.
	host.Mux.HandleFunc("POST /api/backup", host.Owner(func(w http.ResponseWriter, r *http.Request) {
		apphost.Problem(w, 409, "Use Workspace Backups for a complete media backup, or close Cinema and preserve its full data folder. See the included setup guide.")
	}))
	link := host.Config.Origin + "/"
	if host.SetupURL() != "" {
		link = host.SetupURL()
	}
	fmt.Println("My Cinema: " + link)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if !*noBrowser {
		cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", link)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if e = cmd.Start(); e == nil {
			go cmd.Wait()
		}
	}
	done := make(chan error, 1)
	go func() {
		e := application.Run(ctx)
		if e != nil {
			cancel()
		}
		done <- e
	}()
	serverErr := host.Run(ctx)
	cancel()
	workerErr := <-done
	if serverErr != nil && !errors.Is(serverErr, context.Canceled) {
		return serverErr
	}
	return workerErr
}
func main() {
	if e := run(); e != nil {
		apphost.Report(e)
		os.Exit(1)
	}
}
