package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
)

const uploadChunkSize int64 = 8 << 20
const maxUploadSize int64 = 100 << 30

type mediaUpload struct {
	ID, Library, Session, Profile, Name string
	Size, Offset, Updated               int64
	Completed                           bool
}

// A completed video gets its own title folder. A staging directory has no media
// extension and is never exposed through playback. Publishing a directory on
// Windows fails if the destination exists, preserving every existing title.
func uploadNames(name string) (string, string, error) {
	if len(name) == 0 || len(name) > 180 || strings.TrimSpace(name) != name || strings.ContainsAny(name, `<>:"/\|?*`) || strings.HasSuffix(name, ".") || !isMedia(name) {
		return "", "", errors.New("Choose an MP4, MKV, WebM, MOV, AVI, M4V or TS video with a plain filename")
	}
	for _, c := range name {
		if unicode.IsControl(c) {
			return "", "", errors.New("The video filename contains an unsupported character")
		}
	}
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if stem == "" || strings.HasPrefix(stem, ".") || strings.HasSuffix(stem, " ") || strings.HasSuffix(stem, ".") {
		return "", "", errors.New("Choose a video with a title in its filename")
	}
	device := strings.ToUpper(strings.Split(stem, ".")[0])
	if device == "CON" || device == "PRN" || device == "AUX" || device == "NUL" || device == "CLOCK$" || (len(device) == 4 && (strings.HasPrefix(device, "COM") || strings.HasPrefix(device, "LPT")) && device[3] >= '0' && device[3] <= '9') {
		return "", "", errors.New("Rename this video before uploading it")
	}
	return name, stem, nil
}

func stageName(id string) string { return ".cinema-upload-" + id }

func (a *App) uploadRoot(library string) (*os.Root, error) {
	var path string
	if e := a.H.DB.QueryRow("SELECT path FROM libraries WHERE id=?", library).Scan(&path); e != nil {
		return nil, errors.New("Choose a connected media folder")
	}
	path, e := validateRoot(path)
	if e != nil {
		return nil, e
	}
	return os.OpenRoot(path)
}

func (a *App) createUpload(w http.ResponseWriter, r *http.Request) {
	if !a.admin(w, r) || !a.media(w) {
		return
	}
	var in struct {
		Library, Name string
		Size          int64
	}
	if !apphost.Decode(w, r, &in) {
		return
	}
	name, title, e := uploadNames(in.Name)
	if e != nil {
		apphost.Problem(w, 400, e.Error())
		return
	}
	if in.Size < 12 || in.Size > maxUploadSize {
		apphost.Problem(w, 400, "Choose a video between 12 bytes and 100 GB")
		return
	}
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()
	a.expireUploadsLocked()
	var count int
	if e = a.H.DB.QueryRow("SELECT count(*) FROM media_uploads WHERE completed=0").Scan(&count); e != nil {
		a.H.DBError(w, e)
		return
	}
	if count >= 8 {
		apphost.Problem(w, 409, "Finish or cancel the other uploads before adding more videos")
		return
	}
	root, e := a.uploadRoot(in.Library)
	if e != nil {
		apphost.Problem(w, 400, e.Error())
		return
	}
	defer root.Close()
	for _, target := range []string{name, title} {
		if _, e = root.Lstat(target); !os.IsNotExist(e) {
			apphost.Problem(w, 409, "A video or folder with this name already exists. Rename your upload; existing videos are never replaced.")
			return
		}
	}
	id := apphost.ID()
	stage := stageName(id)
	if e = root.Mkdir(stage, 0700); e != nil {
		apphost.Problem(w, 400, "This media folder is not writable. Choose another folder or check its permissions.")
		return
	}
	f, e := root.OpenFile(filepath.Join(stage, "video.part"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		_ = root.Remove(stage)
		apphost.Problem(w, 400, "The upload could not be started in this folder")
		return
	}
	f.Close()
	var profile string
	e = a.H.DB.QueryRow("SELECT profile FROM active_profiles WHERE session=?", sessionKey(r)).Scan(&profile)
	if e == nil {
		_, e = a.H.DB.Exec("INSERT INTO media_uploads(id,library,session,profile,name,size,offset,updated) VALUES(?,?,?,?,?,?,0,?)", id, in.Library, sessionKey(r), profile, name, in.Size, time.Now().Unix())
	}
	if e != nil {
		_ = root.RemoveAll(stage)
		a.H.DBError(w, e)
		return
	}
	apphost.JSON(w, 201, map[string]any{"id": id, "offset": 0, "chunkSize": uploadChunkSize})
}

func (a *App) findUpload(w http.ResponseWriter, r *http.Request) (mediaUpload, bool) {
	var u mediaUpload
	id := r.PathValue("id")
	if !validID.MatchString(id) {
		apphost.Problem(w, 404, "Upload not found")
		return u, false
	}
	e := a.H.DB.QueryRow("SELECT id,library,session,profile,name,size,offset,updated,completed FROM media_uploads WHERE id=? AND session=?", id, sessionKey(r)).Scan(&u.ID, &u.Library, &u.Session, &u.Profile, &u.Name, &u.Size, &u.Offset, &u.Updated, &u.Completed)
	var profile string
	_ = a.H.DB.QueryRow("SELECT profile FROM active_profiles WHERE session=?", sessionKey(r)).Scan(&profile)
	if e != nil || profile != u.Profile {
		apphost.Problem(w, 404, "Upload not found in this viewing profile")
		return u, false
	}
	return u, true
}

func (a *App) uploadStatus(w http.ResponseWriter, r *http.Request) {
	if !a.admin(w, r) {
		return
	}
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()
	if u, ok := a.findUpload(w, r); ok {
		apphost.JSON(w, 200, map[string]any{"offset": u.Offset, "size": u.Size, "completed": u.Completed, "name": u.Name})
	}
}

func (a *App) uploadChunk(w http.ResponseWriter, r *http.Request) {
	if !a.admin(w, r) {
		return
	}
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()
	u, ok := a.findUpload(w, r)
	if !ok {
		return
	}
	offset, e := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	if u.Completed {
		apphost.Problem(w, 409, "This video is already saved")
		return
	}
	if e != nil || offset != u.Offset {
		apphost.JSON(w, 409, map[string]any{"error": "The upload position changed. Retry the current video.", "offset": u.Offset})
		return
	}
	if r.ContentLength <= 0 || r.ContentLength > uploadChunkSize || r.ContentLength > u.Size-u.Offset {
		apphost.Problem(w, 400, "Invalid video chunk size")
		return
	}
	root, e := a.uploadRoot(u.Library)
	if e != nil {
		apphost.Problem(w, 409, "The upload folder is no longer available")
		return
	}
	defer root.Close()
	f, e := root.OpenFile(filepath.Join(stageName(u.ID), "video.part"), os.O_WRONLY, 0600)
	if e != nil {
		apphost.Problem(w, 409, "The partial upload is unavailable. Cancel it and try again.")
		return
	}
	defer f.Close()
	if e = f.Truncate(u.Offset); e == nil {
		_, e = f.Seek(u.Offset, io.SeekStart)
	}
	if e != nil {
		apphost.Problem(w, 507, "The video could not be written. Check free space and folder permissions.")
		return
	}
	n, e := io.Copy(f, http.MaxBytesReader(w, r.Body, uploadChunkSize))
	if e == nil && n != r.ContentLength {
		e = io.ErrUnexpectedEOF
	}
	if e == nil {
		e = f.Sync()
	}
	if e != nil {
		_ = f.Truncate(u.Offset)
		apphost.Problem(w, 507, "The upload was interrupted or storage is full. Retry or cancel this video.")
		return
	}
	if _, e = a.H.DB.Exec("UPDATE media_uploads SET offset=?,updated=? WHERE id=?", u.Offset+n, time.Now().Unix(), u.ID); e != nil {
		a.H.DBError(w, e)
		return
	}
	apphost.JSON(w, 200, map[string]any{"offset": u.Offset + n})
}

func (a *App) completeUpload(w http.ResponseWriter, r *http.Request) {
	if !a.admin(w, r) || !a.media(w) {
		return
	}
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()
	u, ok := a.findUpload(w, r)
	if !ok {
		return
	}
	if u.Offset != u.Size {
		apphost.Problem(w, 409, "Wait for the video upload to finish")
		return
	}
	if u.Completed {
		apphost.JSON(w, 201, map[string]any{"uploaded": true, "name": u.Name, "scanning": true})
		return
	}
	root, e := a.uploadRoot(u.Library)
	if e != nil {
		apphost.Problem(w, 409, "The upload folder is no longer available")
		return
	}
	defer root.Close()
	name, title, e := uploadNames(u.Name)
	if e != nil {
		apphost.Problem(w, 400, e.Error())
		return
	}
	stage := stageName(u.ID)
	partial := filepath.Join(stage, "video.part")
	// A process stop between the two renames leaves the complete video within
	// staging. Recover that name before validating and publishing again.
	if _, e = root.Lstat(partial); os.IsNotExist(e) {
		_ = root.Rename(filepath.Join(stage, name), partial)
	}
	f, e := root.Open(partial)
	if e != nil {
		apphost.Problem(w, 409, "The partial upload is unavailable")
		return
	}
	st, e := f.Stat()
	header := make([]byte, 512)
	n, _ := f.Read(header)
	f.Close()
	if e != nil || !st.Mode().IsRegular() || st.Size() != u.Size {
		apphost.Problem(w, 409, "The uploaded size does not match the selected video")
		return
	}
	if !videoHeader(header[:n]) {
		apphost.Problem(w, 400, "This file is not a supported video container. Choose an MP4, MKV, WebM, MOV, AVI, M4V or TS video.")
		return
	}
	for _, target := range []string{name, title} {
		if _, e = root.Lstat(target); !os.IsNotExist(e) {
			apphost.Problem(w, 409, "A video or folder with this name already exists. Existing videos were kept.")
			return
		}
	}
	// Rename the part inside staging first. A retry after an interrupted final
	// publish can complete from that name without duplicating the video bytes.
	if e = root.Rename(partial, filepath.Join(stage, name)); e != nil {
		apphost.Problem(w, 409, "The video could not be prepared. Cancel and try again.")
		return
	}
	if e = publishUpload(root, stage, title); e != nil {
		_ = root.Rename(filepath.Join(stage, name), partial)
		apphost.Problem(w, 409, "The video could not be added. A title folder may already exist; existing files were kept.")
		return
	}
	_, e = a.H.DB.Exec("UPDATE media_uploads SET completed=1,updated=? WHERE id=?", time.Now().Unix(), u.ID)
	if e != nil {
		a.H.Log.Error("uploaded media receipt cleanup failed", "upload", u.ID)
	}
	a.H.Audit(apphost.Current(r).User.ID, "video-uploaded", u.ID)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	e = a.engine.Do(ctx, "POST", "/Library/Refresh", a.secrets.Admin.Token, nil, nil)
	apphost.JSON(w, 201, map[string]any{"uploaded": true, "name": name, "scanning": e == nil})
}

func videoHeader(b []byte) bool {
	if len(b) < 12 {
		return false
	}
	atom := string(b[4:8])
	return atom == "ftyp" || atom == "moov" || atom == "mdat" || atom == "wide" || atom == "free" ||
		(string(b[:4]) == "\x1a\x45\xdf\xa3") || (string(b[:4]) == "RIFF" && string(b[8:12]) == "AVI ") ||
		(len(b) > 376 && b[0] == 0x47 && b[188] == 0x47 && b[376] == 0x47)
}

func (a *App) cancelUpload(w http.ResponseWriter, r *http.Request) {
	if !a.admin(w, r) {
		return
	}
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()
	u, ok := a.findUpload(w, r)
	if !ok {
		return
	}
	if e := a.removeUploadLocked(u); e != nil {
		apphost.Problem(w, 409, "The unfinished upload could not be removed. Reconnect its media folder and retry.")
		return
	}
	apphost.JSON(w, 200, map[string]bool{"cancelled": true})
}

func (a *App) removeUploadLocked(u mediaUpload) error {
	if u.Completed {
		_, e := a.H.DB.Exec("DELETE FROM media_uploads WHERE id=?", u.ID)
		return e
	}
	root, e := a.uploadRoot(u.Library)
	if e != nil {
		return e
	}
	defer root.Close()
	if e = root.RemoveAll(stageName(u.ID)); e != nil {
		return e
	}
	_, e = a.H.DB.Exec("DELETE FROM media_uploads WHERE id=?", u.ID)
	return e
}

func (a *App) expireUploadsLocked() {
	rows, e := a.H.DB.Query("SELECT id,library,completed FROM media_uploads WHERE updated<?", time.Now().Add(-24*time.Hour).Unix())
	if e != nil {
		return
	}
	var expired []mediaUpload
	for rows.Next() {
		var u mediaUpload
		if rows.Scan(&u.ID, &u.Library, &u.Completed) == nil && validID.MatchString(u.ID) {
			expired = append(expired, u)
		}
	}
	rows.Close()
	for _, u := range expired {
		_ = a.removeUploadLocked(u)
	}
}
