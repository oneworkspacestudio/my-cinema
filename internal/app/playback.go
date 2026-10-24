package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/oneworkspacestudio/my-cinema/internal/jellyfin"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Playback struct {
	ID, Item, Profile, Session, PlaySession, Source string
	Method                                          string
	Cred                                            jellyfin.Credentials
	Duration, Position                              float64
	Created, Seen                                   time.Time
	Resources                                       map[string]string
	Context                                         context.Context
	Cancel                                          context.CancelFunc
}

func ioCopy(w io.Writer, r io.Reader, max int64) (int64, error) {
	return io.Copy(w, io.LimitReader(r, max))
}
func deviceProfile() map[string]any {
	return map[string]any{"Name": "My Cinema browser", "MaxStreamingBitrate": 20000000, "MaxStaticBitrate": 100000000, "DirectPlayProfiles": []any{map[string]any{"Container": "mp4,m4v", "Type": "Video", "VideoCodec": "h264", "AudioCodec": "aac,mp3"}, map[string]any{"Container": "webm", "Type": "Video", "VideoCodec": "vp8,vp9", "AudioCodec": "opus,vorbis"}}, "TranscodingProfiles": []any{map[string]any{"Container": "ts", "Type": "Video", "VideoCodec": "h264", "AudioCodec": "aac", "Protocol": "hls", "Context": "Streaming", "MaxAudioChannels": "2", "MinSegments": 2, "SegmentLength": 4, "BreakOnNonKeyFrames": true}}, "SubtitleProfiles": []any{map[string]string{"Format": "vtt", "Method": "External"}, map[string]string{"Format": "srt", "Method": "External"}, map[string]string{"Format": "ass", "Method": "Encode"}, map[string]string{"Format": "pgssub", "Method": "Encode"}}}
}
func (a *App) startPlayback(w http.ResponseWriter, r *http.Request) {
	p, c, ok := a.viewer(w, r)
	if !ok {
		return
	}
	var in struct {
		ID, Source      string
		Audio, Subtitle int
		Bitrate         int
		Restart         bool
	}
	if !apphost.Decode(w, r, &in) {
		return
	}
	item, ok := a.permitted(r, p, c, in.ID)
	if !ok || item.Type == "Series" || item.Type == "Season" {
		apphost.Problem(w, 404, "This video is unavailable to the active profile")
		return
	}
	if in.Audio < -1 || in.Subtitle < -1 || in.Audio > 100 || in.Subtitle > 100 {
		apphost.Problem(w, 400, "Invalid media track")
		return
	}
	if in.Bitrate == 0 {
		in.Bitrate = 20000000
	}
	if in.Bitrate < 500000 || in.Bitrate > 50000000 {
		apphost.Problem(w, 400, "Choose a supported quality")
		return
	}
	start := item.UserData.PlaybackPositionTicks
	if in.Restart {
		start = 0
	}
	q := url.Values{"UserId": {c.UserID}, "StartTimeTicks": {strconv.FormatInt(start, 10)}, "IsPlayback": {"true"}, "AutoOpenLiveStream": {"true"}, "MaxStreamingBitrate": {strconv.Itoa(in.Bitrate)}}
	if in.Audio >= 0 || in.Bitrate < 20000000 {
		q.Set("EnableDirectPlay", "false")
	}
	if in.Source != "" {
		if !validID.MatchString(in.Source) {
			apphost.Problem(w, 400, "Invalid video version")
			return
		}
		q.Set("MediaSourceId", in.Source)
	}
	if in.Audio >= 0 {
		q.Set("AudioStreamIndex", strconv.Itoa(in.Audio))
	}
	if in.Subtitle >= 0 {
		q.Set("SubtitleStreamIndex", strconv.Itoa(in.Subtitle))
	} else {
		q.Set("SubtitleStreamIndex", "-1")
	}
	var info struct {
		MediaSources  []jellyfin.Source
		PlaySessionID string `json:"PlaySessionId"`
		ErrorCode     string
	}
	e := a.engine.Do(r.Context(), "POST", "/Items/"+item.ID+"/PlaybackInfo?"+q.Encode(), c.Token, map[string]any{"DeviceProfile": deviceProfile()}, &info)
	if e != nil {
		a.fail(w, e)
		return
	}
	if info.ErrorCode != "" || len(info.MediaSources) == 0 {
		apphost.Problem(w, 422, "This video cannot be played with the selected tracks. Try another quality or version.")
		return
	}
	source := info.MediaSources[0]
	if in.Source != "" {
		found := false
		for _, s := range info.MediaSources {
			if s.ID == in.Source {
				source = s
				found = true
				break
			}
		}
		if !found {
			apphost.Problem(w, 404, "Video version unavailable")
			return
		}
	}
	id := apphost.ID()
	playCtx, cancel := context.WithCancel(context.Background())
	play := &Playback{ID: id, Item: item.ID, Profile: p.ID, Session: sessionKey(r), PlaySession: info.PlaySessionID, Source: source.ID, Cred: c, Duration: float64(item.RunTimeTicks) / 1e7, Position: float64(start) / 1e7, Created: time.Now(), Seen: time.Now(), Resources: map[string]string{}, Context: playCtx, Cancel: cancel}
	path := ""
	mode := "direct"
	textSubtitle := in.Subtitle < 0
	for _, track := range source.MediaStreams {
		if track.Type == "Subtitle" && track.Index == in.Subtitle && track.Codec != "pgssub" && track.Codec != "dvdsub" {
			textSubtitle = true
		}
	}
	if source.SupportsDirectPlay && in.Audio < 0 && textSubtitle && in.Bitrate >= 20000000 && (source.Container == "mp4" || source.Container == "m4v" || source.Container == "webm") {
		path = "/Videos/" + item.ID + "/stream." + source.Container + "?Static=true&MediaSourceId=" + urlQuery(source.ID) + "&PlaySessionId=" + urlQuery(info.PlaySessionID)
	} else if source.TranscodingUrl != "" {
		path = source.TranscodingUrl
		mode = "hls"
	} else if source.SupportsDirectStream && textSubtitle {
		path = "/Videos/" + item.ID + "/stream.mp4?Static=false&MediaSourceId=" + urlQuery(source.ID) + "&VideoCodec=copy&AudioCodec=aac&AudioStreamIndex=" + strconv.Itoa(in.Audio)
		mode = "remux"
	} else {
		cancel()
		apphost.Problem(w, 422, "No compatible playback route was available. Try another audio/subtitle track or quality.")
		return
	}
	play.Method = map[string]string{"direct": "DirectPlay", "remux": "DirectStream", "hls": "Transcode"}[mode]
	resource, e := a.resource(play, path, a.engine.URL)
	if e != nil {
		cancel()
		a.fail(w, e)
		return
	}
	tracks := []map[string]any{}
	for _, s := range source.MediaStreams {
		if s.Type == "Subtitle" && s.Codec != "pgssub" && s.Codec != "dvdsub" {
			path := fmt.Sprintf("/Videos/%s/%s/Subtitles/%d/Stream.vtt", item.ID, source.ID, s.Index)
			u, e := a.resource(play, path, a.engine.URL)
			if e == nil {
				tracks = append(tracks, map[string]any{"index": s.Index, "name": s.DisplayTitle, "language": s.Language, "url": u})
			}
		}
	}
	if e = a.report(r.Context(), play, "/Sessions/Playing", false); e != nil {
		cancel()
		a.fail(w, e)
		return
	}
	a.mu.Lock()
	for key, v := range a.plays {
		if v.Session == play.Session || time.Since(v.Seen) > 3*time.Minute {
			v.Cancel()
			delete(a.plays, key)
		}
	}
	a.plays[id] = play
	a.mu.Unlock()
	apphost.JSON(w, 201, map[string]any{"schema": 1, "id": id, "url": resource, "mode": mode, "start": play.Position, "duration": play.Duration, "subtitles": tracks, "selectedSubtitle": in.Subtitle, "selectedAudio": in.Audio, "item": a.dto(item)})
}
func (a *App) resource(p *Playback, raw, base string) (string, error) {
	b, e := url.Parse(base)
	if e != nil {
		return "", e
	}
	u, e := url.Parse(raw)
	if e != nil {
		return "", e
	}
	u = b.ResolveReference(u)
	origin, e := url.Parse(a.engine.URL)
	if e != nil {
		return "", e
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	sameItem := len(parts) >= 3 && strings.EqualFold(parts[0], "videos") && strings.EqualFold(strings.ReplaceAll(parts[1], "-", ""), strings.ReplaceAll(p.Item, "-", ""))
	if u.Scheme != origin.Scheme || u.Host != origin.Host || u.User != nil || !sameItem || strings.Contains(u.Path, "..") || strings.ContainsAny(u.Path, "\\\r\n") {
		return "", errors.New("media engine returned an unexpected stream resource")
	}
	if len(p.Resources) > 20000 {
		return "", errors.New("too many playback resources")
	}
	id := apphost.Hash(u.String())
	p.Resources[id] = u.RequestURI()
	return a.H.Base + "/api/playback/" + p.ID + "/stream/" + id, nil
}
func (a *App) playback(w http.ResponseWriter, r *http.Request) (*Playback, bool) {
	profile, _, ok := a.viewer(w, r)
	if !ok {
		return nil, false
	}
	a.mu.Lock()
	p, ok := a.plays[r.PathValue("id")]
	if ok {
		ok = p.Profile == profile.ID && p.Session == sessionKey(r) && time.Since(p.Seen) < 3*time.Minute && time.Since(p.Created) < 8*time.Hour
	}
	a.mu.Unlock()
	if !ok {
		apphost.Problem(w, 404, "Playback expired or belongs to another profile")
		return nil, false
	}
	if _, ok = a.permitted(r, profile, p.Cred, p.Item); !ok {
		p.Cancel()
		apphost.Problem(w, 404, "This video is no longer available to the profile")
		return nil, false
	}
	return p, true
}
func (a *App) report(ctx context.Context, p *Playback, path string, paused bool) error {
	return a.engine.Do(ctx, "POST", path, p.Cred.Token, map[string]any{"ItemId": p.Item, "MediaSourceId": p.Source, "PlaySessionId": p.PlaySession, "PositionTicks": int64(p.Position * 1e7), "CanSeek": true, "IsPaused": paused, "PlayMethod": p.Method}, nil)
}
func (a *App) progress(w http.ResponseWriter, r *http.Request) {
	p, ok := a.playback(w, r)
	if !ok {
		return
	}
	var in struct {
		Position float64
		Paused   bool
	}
	if !apphost.Decode(w, r, &in) {
		return
	}
	if math.IsNaN(in.Position) || math.IsInf(in.Position, 0) || in.Position < 0 || (p.Duration > 0 && in.Position > p.Duration+10) {
		apphost.Problem(w, 400, "Invalid playback position")
		return
	}
	a.mu.Lock()
	p.Position = in.Position
	p.Seen = time.Now()
	copy := *p
	a.mu.Unlock()
	if e := a.report(r.Context(), &copy, "/Sessions/Playing/Progress", in.Paused); e != nil {
		a.fail(w, e)
		return
	}
	apphost.JSON(w, 200, map[string]bool{"saved": true})
}
func (a *App) stopPlayback(w http.ResponseWriter, r *http.Request) {
	p, ok := a.playback(w, r)
	if !ok {
		return
	}
	var in struct{ Position float64 }
	if !apphost.Decode(w, r, &in) {
		return
	}
	if in.Position < 0 || math.IsNaN(in.Position) || math.IsInf(in.Position, 0) || (p.Duration > 0 && in.Position > p.Duration+10) {
		apphost.Problem(w, 400, "Invalid playback position")
		return
	}
	a.mu.Lock()
	p.Position = in.Position
	copy := *p
	delete(a.plays, p.ID)
	p.Cancel()
	a.mu.Unlock()
	if e := a.report(r.Context(), &copy, "/Sessions/Playing/Stopped", true); e != nil {
		a.fail(w, e)
		return
	}
	_ = a.engine.Do(r.Context(), "DELETE", "/Videos/ActiveEncodings?DeviceId=mycinema-broker&PlaySessionId="+urlQuery(p.PlaySession), p.Cred.Token, nil, nil)
	apphost.JSON(w, 200, map[string]bool{"stopped": true})
}

var uriAttr = regexp.MustCompile(`URI="([^"]+)"`)

func (a *App) stream(w http.ResponseWriter, r *http.Request) {
	p, ok := a.playback(w, r)
	if !ok {
		return
	}
	a.mu.Lock()
	path, ok := p.Resources[r.PathValue("path")]
	a.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stop := context.AfterFunc(p.Context, cancel)
	defer stop()
	req, e := http.NewRequestWithContext(ctx, "GET", a.engine.URL+path, nil)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	req.Header.Set("Authorization", `MediaBrowser Token="`+p.Cred.Token+`"`)
	if v := r.Header.Get("Range"); v != "" {
		req.Header.Set("Range", v)
	}
	client := &http.Client{Transport: a.engine.HTTP.Transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, e := client.Do(req)
	if e != nil {
		apphost.Problem(w, 502, "The video stream was interrupted. Retry playback.")
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 206 {
		apphost.Problem(w, 502, "The source or selected codec could not be streamed. Retry with a lower quality.")
		return
	}
	content := res.Header.Get("Content-Type")
	if strings.Contains(strings.ToLower(content), "mpegurl") || strings.HasSuffix(strings.Split(path, "?")[0], ".m3u8") {
		var out strings.Builder
		scan := bufio.NewScanner(io.LimitReader(res.Body, 2<<20))
		scan.Buffer(make([]byte, 4096), 256<<10)
		a.mu.Lock()
		defer a.mu.Unlock()
		for scan.Scan() {
			line := scan.Text()
			if strings.HasPrefix(line, "#") {
				if uriAttr.MatchString(line) {
					var err error
					line = uriAttr.ReplaceAllStringFunc(line, func(attr string) string {
						match := uriAttr.FindStringSubmatch(attr)
						u, e := a.resource(p, match[1], a.engine.URL+path)
						if e != nil {
							err = e
							return ""
						}
						return `URI="` + u + `"`
					})
					if err != nil {
						apphost.Problem(w, 502, "The playlist contains an unsupported resource")
						return
					}
				}
			} else if strings.TrimSpace(line) != "" {
				u, e := a.resource(p, line, a.engine.URL+path)
				if e != nil {
					apphost.Problem(w, 502, "The playlist contains an unsupported stream")
					return
				}
				line = u
			}
			out.WriteString(line)
			out.WriteByte('\n')
		}
		if scan.Err() != nil {
			apphost.Problem(w, 502, "The playlist was interrupted")
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-store")
		io.WriteString(w, out.String())
		return
	}
	for _, name := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		if v := res.Header.Get(name); v != "" {
			w.Header().Set(name, v)
		}
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, res.Body)
}
