package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
)

func (a *App) drive() (string, error) {
	if a.driveRoot == "" {
		return "", errors.New("My Drive is not connected to this Cinema installation")
	}
	root, e := validateRoot(a.driveRoot)
	if e != nil {
		return "", errors.New("My Drive is unavailable. Reconnect its disk and retry")
	}
	if inside(root, a.H.Config.Data) || inside(a.H.Config.Data, root) {
		return "", errors.New("My Drive must be separate from private Cinema data")
	}
	return root, nil
}

func (a *App) driveFolder(folder string) (string, error) {
	root, e := a.drive()
	if e != nil {
		return "", e
	}
	if folder == "" {
		return root, nil
	}
	if strings.ContainsAny(folder, "\\:\x00\r\n") || strings.HasPrefix(folder, "/") || path.Clean(folder) != folder || !filepath.IsLocal(filepath.FromSlash(folder)) || folder == "." {
		return "", errors.New("Choose a folder inside My Drive")
	}
	candidate := filepath.Join(root, filepath.FromSlash(folder))
	if !inside(root, candidate) {
		return "", errors.New("Choose a folder inside My Drive")
	}
	return validateRoot(candidate)
}

func (a *App) folderLabel(absolute string) string {
	root, e := a.drive()
	if e == nil && inside(root, absolute) {
		rel, e := filepath.Rel(root, absolute)
		if e == nil {
			if rel == "." {
				return "My Drive"
			}
			return "My Drive / " + strings.ReplaceAll(filepath.ToSlash(rel), "/", " / ")
		}
	}
	return absolute // Existing external libraries stay connected and visible to their owner.
}

func (a *App) folders(w http.ResponseWriter, r *http.Request) {
	if !a.admin(w, r) {
		return
	}
	folder := r.URL.Query().Get("folder")
	absolute, e := a.driveFolder(folder)
	if e != nil {
		apphost.Problem(w, 400, e.Error())
		return
	}
	dir, e := os.OpenRoot(absolute)
	if e != nil {
		apphost.Problem(w, 400, "This Drive folder is unavailable")
		return
	}
	defer dir.Close()
	f, e := dir.Open(".")
	if e != nil {
		apphost.Problem(w, 400, "This Drive folder cannot be read")
		return
	}
	defer f.Close()
	type entry struct {
		Name   string `json:"name"`
		Folder string `json:"folder"`
	}
	out := []entry{}
	truncated := false
	for scanned := 0; ; {
		entries, err := f.ReadDir(256)
		for _, item := range entries {
			scanned++
			if !item.IsDir() || strings.HasPrefix(item.Name(), ".") || rejectLink(filepath.Join(absolute, item.Name())) {
				continue
			}
			child := path.Join(folder, item.Name())
			if _, e = a.driveFolder(child); e == nil {
				out = append(out, entry{item.Name(), child})
			}
		}
		if err != nil && err != io.EOF {
			apphost.Problem(w, 400, "This Drive folder could not be listed")
			return
		}
		if err == io.EOF {
			break
		}
		if len(out) >= 1000 || scanned >= 10000 {
			truncated = true
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	parent := ""
	if folder != "" && path.Dir(folder) != "." {
		parent = path.Dir(folder)
	}
	apphost.JSON(w, 200, map[string]any{"folder": folder, "label": a.folderLabel(absolute), "parent": parent, "canGoUp": folder != "", "folders": out, "empty": len(out) == 0, "truncated": truncated, "canSelect": folder != ""})
}

// Each destination is provisioned once. The marker is per type, so retries are
// idempotent and a later owner-selected location survives every restart.
func (a *App) ensureDefaultLibraries(ctx context.Context) error {
	a.libraryMu.Lock()
	defer a.libraryMu.Unlock()
	if a.H.Config.Issuer == "" && a.driveRoot != "" {
		if e := os.MkdirAll(a.driveRoot, 0700); e != nil {
			return e
		}
	}
	root, e := a.drive()
	if e != nil {
		return e
	}
	dir, e := os.OpenRoot(root)
	if e != nil {
		return e
	}
	defer dir.Close()
	if e = dir.Mkdir("Cinema", 0700); e != nil && !os.IsExist(e) {
		return e
	}
	if _, e = a.driveFolder("Cinema"); e != nil {
		return e
	}
	for _, def := range []struct{ name, kind string }{{"Movies", "movies"}, {"TV Shows", "tvshows"}, {"Home Videos", "homevideos"}} {
		key := "drive-default-" + def.kind
		var done string
		if e = a.H.DB.QueryRow("SELECT value FROM cinema_settings WHERE key=?", key).Scan(&done); e == nil {
			continue
		} else if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		folder := "Cinema/" + def.name
		if e = dir.Mkdir(filepath.FromSlash(folder), 0700); e != nil && !os.IsExist(e) {
			return e
		}
		absolute, err := a.driveFolder(folder)
		if err != nil {
			return err
		}
		var id string
		if err = a.H.DB.QueryRow("SELECT id FROM libraries WHERE path=? COLLATE NOCASE", absolute).Scan(&id); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			name := def.name
			for n := 1; ; n++ {
				var count int
				if err = a.H.DB.QueryRow("SELECT count(*) FROM libraries WHERE name=? COLLATE NOCASE", name).Scan(&count); err != nil {
					return err
				}
				if count == 0 {
					break
				}
				name = fmt.Sprintf("%s (Cinema %d)", def.name, n)
			}
			id, err = a.connectLibrary(ctx, name, def.kind, absolute, false)
			if err != nil {
				return err
			}
		}
		if _, e = a.H.DB.Exec("INSERT INTO cinema_settings(key,value)VALUES(?,?) ON CONFLICT(key)DO UPDATE SET value=excluded.value", key, id); e != nil {
			return e
		}
	}
	return nil
}

func (a *App) connectLibrary(ctx context.Context, name, kind, absolute string, metadata bool) (string, error) {
	// Reconcile an interrupted create without deleting or duplicating engine data.
	var existing []struct {
		Name      string
		Locations []string
	}
	if e := a.engine.Do(ctx, "GET", "/Library/VirtualFolders", a.secrets.Admin.Token, nil, &existing); e != nil {
		return "", e
	}
	found := false
	for _, item := range existing {
		if !strings.EqualFold(item.Name, name) {
			continue
		}
		for _, p := range item.Locations {
			if strings.EqualFold(p, absolute) {
				found = true
			}
		}
		if !found {
			return "", errors.New("a different media engine library already uses that name")
		}
	}
	if !found {
		if e := a.engine.Do(ctx, "POST", "/Library/VirtualFolders?name="+urlQuery(name)+"&collectionType="+kind+"&refreshLibrary=true", a.secrets.Admin.Token, map[string]any{"LibraryOptions": map[string]any{"PathInfos": []map[string]string{{"Path": absolute}}, "EnableRealtimeMonitor": true, "EnableAutomaticSeriesGrouping": true, "EnableInternetProviders": metadata}}, nil); e != nil {
			return "", e
		}
	}
	id := apphost.ID()
	_, e := a.H.DB.Exec("INSERT INTO libraries VALUES(?,?,?,?)", id, name, kind, absolute)
	return id, e
}

func (a *App) changeLibraryFolder(w http.ResponseWriter, r *http.Request) {
	if !a.admin(w, r) || !a.media(w) {
		return
	}
	var in struct{ Folder string }
	if !apphost.Decode(w, r, &in) {
		return
	}
	if in.Folder == "" {
		apphost.Problem(w, 400, "Choose a media folder inside My Drive")
		return
	}
	absolute, e := a.driveFolder(in.Folder)
	if e != nil {
		apphost.Problem(w, 400, e.Error())
		return
	}
	a.libraryMu.Lock()
	defer a.libraryMu.Unlock()
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()
	a.expireUploadsLocked()
	var name, old string
	if e = a.H.DB.QueryRow("SELECT name,path FROM libraries WHERE id=?", r.PathValue("id")).Scan(&name, &old); e != nil {
		apphost.Problem(w, 404, "Library not found")
		return
	}
	var busy, conflict int
	if e = a.H.DB.QueryRow("SELECT count(*) FROM media_uploads WHERE library=? AND completed=0", r.PathValue("id")).Scan(&busy); e != nil {
		a.H.DBError(w, e)
		return
	}
	if busy > 0 {
		apphost.Problem(w, 409, "Finish or cancel uploads before changing this folder")
		return
	}
	if e = a.H.DB.QueryRow("SELECT count(*) FROM libraries WHERE path=? COLLATE NOCASE AND id<>?", absolute, r.PathValue("id")).Scan(&conflict); e != nil {
		a.H.DBError(w, e)
		return
	}
	if conflict > 0 {
		apphost.Problem(w, 409, "This folder is already connected to another library")
		return
	}
	if !strings.EqualFold(old, absolute) {
		add := func(p string) error {
			return a.engine.Do(r.Context(), "POST", "/Library/VirtualFolders/Paths?refreshLibrary=false", a.secrets.Admin.Token, map[string]any{"Name": name, "PathInfo": map[string]string{"Path": p}}, nil)
		}
		remove := func(p string) error {
			return a.engine.Do(r.Context(), "DELETE", "/Library/VirtualFolders/Paths?name="+urlQuery(name)+"&path="+urlQuery(p)+"&refreshLibrary=false", a.secrets.Admin.Token, nil, nil)
		}
		if e = add(absolute); e != nil {
			a.fail(w, e)
			return
		}
		if e = remove(old); e != nil {
			_ = remove(absolute)
			a.fail(w, e)
			return
		}
		if _, e = a.H.DB.Exec("UPDATE libraries SET path=? WHERE id=?", absolute, r.PathValue("id")); e != nil {
			_ = add(old)
			_ = remove(absolute)
			a.H.DBError(w, e)
			return
		}
	}
	scanning := a.engine.Do(r.Context(), "POST", "/Library/Refresh", a.secrets.Admin.Token, nil, nil) == nil
	a.H.Audit(apphost.Current(r).User.ID, "library-folder-changed", r.PathValue("id"))
	apphost.JSON(w, 200, map[string]any{"saved": true, "filesRetained": true, "scanning": scanning})
}
