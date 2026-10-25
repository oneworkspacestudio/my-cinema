package app

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/oneworkspacestudio/my-cinema/internal/jellyfin"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
)

func TestUploadFilenameBoundary(t *testing.T) {
	for _, name := range []string{"../movie.mp4", `..\movie.mp4`, "a:stream.mp4", "CON.mp4", "LPT1.mkv", "NUL.extra.mp4", ".mp4", "film.exe", "film.mp4.exe", "Film .mp4", "Film\n.mp4", "Film.mp4.", "Film .mov"} {
		if _, _, e := uploadNames(name); e == nil {
			t.Errorf("accepted %q", name)
		}
	}
	for _, name := range []string{"First Light.mp4", "日本の映像.MKV", "My film (2026).webm"} {
		if _, _, e := uploadNames(name); e != nil {
			t.Errorf("rejected %q: %v", name, e)
		}
	}
	if videoHeader([]byte("MZthis is an executable renamed as a video")) {
		t.Fatal("executable accepted")
	}
}

func TestUploadTransactionAndBoundaries(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows distribution")
	}
	root := t.TempDir()
	data := filepath.Join(root, "private")
	media := filepath.Join(root, "media")
	if e := os.Mkdir(media, 0700); e != nil {
		t.Fatal(e)
	}
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("synthetic"), Mode: fs.FileMode(0600)}}
	h, e := apphost.New(apphost.Config{ID: "cinema", Name: "Cinema test", Version: "test", Data: data, Listen: "127.0.0.1:0", Origin: "http://localhost:29321"}, ui)
	if e != nil {
		t.Fatal(e)
	}
	defer h.Close()
	if e = h.Bootstrap("upload@example.test", "synthetic-test-password"); e != nil {
		t.Fatal(e)
	}
	a, e := New(h)
	if e != nil {
		t.Fatal(e)
	}
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/Library/VirtualFolders" {
			apphost.JSON(w, 200, []any{})
			return
		}
		w.WriteHeader(204)
	}))
	defer engine.Close()
	a.engine = jellyfin.New(engine.URL)
	a.ready = true
	handler := h.Handler()
	var cookie *http.Cookie
	csrf := ""
	call := func(method, path string, body []byte, authorized, withCSRF bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://localhost:29321"+path, bytes.NewReader(body))
		r.RemoteAddr = "127.0.0.1:12345"
		r.Header.Set("Origin", "http://localhost:29321")
		r.Header.Set("Content-Type", "application/json")
		if authorized && cookie != nil {
			r.AddCookie(cookie)
		}
		if withCSRF {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	login := call("POST", "/api/login", []byte(`{"email":"upload@example.test","password":"synthetic-test-password"}`), false, false)
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body.String())
	}
	cookie = login.Result().Cookies()[0]
	me := call("GET", "/api/me", nil, true, false)
	var session apphost.Session
	if e = json.Unmarshal(me.Body.Bytes(), &session); e != nil {
		t.Fatal(e)
	}
	csrf = session.CSRF
	if _, e = h.DB.Exec("INSERT INTO profiles(id,owner,name,kind,pin,rating,avatar)VALUES('adult',?,'Synthetic adult','adult',?,18,'blue')", session.User.ID, []byte("test-only-pin-hash")); e != nil {
		t.Fatal(e)
	}
	if _, e = h.DB.Exec("INSERT INTO active_profiles VALUES(?,?,?)", apphost.Hash(cookie.Value), "adult", time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)); e != nil {
		t.Fatal(e)
	}
	if _, e = h.DB.Exec("INSERT INTO libraries VALUES('library','Synthetic videos','homevideos',?)", media); e != nil {
		t.Fatal(e)
	}
	request := []byte(`{"library":"library","name":"Synthetic film.mp4","size":32}`)
	for _, tc := range []struct {
		auth, csrf bool
		want       int
	}{{false, false, 401}, {true, false, 403}} {
		if w := call("POST", "/api/admin/uploads", request, tc.auth, tc.csrf); w.Code != tc.want {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if _, e = h.DB.Exec("UPDATE profiles SET kind='child' WHERE id='adult'"); e != nil {
		t.Fatal(e)
	}
	if w := call("POST", "/api/admin/uploads", request, true, true); w.Code != 403 {
		t.Fatal("child upload", w.Code)
	}
	_, _ = h.DB.Exec("UPDATE profiles SET kind='adult' WHERE id='adult'")
	_, _ = h.DB.Exec("UPDATE host_users SET role='member' WHERE id=?", session.User.ID)
	if w := call("POST", "/api/admin/uploads", request, true, true); w.Code != 403 {
		t.Fatal("member upload", w.Code)
	}
	_, _ = h.DB.Exec("UPDATE host_users SET role='owner' WHERE id=?", session.User.ID)
	newFolder := filepath.Join(root, "new videos")
	newLibrary, _ := json.Marshal(map[string]any{"name": "New media folder", "path": newFolder, "kind": "movies", "create": true})
	if w := call("POST", "/api/admin/libraries", newLibrary, true, true); w.Code != 201 {
		t.Fatal("new folder", w.Code, w.Body.String())
	}
	if st, e := os.Stat(newFolder); e != nil || !st.IsDir() {
		t.Fatal("new folder missing", e)
	}
	badLibrary, _ := json.Marshal(map[string]any{"name": "Bad folder", "path": filepath.Join(data, "media"), "kind": "movies", "create": true})
	if w := call("POST", "/api/admin/libraries", badLibrary, true, true); w.Code != 400 {
		t.Fatal("private folder created", w.Code)
	}
	create := func() string {
		w := call("POST", "/api/admin/uploads", request, true, true)
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
		var v struct{ ID string }
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		return v.ID
	}
	id := create()
	path := "/api/admin/uploads/" + id
	if w := call("POST", path+"/complete", []byte(`{}`), true, true); w.Code != 409 {
		t.Fatal("incomplete publish", w.Code)
	}
	if w := call("DELETE", "/api/admin/libraries/library", nil, true, true); w.Code != 409 {
		t.Fatal("removed active library", w.Code)
	}
	video := append([]byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, bytes.Repeat([]byte{0}, 20)...)
	if w := call("PUT", path+"?offset=0", video[:16], true, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("PUT", path+"?offset=0", video[:16], true, true); w.Code != 409 {
		t.Fatal("duplicate chunk", w.Code)
	}
	if w := call("PUT", path+"?offset=16", video[16:], true, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, e = os.Stat(filepath.Join(media, "Synthetic film")); !os.IsNotExist(e) {
		t.Fatal("unfinished video exposed")
	}
	if w := call("POST", path+"/complete", []byte(`{}`), true, true); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	got, e := os.ReadFile(filepath.Join(media, "Synthetic film", "Synthetic film.mp4"))
	if w := call("POST", path+"/complete", []byte(`{}`), true, true); w.Code != 201 {
		t.Fatal("completion retry", w.Code, w.Body.String())
	}
	if w := call("DELETE", path, nil, true, true); w.Code != 200 {
		t.Fatal("completed receipt removal", w.Code)
	}
	if e != nil || !bytes.Equal(got, video) {
		t.Fatal("saved bytes", e)
	}
	if w := call("POST", "/api/admin/uploads", request, true, true); w.Code != 409 {
		t.Fatal("duplicate upload", w.Code)
	}
	// Publication must not replace an empty directory either, including when it
	// appears after the initial duplicate check.
	stage := "stage"
	_ = os.Mkdir(filepath.Join(media, stage), 0700)
	_ = os.Mkdir(filepath.Join(media, "existing"), 0700)
	oroot, e := os.OpenRoot(media)
	if e != nil {
		t.Fatal(e)
	}
	defer oroot.Close()
	if e = publishUpload(oroot, stage, "existing"); e == nil {
		t.Fatal("existing directory replaced")
	}
	request = []byte(`{"library":"library","name":"Cancelled.mp4","size":32}`)
	id = create()
	if w := call("DELETE", "/api/admin/uploads/"+id, nil, true, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, e = os.Stat(filepath.Join(media, stageName(id))); !os.IsNotExist(e) {
		t.Fatal("cancelled staging retained")
	}
	request = []byte(`{"library":"library","name":"Not a video.mp4","size":32}`)
	id = create()
	path = "/api/admin/uploads/" + id
	if w := call("PUT", path+"?offset=0", []byte(strings.Repeat("MZ", 16)), true, true); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := call("POST", path+"/complete", []byte(`{}`), true, true); w.Code != 400 {
		t.Fatal("invalid content published", w.Code)
	}
	if w := call("DELETE", path, nil, true, true); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if _, e = os.Stat(filepath.Join(media, "Not a video")); !os.IsNotExist(e) {
		t.Fatal("invalid media exposed")
	}
}
