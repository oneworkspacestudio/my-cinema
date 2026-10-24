package app

import (
	"github.com/oneworkspacestudio/my-cinema/internal/jellyfin"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Media struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Type       string            `json:"type"`
	Overview   string            `json:"overview"`
	Rating     string            `json:"rating"`
	Year       int               `json:"year"`
	Duration   float64           `json:"duration"`
	Progress   float64           `json:"progress"`
	Played     bool              `json:"played"`
	Favourite  bool              `json:"favourite"`
	Genres     []string          `json:"genres"`
	Poster     string            `json:"poster"`
	Backdrop   string            `json:"backdrop"`
	Added      string            `json:"added"`
	SeriesID   string            `json:"seriesId,omitempty"`
	SeriesName string            `json:"seriesName,omitempty"`
	Episode    int               `json:"episode,omitempty"`
	Season     int               `json:"season,omitempty"`
	People     []jellyfin.Person `json:"people,omitempty"`
	Sources    []Version         `json:"sources,omitempty"`
}
type Version struct {
	ID        string  `json:"id"`
	Container string  `json:"container"`
	Tracks    []Track `json:"tracks"`
}
type Track struct {
	Index    int    `json:"index"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Language string `json:"language"`
}

func (a *App) dto(i jellyfin.Item) Media {
	m := Media{ID: i.ID, Title: i.Name, Type: i.Type, Overview: i.Overview, Rating: i.OfficialRating, Year: i.ProductionYear, Duration: float64(i.RunTimeTicks) / 1e7, Progress: float64(i.UserData.PlaybackPositionTicks) / 1e7, Played: i.UserData.Played, Favourite: i.UserData.IsFavorite, Genres: i.Genres, Added: i.DateCreated, SeriesID: i.SeriesID, SeriesName: i.SeriesName, Episode: i.IndexNumber, Season: i.ParentIndexNumber, People: i.People}
	m.Poster = a.H.Base + "/api/items/" + i.ID + "/image/Primary"
	m.Backdrop = a.H.Base + "/api/items/" + i.ID + "/image/Backdrop"
	for _, s := range i.MediaSources {
		v := Version{ID: s.ID, Container: s.Container, Tracks: []Track{}}
		for _, t := range s.MediaStreams {
			if t.Type == "Audio" || t.Type == "Subtitle" {
				v.Tracks = append(v.Tracks, Track{t.Index, t.Type, t.DisplayTitle, t.Language})
			}
		}
		m.Sources = append(m.Sources, v)
	}
	return m
}
func ratingAge(raw string) (int, bool) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	for _, prefix := range []string{"GB-", "UK:", "UK-", "GB:", "BBFC-", "BBFC:"} {
		s = strings.TrimPrefix(s, prefix)
	}
	switch s {
	case "U", "G", "TV-Y", "TV-G", "APPROVED":
		return 0, true
	case "TV-Y7", "TV-Y7-FV":
		return 7, true
	case "PG", "TV-PG":
		return 10, true
	case "12", "12A":
		return 12, true
	case "PG-13":
		return 13, true
	case "TV-14":
		return 14, true
	case "15":
		return 15, true
	case "R", "16":
		return 16, true
	case "18", "NC-17", "TV-MA":
		return 18, true
	}
	return 0, false
}
func visible(p Profile, i jellyfin.Item) bool {
	if p.Kind != "child" {
		return true
	}
	age, known := ratingAge(i.OfficialRating)
	return known && age <= p.Rating
}
func (a *App) permitted(r *http.Request, p Profile, c jellyfin.Credentials, id string) (jellyfin.Item, bool) {
	if !validID.MatchString(id) {
		return jellyfin.Item{}, false
	}
	i, e := a.engine.Item(r.Context(), c, id)
	if e != nil || i.ID != id {
		return i, false
	}
	if p.Kind == "child" && i.OfficialRating == "" && i.SeriesID != "" && i.SeriesID != id {
		parent, e := a.engine.Item(r.Context(), c, i.SeriesID)
		if e == nil {
			i.OfficialRating = parent.OfficialRating
		}
	}
	return i, visible(p, i)
}
func (a *App) filtered(p Profile, items []jellyfin.Item) []Media {
	out := []Media{}
	for _, i := range items {
		if visible(p, i) {
			out = append(out, a.dto(i))
		}
	}
	return out
}
func (a *App) catalog(w http.ResponseWriter, r *http.Request) {
	p, c, ok := a.viewer(w, r)
	if !ok {
		return
	}
	q := url.Values{"Recursive": {"true"}, "IncludeItemTypes": {"Movie,Series,Video"}, "SortBy": {"SortName"}, "SortOrder": {"Ascending"}}
	switch r.URL.Query().Get("type") {
	case "movie":
		q.Set("IncludeItemTypes", "Movie")
	case "series":
		q.Set("IncludeItemTypes", "Series")
	case "new":
		q.Set("SortBy", "DateCreated")
		q.Set("SortOrder", "Descending")
	}
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	if start < 0 || start > 100000 {
		apphost.Problem(w, 400, "Invalid catalog page")
		return
	}
	q.Set("StartIndex", strconv.Itoa(start))
	q.Set("Limit", "100")
	out, e := a.engine.Items(r.Context(), c, q)
	if e != nil {
		a.fail(w, e)
		return
	}
	apphost.JSON(w, 200, map[string]any{"schema": 1, "items": a.filtered(p, out.Items), "next": start + len(out.Items), "hasMore": start+len(out.Items) < out.TotalRecordCount})
}

type Rail struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Items []Media `json:"items"`
}

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	p, c, ok := a.viewer(w, r)
	if !ok {
		return
	}
	rails := []Rail{}
	add := func(id, name string, out jellyfin.Items) {
		ms := a.filtered(p, out.Items)
		if len(ms) > 0 {
			rails = append(rails, Rail{id, name, ms})
		}
	}
	resume, e := a.engine.Resume(r.Context(), c)
	if e != nil {
		a.fail(w, e)
		return
	}
	add("continue", "Continue watching", resume)
	queries := []struct {
		id, name string
		q        url.Values
	}{
		{"list", "My List", url.Values{"Recursive": {"true"}, "IncludeItemTypes": {"Movie,Series,Video"}, "Filters": {"IsFavorite"}, "SortBy": {"SortName"}}},
		{"recent", "Recently added", url.Values{"Recursive": {"true"}, "IncludeItemTypes": {"Movie,Series,Video"}, "SortBy": {"DateCreated"}, "SortOrder": {"Descending"}}},
		{"movies", "Movie night", url.Values{"Recursive": {"true"}, "IncludeItemTypes": {"Movie,Video"}, "SortBy": {"SortName"}}},
		{"series", "Your next series", url.Values{"Recursive": {"true"}, "IncludeItemTypes": {"Series"}, "SortBy": {"SortName"}}},
	}
	for _, q := range queries {
		q.q.Set("Limit", "30")
		out, e := a.engine.Items(r.Context(), c, q.q)
		if e != nil {
			a.fail(w, e)
			return
		}
		add(q.id, q.name, out)
	}
	if len(resume.Items) > 0 && visible(p, resume.Items[0]) && len(resume.Items[0].Genres) > 0 {
		seed := resume.Items[0]
		out, e := a.engine.Items(r.Context(), c, url.Values{"Recursive": {"true"}, "IncludeItemTypes": {"Movie,Series,Video"}, "Genres": {seed.Genres[0]}, "Limit": {"20"}})
		if e == nil {
			filtered := out.Items[:0]
			for _, i := range out.Items {
				if i.ID != seed.ID {
					filtered = append(filtered, i)
				}
			}
			out.Items = filtered
			add("suggested", "Suggestions after “"+seed.Name+"”", out)
		}
	}
	var next jellyfin.Items
	if e = a.engine.Do(r.Context(), "GET", "/Shows/NextUp?UserId="+c.UserID+"&Limit=20&Fields="+urlQuery(jellyfin.Fields), c.Token, nil, &next); e == nil {
		add("next", "Next episodes", next)
	}
	var hero *Media
	for _, row := range rails {
		if len(row.Items) > 0 {
			m := row.Items[0]
			hero = &m
			break
		}
	}
	apphost.JSON(w, 200, map[string]any{"schema": 1, "profile": p, "hero": hero, "rows": rails})
}
func (a *App) search(w http.ResponseWriter, r *http.Request) {
	p, c, ok := a.viewer(w, r)
	if !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 160 {
		apphost.Problem(w, 400, "Use a shorter search")
		return
	}
	if q == "" {
		apphost.JSON(w, 200, map[string]any{"items": []Media{}})
		return
	}
	out, e := a.engine.Items(r.Context(), c, url.Values{"Recursive": {"true"}, "IncludeItemTypes": {"Movie,Series,Episode,Video"}, "SearchTerm": {q}, "Limit": {"80"}})
	if e != nil {
		a.fail(w, e)
		return
	}
	apphost.JSON(w, 200, map[string]any{"schema": 1, "items": a.filtered(p, out.Items)})
}
func (a *App) myList(w http.ResponseWriter, r *http.Request) {
	p, c, ok := a.viewer(w, r)
	if !ok {
		return
	}
	out, e := a.engine.Items(r.Context(), c, url.Values{"Recursive": {"true"}, "IncludeItemTypes": {"Movie,Series,Video"}, "Filters": {"IsFavorite"}, "SortBy": {"SortName"}, "Limit": {"500"}})
	if e != nil {
		a.fail(w, e)
		return
	}
	items := a.filtered(p, out.Items)
	for n := range items {
		var at string
		if a.H.DB.QueryRow("SELECT at FROM saved_titles WHERE profile=? AND item=?", p.ID, items[n].ID).Scan(&at) == nil {
			items[n].Added = at
		}
	}
	apphost.JSON(w, 200, map[string]any{"schema": 1, "items": items})
}
func (a *App) item(w http.ResponseWriter, r *http.Request) {
	p, c, ok := a.viewer(w, r)
	if !ok {
		return
	}
	i, ok := a.permitted(r, p, c, r.PathValue("id"))
	if !ok {
		apphost.Problem(w, 404, "Title not found in this profile")
		return
	}
	apphost.JSON(w, 200, a.dto(i))
}
func (a *App) image(w http.ResponseWriter, r *http.Request) {
	p, c, ok := a.viewer(w, r)
	if !ok {
		return
	}
	i, ok := a.permitted(r, p, c, r.PathValue("id"))
	if !ok {
		apphost.Problem(w, 404, "Artwork unavailable")
		return
	}
	kind := r.PathValue("kind")
	if kind != "Primary" && kind != "Backdrop" {
		apphost.Problem(w, 404, "Artwork unavailable")
		return
	}
	id := i.ID
	if kind == "Backdrop" && len(i.BackdropImageTags) == 0 && i.ParentBackdropItemID != "" {
		if _, ok = a.permitted(r, p, c, i.ParentBackdropItemID); !ok {
			http.NotFound(w, r)
			return
		}
		id = i.ParentBackdropItemID
	}
	res, e := a.engine.Request(r.Context(), "GET", "/Items/"+id+"/Images/"+kind+"?maxWidth=1600&quality=88", c.Token, nil)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "image/") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", res.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = ioCopy(w, res.Body, 20<<20)
}
func (a *App) seasons(w http.ResponseWriter, r *http.Request) {
	p, c, ok := a.viewer(w, r)
	if !ok {
		return
	}
	series, ok := a.permitted(r, p, c, r.PathValue("id"))
	if !ok || series.Type != "Series" {
		apphost.Problem(w, 404, "Series unavailable")
		return
	}
	var out jellyfin.Items
	e := a.engine.Do(r.Context(), "GET", "/Shows/"+series.ID+"/Seasons?UserId="+c.UserID+"&Fields="+urlQuery(jellyfin.Fields), c.Token, nil, &out)
	if e != nil {
		a.fail(w, e)
		return
	}
	for n := range out.Items {
		if out.Items[n].OfficialRating == "" {
			out.Items[n].OfficialRating = series.OfficialRating
		}
	}
	apphost.JSON(w, 200, map[string]any{"items": a.filtered(p, out.Items)})
}
func (a *App) episodes(w http.ResponseWriter, r *http.Request) {
	p, c, ok := a.viewer(w, r)
	if !ok {
		return
	}
	series, ok := a.permitted(r, p, c, r.PathValue("id"))
	if !ok || series.Type != "Series" {
		apphost.Problem(w, 404, "Series unavailable")
		return
	}
	q := url.Values{"UserId": {c.UserID}, "Fields": {jellyfin.Fields}, "Limit": {"500"}, "SortBy": {"ParentIndexNumber,IndexNumber"}, "SortOrder": {"Ascending"}}
	if season := r.URL.Query().Get("season"); season != "" {
		if !validID.MatchString(season) {
			apphost.Problem(w, 400, "Invalid season")
			return
		}
		q.Set("SeasonId", season)
	}
	var out jellyfin.Items
	e := a.engine.Do(r.Context(), "GET", "/Shows/"+series.ID+"/Episodes?"+q.Encode(), c.Token, nil, &out)
	if e != nil {
		a.fail(w, e)
		return
	}
	for n := range out.Items {
		if out.Items[n].OfficialRating == "" {
			out.Items[n].OfficialRating = series.OfficialRating
		}
	}
	apphost.JSON(w, 200, map[string]any{"items": a.filtered(p, out.Items)})
}
func (a *App) favourite(w http.ResponseWriter, r *http.Request) {
	p, c, ok := a.viewer(w, r)
	if !ok {
		return
	}
	i, ok := a.permitted(r, p, c, r.PathValue("id"))
	if !ok {
		apphost.Problem(w, 404, "Title unavailable")
		return
	}
	if e := a.engine.Do(r.Context(), r.Method, "/Users/"+c.UserID+"/FavoriteItems/"+i.ID, c.Token, nil, nil); e != nil {
		a.fail(w, e)
		return
	}
	if r.Method == "POST" {
		_, _ = a.H.DB.Exec("INSERT INTO saved_titles VALUES(?,?,?) ON CONFLICT(profile,item)DO NOTHING", p.ID, i.ID, time.Now().UTC().Format(time.RFC3339Nano))
	} else {
		_, _ = a.H.DB.Exec("DELETE FROM saved_titles WHERE profile=? AND item=?", p.ID, i.ID)
	}
	apphost.JSON(w, 200, map[string]bool{"favourite": r.Method == "POST"})
}
func (a *App) similar(w http.ResponseWriter, r *http.Request) {
	p, c, ok := a.viewer(w, r)
	if !ok {
		return
	}
	i, ok := a.permitted(r, p, c, r.PathValue("id"))
	if !ok {
		apphost.Problem(w, 404, "Title unavailable")
		return
	}
	var out jellyfin.Items
	if e := a.engine.Do(r.Context(), "GET", "/Items/"+i.ID+"/Similar?UserId="+c.UserID+"&Limit=12&Fields="+urlQuery(jellyfin.Fields), c.Token, nil, &out); e != nil {
		a.fail(w, e)
		return
	}
	apphost.JSON(w, 200, map[string]any{"items": a.filtered(p, out.Items)})
}
func (a *App) watched(w http.ResponseWriter, r *http.Request) {
	p, c, ok := a.viewer(w, r)
	if !ok {
		return
	}
	i, ok := a.permitted(r, p, c, r.PathValue("id"))
	if !ok {
		apphost.Problem(w, 404, "Title unavailable")
		return
	}
	var in struct{ Watched bool }
	if !apphost.Decode(w, r, &in) {
		return
	}
	method := "DELETE"
	if in.Watched {
		method = "POST"
	}
	if e := a.engine.Do(r.Context(), method, "/Users/"+c.UserID+"/PlayedItems/"+i.ID, c.Token, nil, nil); e != nil {
		a.fail(w, e)
		return
	}
	apphost.JSON(w, 200, map[string]bool{"watched": in.Watched})
}
