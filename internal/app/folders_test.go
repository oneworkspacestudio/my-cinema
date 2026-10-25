package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/oneworkspacestudio/my-cinema/internal/jellyfin"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"golang.org/x/crypto/bcrypt"
)

func TestDriveDefaultsPickerChangesAndOneAccount(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows distribution")
	}
	root := t.TempDir()
	files := filepath.Join(root, "Drive")
	if e := os.Mkdir(files, 0700); e != nil {
		t.Fatal(e)
	}
	h, e := apphost.New(apphost.Config{ID: "cinema", Name: "Synthetic", Version: "test", Data: filepath.Join(root, "Private"), Listen: "127.0.0.1:0", Origin: "http://localhost:29321"}, fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("synthetic")}})
	if e != nil {
		t.Fatal(e)
	}
	defer h.Close()
	if e = h.Bootstrap("fixture@example.test", "synthetic-test-password"); e != nil {
		t.Fatal(e)
	}
	a, e := New(h)
	if e != nil {
		t.Fatal(e)
	}
	a.driveRoot = files
	virtual := map[string][]string{}
	failRemove := false
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/Library/VirtualFolders" {
			out := []map[string]any{}
			for name, paths := range virtual {
				out = append(out, map[string]any{"Name": name, "Locations": paths})
			}
			apphost.JSON(w, 200, out)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/Library/VirtualFolders" {
			var in struct {
				LibraryOptions struct{ PathInfos []struct{ Path string } }
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			virtual[r.URL.Query().Get("name")] = []string{in.LibraryOptions.PathInfos[0].Path}
		}
		if r.URL.Path == "/Library/VirtualFolders/Paths" {
			if r.Method == "POST" {
				var in struct {
					Name     string
					PathInfo struct{ Path string }
				}
				_ = json.NewDecoder(r.Body).Decode(&in)
				virtual[in.Name] = append(virtual[in.Name], in.PathInfo.Path)
			}
			if r.Method == "DELETE" {
				if failRemove {
					failRemove = false
					w.WriteHeader(500)
					return
				}
				name, path := r.URL.Query().Get("name"), r.URL.Query().Get("path")
				keep := []string{}
				for _, p := range virtual[name] {
					if p != path {
						keep = append(keep, p)
					}
				}
				virtual[name] = keep
			}
		}
		if r.URL.Path == "/Users/New" {
			apphost.JSON(w, 200, map[string]any{"Id": "abcdef", "Policy": map[string]any{}})
			return
		}
		if r.URL.Path == "/Users/AuthenticateByName" {
			apphost.JSON(w, 200, map[string]any{"AccessToken": "synthetic-only", "User": map[string]string{"Id": "abcdef"}})
			return
		}
		w.WriteHeader(204)
	}))
	defer engine.Close()
	a.engine = jellyfin.New(engine.URL)
	a.ready = true
	var cookie *http.Cookie
	csrf := ""
	call := func(method, path string, body any, auth, token bool) *httptest.ResponseRecorder {
		var b []byte
		if body != nil {
			b, _ = json.Marshal(body)
		}
		r := httptest.NewRequest(method, "http://localhost:29321"+path, bytes.NewReader(b))
		r.RemoteAddr = "127.0.0.1:12345"
		r.Header.Set("Origin", "http://localhost:29321")
		r.Header.Set("Content-Type", "application/json")
		if auth && cookie != nil {
			r.AddCookie(cookie)
		}
		if token {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		h.Handler().ServeHTTP(w, r)
		return w
	}
	login := call("POST", "/api/login", map[string]string{"email": "fixture@example.test", "password": "synthetic-test-password"}, false, false)
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body.String())
	}
	cookie = login.Result().Cookies()[0]
	var session apphost.Session
	_ = json.Unmarshal(call("GET", "/api/me", nil, true, false).Body.Bytes(), &session)
	csrf = session.CSRF
	pin, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.MinCost)
	if _, e = h.DB.Exec("INSERT INTO profiles(id,owner,name,kind,pin,rating,avatar)VALUES('adult',?,'Synthetic adult','adult',?,18,'blue')", session.User.ID, pin); e != nil {
		t.Fatal(e)
	}
	selectProfile := func(pin string) *httptest.ResponseRecorder {
		return call("POST", "/api/profiles/select", map[string]string{"id": "adult", "pin": pin}, true, true)
	}
	if w := selectProfile(""); w.Code != 403 {
		t.Fatal("local PIN bypass", w.Code)
	}
	if w := selectProfile("123456"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("POST", "/api/profiles", map[string]any{"name": "No local PIN", "kind": "adult", "rating": 18}, true, true); w.Code != 400 {
		t.Fatal("local adult without PIN", w.Code)
	}
	h.Config.Issuer = "https://accounts.example.test"
	h.Config.Owner = session.User.Subject
	if w := selectProfile(""); w.Code != 401 {
		t.Fatal("local session claimed OneAccount", w.Code)
	}
	// Synthetic authoritative session data models a successfully verified OIDC
	// login. Browser bodies/headers never control this issuer value.
	if _, e = h.DB.Exec("UPDATE host_users SET issuer=? WHERE id=?", h.Config.Issuer, session.User.ID); e != nil {
		t.Fatal(e)
	}
	if w := selectProfile(""); w.Code != 200 || strings.Contains(w.Body.String(), `"protected":true`) {
		t.Fatal("OneAccount still needs PIN", w.Code, w.Body.String())
	}
	if w := call("GET", "/api/profiles", nil, true, false); w.Code != 200 || !strings.Contains(w.Body.String(), `"pinRequired":false`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, e = h.DB.Exec("UPDATE active_profiles SET unlocked=?", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)); e != nil {
		t.Fatal(e)
	}
	if e = a.ensureDefaultLibraries(context.Background()); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"Movies", "TV Shows", "Home Videos"} {
		if st, e := os.Stat(filepath.Join(files, "Cinema", name)); e != nil || !st.IsDir() {
			t.Fatal(name, e)
		}
	}
	if len(virtual) != 3 {
		t.Fatal("defaults not registered", virtual)
	}
	if w := call("GET", "/api/admin/folders?folder=Cinema", nil, true, false); w.Code != 200 || !strings.Contains(w.Body.String(), "TV Shows") {
		t.Fatal("picker or OneAccount expiry", w.Code, w.Body.String())
	}
	if w := call("GET", "/api/admin/folders", nil, false, false); w.Code != 401 {
		t.Fatal("anonymous folders", w.Code)
	}
	for _, folder := range []string{"../Private", "/Cinema", "C:/Windows", `Cinema\Movies`, "Cinema/../Movies", "."} {
		if _, e = a.driveFolder(folder); e == nil {
			t.Fatal("picker escaped", folder)
		}
	}
	outside := filepath.Join(root, "Outside")
	_ = os.Mkdir(outside, 0700)
	junction := filepath.Join(files, "Escape")
	if output, e := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, outside).CombinedOutput(); e != nil {
		t.Fatal("synthetic junction", e, string(output))
	}
	if _, e = a.driveFolder("Escape"); e == nil {
		t.Fatal("junction accepted")
	}
	if w := call("GET", "/api/admin/folders", nil, true, false); strings.Contains(w.Body.String(), "Escape") {
		t.Fatal("junction visible", w.Body.String())
	}
	var library string
	if e = h.DB.QueryRow("SELECT id FROM libraries WHERE kind='movies'").Scan(&library); e != nil {
		t.Fatal(e)
	}
	old := filepath.Join(files, "Cinema", "Movies")
	_ = os.WriteFile(filepath.Join(old, "original.mp4"), []byte("existing original"), 0600)
	custom := filepath.Join(files, "Chosen movies")
	_ = os.Mkdir(custom, 0700)
	endpoint := "/api/admin/libraries/" + library + "/folder"
	body := map[string]string{"folder": "Chosen movies"}
	if w := call("PUT", endpoint, body, true, false); w.Code != 403 {
		t.Fatal("folder CSRF bypass", w.Code)
	}
	_, e = h.DB.Exec("INSERT INTO media_uploads VALUES('busy',?,'fixture','adult','Movie.mp4',32,0,?,0)", library, time.Now().Unix())
	if e != nil {
		t.Fatal(e)
	}
	if w := call("PUT", endpoint, body, true, true); w.Code != 409 {
		t.Fatal("changed active upload", w.Code)
	}
	_, _ = h.DB.Exec("DELETE FROM media_uploads WHERE id='busy'")
	if w := call("PUT", endpoint, body, true, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = a.ensureDefaultLibraries(context.Background()); e != nil {
		t.Fatal(e)
	}
	var current string
	_ = h.DB.QueryRow("SELECT path FROM libraries WHERE id=?", library).Scan(&current)
	if current != custom || len(virtual) != 3 || len(virtual["Movies"]) != 1 || virtual["Movies"][0] != custom {
		t.Fatal("owner choice lost on restart", current, virtual)
	}
	if b, e := os.ReadFile(filepath.Join(old, "original.mp4")); e != nil || string(b) != "existing original" {
		t.Fatal("original changed", e)
	}
	_ = os.Mkdir(filepath.Join(files, "Retry folder"), 0700)
	failRemove = true
	if w := call("PUT", endpoint, map[string]string{"folder": "Retry folder"}, true, true); w.Code != 502 {
		t.Fatal("engine failure ignored", w.Code)
	}
	_ = h.DB.QueryRow("SELECT path FROM libraries WHERE id=?", library).Scan(&current)
	if current != custom || len(virtual["Movies"]) != 1 || virtual["Movies"][0] != custom {
		t.Fatal("failed edit not compensated", current, virtual)
	}
	if w := call("POST", "/api/profiles", map[string]any{"name": "OneAccount adult", "kind": "adult", "rating": 18}, true, true); w.Code != 201 {
		t.Fatal("OneAccount creation requires PIN", w.Code, w.Body.String())
	}
	_, _ = h.DB.Exec("UPDATE profiles SET kind='child' WHERE id='adult'")
	if w := call("GET", "/api/admin/folders", nil, true, false); w.Code != 403 {
		t.Fatal("child browsed owner Drive", w.Code)
	}
	_, _ = h.DB.Exec("UPDATE profiles SET kind='adult' WHERE id='adult'")
	h.Config.Owner = "another-synthetic-owner"
	if w := call("GET", "/api/admin/folders", nil, true, false); w.Code != 403 {
		t.Fatal("member browsed owner Drive", w.Code)
	}
	_, _ = h.DB.Exec("UPDATE host_sessions SET expires=?", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano))
	if w := call("POST", "/api/profiles/select", map[string]string{"id": "adult"}, true, true); w.Code != 401 {
		t.Fatal("expired OneAccount session", w.Code)
	}
}
