package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/oneworkspacestudio/my-cinema/internal/jellyfin"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (a *App) engineStatus(ready bool, msg string) {
	a.mu.Lock()
	a.ready = ready
	a.status = msg
	a.mu.Unlock()
}

// Preserve only the existing Windows app-health context. Dropping this marker
// from a parent that has it causes the pinned native encoder to fail at process
// initialization on the reference host. Never invent a compatibility layer or
// forward arbitrary layer options/account/provider environment variables.
func mediaEnvironment() []string {
	env := []string{"DOTNET_CLI_TELEMETRY_OPTOUT=1", "LANG=C.UTF-8", "LC_ALL=C.UTF-8"}
	if os.Getenv("__COMPAT_LAYER") == "DetectorsAppHealth" {
		env = append(env, "__COMPAT_LAYER=DetectorsAppHealth")
	}
	return env
}

var errEngineExit = errors.New("private media engine exited")

func (a *App) runEngine(ctx context.Context) error {
	for attempt := 0; attempt < 3; attempt++ {
		e := a.engineCycle(ctx)
		if !errors.Is(e, errEngineExit) {
			return e
		}
		if attempt == 2 {
			a.engineStatus(false, "The private media engine could not finish startup after three attempts. Check its local diagnostic log, then restart My Cinema.")
			<-ctx.Done()
			return nil
		}
		a.engineStatus(false, "The private media engine is retrying startup. Your media and settings are preserved.")
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
		}
	}
	return nil
}
func (a *App) engineCycle(ctx context.Context) error {
	_, p, _ := net.SplitHostPort(a.H.Config.Listen)
	port, _ := strconv.Atoi(p)
	port += 100
	if port > 65535 {
		return errors.New("choose an application port below 65436")
	}
	a.engine = jellyfin.New(fmt.Sprintf("http://127.0.0.1:%d", port))
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	runtime := os.Getenv("MYCINEMA_RUNTIME")
	if runtime == "" {
		runtime, e = prepareRuntime(a.H.Config.Data, exe)
		if e != nil {
			a.engineStatus(false, e.Error())
			<-ctx.Done()
			return nil
		}
	}
	for _, name := range []string{"jellyfin.exe", "ffmpeg.exe", "ffprobe.exe"} {
		if _, e = os.Stat(filepath.Join(runtime, name)); e != nil {
			a.engineStatus(false, "The bundled media engine is missing. Extract the complete My Cinema distribution and restart.")
			<-ctx.Done()
			return nil
		}
	}
	// Reserve only our configured listener; never stop an unrelated process.
	l, e := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if e != nil {
		a.engineStatus(false, "The private media port is already in use. Choose another application port and restart.")
		<-ctx.Done()
		return nil
	}
	l.Close()
	root := filepath.Join(a.H.Config.Data, "jellyfin")
	for _, name := range []string{"data", "config", "cache", "logs"} {
		if e = os.MkdirAll(filepath.Join(root, name), 0700); e != nil {
			return e
		}
	}
	config := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?><NetworkConfiguration><BaseUrl></BaseUrl><EnableHttps>false</EnableHttps><RequireHttps>false</RequireHttps><InternalHttpPort>%d</InternalHttpPort><PublicHttpPort>%d</PublicHttpPort><EnableIPv4>true</EnableIPv4><EnableIPv6>false</EnableIPv6><EnableRemoteAccess>false</EnableRemoteAccess><LocalNetworkAddresses><string>127.0.0.1</string></LocalNetworkAddresses><EnableUPnP>false</EnableUPnP><EnablePublishedServerUriByRequest>false</EnablePublishedServerUriByRequest><AutoDiscovery>false</AutoDiscovery></NetworkConfiguration>`, port, port)
	if e = os.WriteFile(filepath.Join(root, "config", "network.xml"), []byte(config), 0600); e != nil {
		return e
	}
	var encoderVersion bytes.Buffer
	if e = apphost.Service(ctx, filepath.Join(runtime, "ffmpeg.exe"), root, []string{"-version"}, mediaEnvironment(), &encoderVersion); e != nil || !strings.HasPrefix(encoderVersion.String(), "ffmpeg version 8.1.3-Jellyfin") {
		a.engineStatus(false, "The bundled video encoder could not start. Check the local diagnostic log and restart My Cinema.")
		a.H.Log.Error("bundled encoder validation failed", "error", fmt.Sprint(e), "output", encoderVersion.String())
		<-ctx.Done()
		return nil
	}
	log, e := os.OpenFile(filepath.Join(root, "logs", "sidecar.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer log.Close()
	serviceCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- apphost.Service(serviceCtx, filepath.Join(runtime, "jellyfin.exe"), root, []string{"--datadir", filepath.Join(root, "data"), "--configdir", filepath.Join(root, "config"), "--cachedir", filepath.Join(root, "cache"), "--logdir", filepath.Join(root, "logs"), "--ffmpeg", filepath.Join(runtime, "ffmpeg.exe"), "--nowebclient", "--service", "--nonetchange"}, mediaEnvironment(), log)
	}()
	ready := false
	for start := time.Now(); time.Since(start) < 3*time.Minute; {
		select {
		case <-ctx.Done():
			cancel()
			<-done
			return nil
		case <-done:
			a.engineStatus(false, "The private media engine stopped during startup. See the local diagnostic log and restart.")
			return errEngineExit
		case <-time.After(time.Second):
		}
		var info struct {
			Version                string
			StartupWizardCompleted bool
		}
		reqCtx, stop := context.WithTimeout(ctx, 2*time.Second)
		e = a.engine.Do(reqCtx, "GET", "/System/Info/Public", "", nil, &info)
		stop()
		if e != nil {
			continue
		}
		if info.Version != jellyfin.Version && info.Version != jellyfin.Version+".0" && info.Version != jellyfin.Version+".0.0" {
			a.H.Log.Error("media version mismatch", "actual", info.Version, "expected", jellyfin.Version)
			a.engineStatus(false, "The media engine version does not match this release. Restore the pinned runtime.")
			break
		}
		// Public info can answer before migrations finish. Wait for full health.
		if e = a.engine.Do(ctx, "GET", "/health", "", nil, nil); e != nil {
			continue
		}
		if !info.StartupWizardCompleted {
			var setupReady any
			// Public info and health can answer while migrations still serve a
			// 200 HTML startup page. Wait for the typed setup API to be ready.
			if e = a.engine.Do(ctx, "GET", "/Startup/Configuration", "", nil, &setupReady); e != nil {
				continue
			}
			if a.secrets.Admin.Token != "" {
				a.engineStatus(false, "The media database needs recovery. Restore its backup before reconnecting.")
				break
			}
			name, password := a.secrets.ProvisionName, a.secrets.ProvisionPassword
			if name == "" {
				name = "cinema-" + apphost.ID()[:12]
				password = apphost.ID() + apphost.ID()
				a.mu.Lock()
				a.secrets.ProvisionName = name
				a.secrets.ProvisionPassword = password
				e = a.saveSecretsLocked()
				a.mu.Unlock()
				if e != nil {
					return e
				}
			}
			e = a.engine.Do(ctx, "POST", "/Startup/Configuration", "", map[string]any{"UICulture": "en-GB", "MetadataCountryCode": "GB", "PreferredMetadataLanguage": "en"}, nil)
			if e == nil {
				var first any
				e = a.engine.Do(ctx, "GET", "/Startup/User", "", nil, &first)
			}
			if e == nil {
				e = a.engine.Do(ctx, "POST", "/Startup/User", "", map[string]string{"Name": name, "Password": password}, nil)
			}
			if e == nil {
				e = a.engine.Do(ctx, "POST", "/Startup/RemoteAccess", "", map[string]bool{"EnableRemoteAccess": false, "EnableAutomaticPortMapping": false}, nil)
			}
			if e == nil {
				e = a.engine.Do(ctx, "POST", "/Startup/Complete", "", nil, nil)
			}
			if e == nil {
				cred, err := a.engine.Authenticate(ctx, name, password)
				e = err
				if e == nil {
					a.mu.Lock()
					a.secrets.Admin = cred
					a.secrets.ProvisionName = ""
					a.secrets.ProvisionPassword = ""
					e = a.saveSecretsLocked()
					a.mu.Unlock()
				}
			}
		}
		if info.StartupWizardCompleted && a.secrets.Admin.Token == "" && a.secrets.ProvisionName != "" {
			cred, err := a.engine.Authenticate(ctx, a.secrets.ProvisionName, a.secrets.ProvisionPassword)
			e = err
			if e == nil {
				a.mu.Lock()
				a.secrets.Admin = cred
				a.secrets.ProvisionName = ""
				a.secrets.ProvisionPassword = ""
				e = a.saveSecretsLocked()
				a.mu.Unlock()
			}
		}
		if e == nil && a.secrets.Admin.Token != "" {
			var current any
			e = a.engine.Do(ctx, "GET", "/Users/"+a.secrets.Admin.UserID, a.secrets.Admin.Token, nil, &current)
		}
		if jellyfin.Transient(e) {
			continue
		}
		if e != nil || a.secrets.Admin.Token == "" {
			if e != nil {
				a.H.Log.Error("engine bootstrap failed", "kind", e.Error())
			}
			a.engineStatus(false, "The media connection could not be unlocked. Restore the matching data and bridge backup.")
			break
		}
		ready = true
		a.engineStatus(true, "Your private media engine is ready")
		break
	}
	if !ready {
		cancel()
		<-done
		<-ctx.Done()
		return nil
	}
	reconcileCtx, reconcileCancel := context.WithCancel(ctx)
	defer reconcileCancel()
	go a.reconcile(reconcileCtx)
	select {
	case <-ctx.Done():
		stop, finish := context.WithTimeout(context.Background(), 5*time.Second)
		_ = a.engine.Do(stop, "POST", "/System/Shutdown", a.secrets.Admin.Token, nil, nil)
		finish()
		cancel()
		<-done
		return nil
	case <-done:
		a.engineStatus(false, "The media engine stopped. Your files are safe. Restart My Cinema to reconnect.")
		<-ctx.Done()
		return nil
	}
}

func validateRoot(path string) (string, error) {
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n") {
		return "", errors.New("choose an absolute media folder")
	}
	p, e := filepath.Abs(filepath.Clean(path))
	if e != nil {
		return "", e
	}
	st, e := os.Stat(p)
	if e != nil || !st.IsDir() {
		return "", errors.New("the media folder is unavailable")
	}
	for q := p; ; q = filepath.Dir(q) {
		if rejectLink(q) {
			return "", errors.New("media roots cannot traverse links or junctions")
		}
		if q == filepath.Dir(q) {
			break
		}
	}
	f, e := os.Open(p)
	if e != nil {
		return "", errors.New("the media folder cannot be read")
	}
	f.Close()
	return p, nil
}

type Library struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Path string `json:"path"`
}

func (a *App) libraries() ([]Library, error) {
	rows, e := a.H.DB.Query("SELECT id,name,kind,path FROM libraries ORDER BY name")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Library{}
	for rows.Next() {
		var l Library
		if e = rows.Scan(&l.ID, &l.Name, &l.Kind, &l.Path); e != nil {
			return nil, e
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
func (a *App) settings(w http.ResponseWriter, r *http.Request) {
	if !a.admin(w, r) {
		return
	}
	ls, e := a.libraries()
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	for i := range ls {
		if _, e = validateRoot(ls[i].Path); e != nil {
			ls[i].Kind += " (unavailable)"
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	apphost.JSON(w, 200, map[string]any{"libraries": ls, "ready": a.ready, "message": a.status, "jellyfinVersion": jellyfin.Version, "mediaOwnership": "Removing a library never deletes its files. Use the same Movies and TV Shows folders in My Drive.", "runtime": "Bundled, private loopback sidecar", "secrets": "Windows DPAPI; re-link required on relocation"})
}
func (a *App) addLibrary(w http.ResponseWriter, r *http.Request) {
	if !a.admin(w, r) || !a.media(w) {
		return
	}
	var in struct {
		Name, Kind, Path string
		Metadata         bool
	}
	if !apphost.Decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 1 || len(in.Name) > 80 || (in.Kind != "movies" && in.Kind != "tvshows" && in.Kind != "homevideos") {
		apphost.Problem(w, 400, "Choose a library name and type")
		return
	}
	path, e := validateRoot(in.Path)
	if e != nil {
		apphost.Problem(w, 400, e.Error())
		return
	}
	// Never index managed app state or the whole system disk.
	if filepath.Dir(path) == path || inside(path, a.H.Config.Data) || inside(a.H.Config.Data, path) {
		apphost.Problem(w, 400, "Choose a dedicated media folder outside the private app data")
		return
	}
	var count int
	_ = a.H.DB.QueryRow("SELECT count(*) FROM libraries WHERE path=? OR name=?", path, in.Name).Scan(&count)
	if count != 0 {
		apphost.Problem(w, 409, "That folder or library name is already registered")
		return
	}
	e = a.engine.Do(r.Context(), "POST", "/Library/VirtualFolders?name="+urlQuery(in.Name)+"&collectionType="+in.Kind+"&refreshLibrary=true", a.secrets.Admin.Token, map[string]any{"LibraryOptions": map[string]any{"PathInfos": []map[string]string{{"Path": path}}, "EnableRealtimeMonitor": true, "EnableAutomaticSeriesGrouping": true, "EnableInternetProviders": in.Metadata}}, nil)
	if e != nil {
		a.fail(w, e)
		return
	}
	id := apphost.ID()
	_, e = a.H.DB.Exec("INSERT INTO libraries VALUES(?,?,?,?)", id, in.Name, in.Kind, path)
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	a.H.Audit(apphost.Current(r).User.ID, "library-added", id)
	apphost.JSON(w, 201, map[string]any{"id": id, "scanning": true})
}
func (a *App) removeLibrary(w http.ResponseWriter, r *http.Request) {
	if !a.admin(w, r) || !a.media(w) {
		return
	}
	var name string
	if a.H.DB.QueryRow("SELECT name FROM libraries WHERE id=?", r.PathValue("id")).Scan(&name) != nil {
		apphost.Problem(w, 404, "Library not found")
		return
	}
	if e := a.engine.Do(r.Context(), "DELETE", "/Library/VirtualFolders?name="+urlQuery(name)+"&refreshLibrary=true", a.secrets.Admin.Token, nil, nil); e != nil {
		a.fail(w, e)
		return
	}
	_, e := a.H.DB.Exec("DELETE FROM libraries WHERE id=?", r.PathValue("id"))
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	apphost.JSON(w, 200, map[string]bool{"removed": true, "filesRetained": true})
}
func (a *App) scan(w http.ResponseWriter, r *http.Request) {
	if !a.admin(w, r) || !a.media(w) {
		return
	}
	if e := a.engine.Do(r.Context(), "POST", "/Library/Refresh", a.secrets.Admin.Token, nil, nil); e != nil {
		a.fail(w, e)
		return
	}
	apphost.JSON(w, 202, map[string]bool{"scanning": true})
}
func inside(root, path string) bool {
	rel, e := filepath.Rel(root, path)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// Polling supplements Jellyfin's watcher. Two identical snapshots are required;
// files still growing never trigger this application's reconciliation scan.
func (a *App) reconcile(ctx context.Context) {
	tick := time.NewTicker(45 * time.Second)
	defer tick.Stop()
	last, stable := "", ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			ls, e := a.libraries()
			if e != nil {
				continue
			}
			var snapshot strings.Builder
			for _, l := range ls {
				p, e := validateRoot(l.Path)
				if e != nil {
					continue
				}
				_ = filepath.WalkDir(p, func(path string, d os.DirEntry, e error) error {
					if e != nil {
						return filepath.SkipDir
					}
					if rejectLink(path) {
						if d.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
					if !d.IsDir() && isMedia(path) {
						if st, e := d.Info(); e == nil {
							fmt.Fprintf(&snapshot, "%s:%d:%d\n", path, st.Size(), st.ModTime().UnixNano())
						}
					}
					return nil
				})
			}
			now := apphost.Hash(snapshot.String())
			if now == last && now != stable {
				stop, cancel := context.WithTimeout(ctx, 30*time.Second)
				e = a.engine.Do(stop, "POST", "/Library/Refresh", a.secrets.Admin.Token, nil, nil)
				cancel()
				if e == nil {
					stable = now
				}
			}
			last = now
		}
	}
}
func isMedia(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".mp4", ".mkv", ".webm", ".mov", ".avi", ".m4v", ".ts":
		return true
	}
	return false
}
