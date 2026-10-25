package app

import (
	"encoding/json"
	"github.com/oneworkspacestudio/my-cinema/internal/jellyfin"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var validID = regexp.MustCompile(`^[a-fA-F0-9]{1,64}$`)
var validPIN = regexp.MustCompile(`^[0-9]{4,12}$`)

type Profile struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Kind      string          `json:"kind"`
	Rating    int             `json:"rating"`
	Avatar    string          `json:"avatar"`
	Prefs     json.RawMessage `json:"preferences"`
	Protected bool            `json:"protected"`
	owner     string
	pin       []byte
}

func sessionKey(r *http.Request) string {
	c, e := r.Cookie("cinema_session")
	if e != nil {
		return ""
	}
	return apphost.Hash(c.Value)
}
func urlQuery(s string) string { return url.QueryEscape(s) }

// Only a server-validated session from this installation's configured issuer
// replaces the local viewing PIN. Request bodies/headers cannot opt into it.
func (a *App) oneAccount(r *http.Request) bool {
	return a.H.Config.Issuer != "" && apphost.Current(r).User.Issuer == a.H.Config.Issuer
}
func (a *App) profile(id, owner string) (p Profile, e error) {
	var prefs string
	e = a.H.DB.QueryRow("SELECT id,owner,name,kind,pin,rating,avatar,prefs FROM profiles WHERE id=? AND owner=?", id, owner).Scan(&p.ID, &p.owner, &p.Name, &p.Kind, &p.pin, &p.Rating, &p.Avatar, &prefs)
	p.Prefs = json.RawMessage(prefs)
	p.Protected = len(p.pin) > 0
	return
}
func (a *App) profiles(w http.ResponseWriter, r *http.Request) {
	owner := apphost.Current(r).User.ID
	rows, e := a.H.DB.Query("SELECT id,name,kind,rating,avatar,prefs,length(pin)>0 FROM profiles WHERE owner=? ORDER BY rowid", owner)
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	out := []Profile{}
	for rows.Next() {
		var p Profile
		var prefs string
		if e = rows.Scan(&p.ID, &p.Name, &p.Kind, &p.Rating, &p.Avatar, &prefs, &p.Protected); e != nil {
			rows.Close()
			a.H.DBError(w, e)
			return
		}
		p.Prefs = json.RawMessage(prefs)
		if a.oneAccount(r) {
			p.Protected = false
		}
		out = append(out, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	var selected string
	_ = a.H.DB.QueryRow("SELECT profile FROM active_profiles WHERE session=?", sessionKey(r)).Scan(&selected)
	apphost.JSON(w, 200, map[string]any{"profiles": out, "active": selected, "maximum": 6, "pinRequired": !a.oneAccount(r)})
}
func (a *App) active(w http.ResponseWriter, r *http.Request) (Profile, bool) {
	var id, expiry string
	if a.H.DB.QueryRow("SELECT profile,unlocked FROM active_profiles WHERE session=?", sessionKey(r)).Scan(&id, &expiry) != nil {
		apphost.Problem(w, 409, "Choose who is watching first")
		return Profile{}, false
	}
	p, e := a.profile(id, apphost.Current(r).User.ID)
	if e != nil {
		apphost.Problem(w, 403, "This profile is no longer available")
		return p, false
	}
	if a.oneAccount(r) {
		p.Protected = false
	}
	w.Header().Set("X-Cinema-Profile", p.ID)
	if expected := r.Header.Get("X-Cinema-Profile"); expected != "" && expected != p.ID {
		apphost.Problem(w, 409, "The viewing profile changed. Choose who is watching again.")
		return p, false
	}
	// Adult PIN unlocks are bounded; selecting a child immediately revokes them.
	if p.Kind == "adult" && !a.oneAccount(r) {
		until, e := time.Parse(time.RFC3339Nano, expiry)
		if e != nil || time.Now().After(until) {
			_, _ = a.H.DB.Exec("DELETE FROM active_profiles WHERE session=?", sessionKey(r))
			apphost.Problem(w, 409, "Unlock your adult profile again")
			return p, false
		}
	}
	return p, true
}
func (a *App) credentials(id string) (jellyfin.Credentials, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	c, ok := a.secrets.Users[id]
	return c, ok
}
func (a *App) viewer(w http.ResponseWriter, r *http.Request) (Profile, jellyfin.Credentials, bool) {
	p, ok := a.active(w, r)
	if !ok || !a.media(w) {
		return p, jellyfin.Credentials{}, false
	}
	c, ok := a.credentials(p.ID)
	if !ok {
		apphost.Problem(w, 503, "This profile needs its media connection restored")
		return p, c, false
	}
	return p, c, true
}
func (a *App) createProfile(w http.ResponseWriter, r *http.Request) {
	if !a.media(w) {
		return
	}
	owner := apphost.Current(r).User.ID
	// Serialize profile creation, including the first-profile authorization check.
	a.mu.Lock()
	defer a.mu.Unlock()
	var count int
	if e := a.H.DB.QueryRow("SELECT count(*) FROM profiles WHERE owner=?", owner).Scan(&count); e != nil {
		a.H.DBError(w, e)
		return
	}
	if count >= 6 {
		apphost.Problem(w, 409, "This household already has six profiles")
		return
	}
	if count == 0 && apphost.Current(r).User.Role != "owner" {
		apphost.Problem(w, 403, "The Workspace owner must authorize a viewing household for this account")
		return
	}
	if count > 0 {
		p, ok := a.active(w, r)
		if !ok {
			return
		}
		if p.Kind != "adult" {
			apphost.Problem(w, 403, "Unlock an adult profile to create profiles")
			return
		}
	}
	var in struct {
		Name, Kind, PIN, Avatar string
		Rating                  int
	}
	if !apphost.Decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 1 || len(in.Name) > 40 || (in.Kind != "adult" && in.Kind != "child") || in.Rating < 0 || in.Rating > 18 {
		apphost.Problem(w, 400, "Choose a name, profile type and content limit")
		return
	}
	if count == 0 && in.Kind != "adult" {
		apphost.Problem(w, 400, "Create an adult profile first")
		return
	}
	if !a.oneAccount(r) && in.Kind == "adult" && !validPIN.MatchString(in.PIN) {
		apphost.Problem(w, 400, "Protect adult profiles with a 4–12 digit PIN")
		return
	}
	if in.Kind == "child" && in.Rating > 15 {
		apphost.Problem(w, 400, "Choose a child rating of 15 or below")
		return
	}
	if !regexp.MustCompile(`^(blue|violet|mint|sunset|rose|sky)$`).MatchString(in.Avatar) {
		in.Avatar = "blue"
	}
	var pin []byte
	var e error
	if a.oneAccount(r) {
		in.PIN = ""
	}
	if in.PIN != "" {
		if !validPIN.MatchString(in.PIN) {
			apphost.Problem(w, 400, "Use a 4–12 digit PIN")
			return
		}
		pin, e = bcrypt.GenerateFromPassword([]byte(in.PIN), 12)
		if e != nil {
			a.H.DBError(w, e)
			return
		}
	} else {
		pin = []byte{}
	}
	id := apphost.ID()
	name := "cinema-" + id
	password := apphost.ID() + apphost.ID()
	var user struct {
		ID     string `json:"Id"`
		Policy map[string]any
	}
	e = a.engine.Do(r.Context(), "POST", "/Users/New", a.secrets.Admin.Token, map[string]string{"Name": name, "Password": password}, &user)
	if e != nil {
		a.fail(w, e)
		return
	}
	clean := func() { _ = a.engine.Do(r.Context(), "DELETE", "/Users/"+user.ID, a.secrets.Admin.Token, nil, nil) }
	policy := user.Policy
	if policy == nil {
		clean()
		apphost.Problem(w, 502, "The media engine returned an incomplete user policy")
		return
	}
	for k, v := range profilePolicy(in.Kind, in.Rating) {
		policy[k] = v
	}
	if e = a.engine.Do(r.Context(), "POST", "/Users/"+user.ID+"/Policy", a.secrets.Admin.Token, policy, nil); e != nil {
		clean()
		a.fail(w, e)
		return
	}
	cred, e := a.engine.Authenticate(r.Context(), name, password)
	if e != nil {
		clean()
		a.fail(w, e)
		return
	}
	a.secrets.Users[id] = cred
	if e = a.saveSecretsLocked(); e != nil {
		delete(a.secrets.Users, id)
		clean()
		a.H.DBError(w, e)
		return
	}
	_, e = a.H.DB.Exec("INSERT INTO profiles(id,owner,name,kind,pin,rating,avatar)VALUES(?,?,?,?,?,?,?)", id, owner, in.Name, in.Kind, pin, in.Rating, in.Avatar)
	if e != nil {
		delete(a.secrets.Users, id)
		_ = a.saveSecretsLocked()
		clean()
		a.H.DBError(w, e)
		return
	}
	a.H.Audit(owner, "profile-created", id)
	apphost.JSON(w, 201, map[string]string{"id": id})
}
func profilePolicy(kind string, rating int) map[string]any {
	// The broker also checks ratings itself, including unknown/unrated content.
	p := map[string]any{"IsAdministrator": false, "IsHidden": true, "IsDisabled": false, "EnableAllFolders": true, "EnableMediaPlayback": true, "EnableAudioPlaybackTranscoding": true, "EnableVideoPlaybackTranscoding": true, "EnablePlaybackRemuxing": true, "EnableContentDeletion": false, "EnableContentDownloading": false, "EnableLiveTvAccess": false, "EnableLiveTvManagement": false, "EnableRemoteControlOfOtherUsers": false, "EnableSharedDeviceControl": false, "EnablePublicSharing": false, "EnableRemoteAccess": true, "EnableCollectionManagement": false, "EnableSubtitleManagement": false, "EnableSyncTranscoding": false}
	if kind == "child" {
		max := rating // Jellyfin uses US-style numeric parental ratings (confirmed in compatibility tests).
		p["MaxParentalRating"] = max
		p["BlockUnratedItems"] = []string{"Movie", "Series", "Other", "ChannelContent", "LiveTvChannel", "Trailer", "Music"}
	}
	return p
}
func (a *App) selectProfile(w http.ResponseWriter, r *http.Request) {
	var in struct{ ID, PIN string }
	if !apphost.Decode(w, r, &in) {
		return
	}
	p, e := a.profile(in.ID, apphost.Current(r).User.ID)
	if e != nil {
		apphost.Problem(w, 404, "Profile not found")
		return
	}
	if a.oneAccount(r) {
		p.pin = nil
		p.Protected = false
	}
	key := p.owner + ":" + p.ID
	a.mu.Lock()
	att := a.attempts[key]
	if !a.oneAccount(r) && att.Count >= 5 && time.Now().Before(att.Until) {
		a.mu.Unlock()
		apphost.Problem(w, 429, "Too many PIN attempts. Try again in five minutes.")
		return
	}
	a.mu.Unlock()
	good := len(p.pin) == 0 || bcrypt.CompareHashAndPassword(p.pin, []byte(in.PIN)) == nil
	if !good {
		a.mu.Lock()
		if time.Now().After(att.Until) {
			att = attempt{}
		}
		att.Count++
		att.Until = time.Now().Add(5 * time.Minute)
		a.attempts[key] = att
		a.mu.Unlock()
		apphost.Problem(w, 403, "The profile PIN did not match")
		return
	}
	a.mu.Lock()
	delete(a.attempts, key)
	a.mu.Unlock()
	a.mu.Lock()
	for id, play := range a.plays {
		if play.Session == sessionKey(r) {
			play.Cancel()
			delete(a.plays, id)
		}
	}
	a.mu.Unlock()
	_, e = a.H.DB.Exec("INSERT INTO active_profiles VALUES(?,?,?) ON CONFLICT(session)DO UPDATE SET profile=excluded.profile,unlocked=excluded.unlocked", sessionKey(r), p.ID, time.Now().Add(8*time.Hour).UTC().Format(time.RFC3339Nano))
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	apphost.JSON(w, 200, p)
}
func (a *App) lockProfile(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	for id, play := range a.plays {
		if play.Session == sessionKey(r) {
			play.Cancel()
			delete(a.plays, id)
		}
	}
	a.mu.Unlock()
	_, e := a.H.DB.Exec("DELETE FROM active_profiles WHERE session=?", sessionKey(r))
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	apphost.JSON(w, 200, map[string]bool{"locked": true})
}
func (a *App) editProfile(w http.ResponseWriter, r *http.Request) {
	active, ok := a.active(w, r)
	if !ok {
		return
	}
	if active.Kind != "adult" {
		apphost.Problem(w, 403, "Unlock an adult profile to edit profiles")
		return
	}
	p, e := a.profile(r.PathValue("id"), apphost.Current(r).User.ID)
	if e != nil {
		apphost.Problem(w, 404, "Profile not found")
		return
	}
	var in struct {
		Name, PIN, Avatar string
		Rating            int
		Libraries         []string
		BlockedTags       []string
	}
	if !apphost.Decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 1 || len(in.Name) > 40 || in.Rating < 0 || in.Rating > 18 || (p.Kind == "child" && in.Rating > 15) || len(in.BlockedTags) > 30 || len(in.Libraries) > 30 {
		apphost.Problem(w, 400, "Check the profile name and policy")
		return
	}
	pin := p.pin
	if in.PIN != "" && !a.oneAccount(r) {
		if !validPIN.MatchString(in.PIN) {
			apphost.Problem(w, 400, "Use a 4–12 digit PIN")
			return
		}
		pin, e = bcrypt.GenerateFromPassword([]byte(in.PIN), 12)
		if e != nil {
			a.H.DBError(w, e)
			return
		}
	}
	cred, ok := a.credentials(p.ID)
	if !ok {
		apphost.Problem(w, 503, "Restore this profile's media connection first")
		return
	}
	var mapped struct{ Policy map[string]any }
	if e = a.engine.Do(r.Context(), "GET", "/Users/"+cred.UserID, a.secrets.Admin.Token, nil, &mapped); e != nil || mapped.Policy == nil {
		apphost.Problem(w, 502, "The media profile policy could not be loaded")
		return
	}
	policy := mapped.Policy
	for k, v := range profilePolicy(p.Kind, in.Rating) {
		policy[k] = v
	}
	policy["BlockedTags"] = in.BlockedTags
	if len(in.Libraries) > 0 {
		for _, id := range in.Libraries {
			if !validID.MatchString(id) {
				apphost.Problem(w, 400, "Invalid library selection")
				return
			}
		}
		policy["EnableAllFolders"] = false
		policy["EnabledFolders"] = in.Libraries
	}
	if e = a.engine.Do(r.Context(), "POST", "/Users/"+cred.UserID+"/Policy", a.secrets.Admin.Token, policy, nil); e != nil {
		a.fail(w, e)
		return
	}
	if in.Avatar == "" {
		in.Avatar = p.Avatar
	}
	_, e = a.H.DB.Exec("UPDATE profiles SET name=?,pin=?,rating=?,avatar=? WHERE id=?", in.Name, pin, in.Rating, in.Avatar, p.ID)
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	_, _ = a.H.DB.Exec("DELETE FROM active_profiles WHERE profile=?", p.ID)
	a.mu.Lock()
	for id, play := range a.plays {
		if play.Profile == p.ID {
			delete(a.plays, id)
		}
	}
	a.mu.Unlock()
	a.H.Audit(p.owner, "profile-policy-updated", p.ID)
	apphost.JSON(w, 200, map[string]bool{"saved": true, "reselect": true})
}
func (a *App) preferences(w http.ResponseWriter, r *http.Request) {
	p, ok := a.active(w, r)
	if !ok {
		return
	}
	var in struct {
		Audio          string `json:"audio"`
		Subtitles      string `json:"subtitles"`
		Autoplay       bool   `json:"autoplay"`
		RecentSearches bool   `json:"recentSearches"`
		TV             bool   `json:"tv"`
	}
	if !apphost.Decode(w, r, &in) {
		return
	}
	if len(in.Audio) > 20 || len(in.Subtitles) > 20 {
		apphost.Problem(w, 400, "Choose a language code")
		return
	}
	raw, _ := json.Marshal(in)
	_, e := a.H.DB.Exec("UPDATE profiles SET prefs=? WHERE id=?", string(raw), p.ID)
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	apphost.JSON(w, 200, in)
}
func (a *App) clearHistory(w http.ResponseWriter, r *http.Request) {
	p, ok := a.active(w, r)
	if !ok {
		return
	}
	_, e := a.H.DB.Exec("DELETE FROM searches WHERE profile=?", p.ID)
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	apphost.JSON(w, 200, map[string]bool{"cleared": true})
}
