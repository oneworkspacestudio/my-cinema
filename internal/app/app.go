package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/oneworkspacestudio/my-cinema/internal/jellyfin"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type App struct {
	H         *apphost.Host
	mu        sync.Mutex
	engine    *jellyfin.Client
	secrets   Secrets
	ready     bool
	status    string
	plays     map[string]*Playback
	attempts  map[string]attempt
	uploadMu  sync.Mutex
	libraryMu sync.Mutex
	driveRoot string
}
type attempt struct {
	Count int
	Until time.Time
}
type Secrets struct {
	Admin             jellyfin.Credentials            `json:"admin"`
	Users             map[string]jellyfin.Credentials `json:"users"`
	ProvisionName     string                          `json:"provisionName,omitempty"`
	ProvisionPassword string                          `json:"provisionPassword,omitempty"`
}

func New(h *apphost.Host) (*App, error) {
	a := &App{H: h, status: "Starting the private media engine", plays: map[string]*Playback{}, attempts: map[string]attempt{}, secrets: Secrets{Users: map[string]jellyfin.Credentials{}}}
	a.driveRoot = os.Getenv("MYCINEMA_DRIVE_ROOT")
	if a.driveRoot == "" && h.Config.Issuer == "" {
		a.driveRoot = filepath.Join(filepath.Dir(h.Config.Data), "Files")
	}
	var schema int
	if e := h.DB.QueryRow("SELECT coalesce(max(version),0) FROM host_migrations").Scan(&schema); e != nil {
		return nil, e
	}
	if schema > 4 {
		return nil, errors.New("this media data needs a newer My Cinema release")
	}
	if schema < 1 {
		if e := h.Migrate(1, `CREATE TABLE profiles(id TEXT PRIMARY KEY,owner TEXT NOT NULL,name TEXT NOT NULL,kind TEXT NOT NULL,pin BLOB NOT NULL,rating INTEGER NOT NULL,avatar TEXT NOT NULL,prefs TEXT NOT NULL DEFAULT '{}');
 CREATE INDEX profile_owner ON profiles(owner);
 CREATE TABLE active_profiles(session TEXT PRIMARY KEY,profile TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,unlocked TEXT NOT NULL);
 CREATE TABLE libraries(id TEXT PRIMARY KEY,name TEXT NOT NULL,kind TEXT NOT NULL,path TEXT NOT NULL UNIQUE);
 CREATE TABLE cinema_settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
 CREATE TABLE searches(profile TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,query TEXT NOT NULL,at TEXT NOT NULL,PRIMARY KEY(profile,query));`); e != nil {
			return nil, e
		}
	}
	if schema < 2 {
		if e := h.Migrate(2, `CREATE TABLE saved_titles(profile TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,item TEXT NOT NULL,at TEXT NOT NULL,PRIMARY KEY(profile,item));`); e != nil {
			return nil, e
		}
	}
	if schema < 3 {
		if e := h.Migrate(3, `CREATE TABLE media_uploads(id TEXT PRIMARY KEY,library TEXT NOT NULL,session TEXT NOT NULL,profile TEXT NOT NULL,name TEXT NOT NULL,size INTEGER NOT NULL,offset INTEGER NOT NULL DEFAULT 0,updated INTEGER NOT NULL);`); e != nil {
			return nil, e
		}
	}
	if schema < 4 {
		if e := h.Migrate(4, `ALTER TABLE media_uploads ADD COLUMN completed INTEGER NOT NULL DEFAULT 0;`); e != nil {
			return nil, e
		}
	}
	if b, e := os.ReadFile(filepath.Join(h.Config.Data, "bridge.dpapi")); e == nil {
		plain, e := unprotect(b)
		if e != nil {
			a.status = "This media connection belongs to another Windows account or PC. Restore on its original PC or re-link a fresh installation."
			return nil, errors.New(a.status)
		}
		if e = json.Unmarshal(plain, &a.secrets); e != nil {
			return nil, errors.New("The encrypted media connection could not be read")
		}
	}
	if a.secrets.Users == nil {
		a.secrets.Users = map[string]jellyfin.Credentials{}
	}
	routes := []struct {
		path string
		fn   http.HandlerFunc
	}{
		{"GET /api/profiles", a.profiles}, {"POST /api/profiles", a.createProfile}, {"POST /api/profiles/select", a.selectProfile}, {"POST /api/profiles/lock", a.lockProfile}, {"POST /api/profiles/{id}", a.editProfile},
		{"GET /api/status", a.health}, {"GET /api/home", a.home}, {"GET /api/catalog", a.catalog}, {"GET /api/search", a.search}, {"GET /api/my-list", a.myList}, {"GET /api/items/{id}", a.item}, {"GET /api/items/{id}/image/{kind}", a.image},
		{"GET /api/series/{id}/seasons", a.seasons}, {"GET /api/series/{id}/episodes", a.episodes}, {"POST /api/my-list/{id}", a.favourite}, {"DELETE /api/my-list/{id}", a.favourite}, {"POST /api/items/{id}/watched", a.watched},
		{"GET /api/items/{id}/similar", a.similar},
		{"POST /api/playback/sessions", a.startPlayback}, {"POST /api/playback/{id}/progress", a.progress}, {"POST /api/playback/{id}/stop", a.stopPlayback}, {"GET /api/playback/{id}/stream/{path...}", a.stream},
		{"GET /api/admin/settings", a.settings}, {"POST /api/admin/libraries", a.addLibrary}, {"DELETE /api/admin/libraries/{id}", a.removeLibrary}, {"POST /api/admin/scan", a.scan},
		{"GET /api/admin/folders", a.folders}, {"PUT /api/admin/libraries/{id}/folder", a.changeLibraryFolder},
		{"POST /api/admin/uploads", a.createUpload}, {"GET /api/admin/uploads/{id}", a.uploadStatus}, {"PUT /api/admin/uploads/{id}", a.uploadChunk}, {"POST /api/admin/uploads/{id}/complete", a.completeUpload}, {"DELETE /api/admin/uploads/{id}", a.cancelUpload},
		{"POST /api/profile/preferences", a.preferences}, {"DELETE /api/profile/history", a.clearHistory},
	}
	for _, r := range routes {
		h.Mux.HandleFunc(r.path, h.Require(r.fn))
	}
	h.Mux.HandleFunc("GET /cinema-health", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		ready := a.ready
		a.mu.Unlock()
		code := http.StatusOK
		if !ready {
			code = http.StatusServiceUnavailable
		}
		apphost.JSON(w, code, map[string]any{"ready": ready, "version": h.Config.Version})
	})
	return a, nil
}
func (a *App) saveSecretsLocked() error {
	raw, e := json.Marshal(a.secrets)
	if e != nil {
		return e
	}
	b, e := protect(raw)
	if e != nil {
		return e
	}
	path := filepath.Join(a.H.Config.Data, "bridge.dpapi")
	tmp := path + ".new"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func (a *App) health(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	apphost.JSON(w, 200, map[string]any{"ready": a.ready, "message": a.status, "jellyfinVersion": jellyfin.Version, "schema": 1})
}
func (a *App) media(w http.ResponseWriter) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.ready {
		apphost.Problem(w, 503, a.status)
		return false
	}
	return true
}
func (a *App) fail(w http.ResponseWriter, e error) {
	a.H.Log.Error("media operation failed", "kind", e.Error())
	apphost.Problem(w, 502, "The media request could not finish. Retry, or check the media engine in Settings.")
}
func (a *App) admin(w http.ResponseWriter, r *http.Request) bool {
	if apphost.Current(r).User.Role != "owner" {
		apphost.Problem(w, 403, "Only the Workspace owner can manage media folders")
		return false
	}
	p, ok := a.active(w, r)
	if !ok {
		return false
	}
	if p.Kind != "adult" {
		apphost.Problem(w, 403, "Unlock an adult profile first")
		return false
	}
	return true
}
func (a *App) Run(ctx context.Context) error { return a.runEngine(ctx) }
