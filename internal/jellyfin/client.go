// Package jellyfin is the only application boundary allowed to speak Jellyfin.
package jellyfin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const Version = "12.3"

type Client struct {
	URL  string
	HTTP *http.Client
}
type HTTPError struct {
	Method, Path string
	Status       int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("media engine %s %s failed (%d)", e.Method, e.Path, e.Status)
}
func Transient(e error) bool {
	var h *HTTPError
	return errors.As(e, &h) && (h.Status == 503 || h.Status == 502)
}

func New(raw string) *Client {
	return &Client{URL: strings.TrimRight(raw, "/"), HTTP: &http.Client{Timeout: 35 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *Client) Request(ctx context.Context, method, path, token string, body any) (*http.Response, error) {
	var b io.Reader
	if body != nil {
		raw, e := json.Marshal(body)
		if e != nil {
			return nil, e
		}
		b = bytes.NewReader(raw)
	}
	r, e := http.NewRequestWithContext(ctx, method, c.URL+path, b)
	if e != nil {
		return nil, e
	}
	r.Header.Set("Content-Type", "application/json")
	auth := `MediaBrowser Client="My Cinema", Device="Private broker", DeviceId="mycinema-broker", Version="0.1"`
	if token != "" {
		auth += `, Token="` + token + `"`
	}
	r.Header.Set("Authorization", auth)
	return c.HTTP.Do(r)
}
func (c *Client) Do(ctx context.Context, method, path, token string, body, out any) error {
	res, e := c.Request(ctx, method, path, token, body)
	if e != nil {
		return errors.New("The media engine is not responding")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		return &HTTPError{method, strings.Split(path, "?")[0], res.StatusCode}
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(out)
	}
	return nil
}

type Credentials struct {
	UserID string `json:"userId"`
	Token  string `json:"token"`
}

func (c *Client) Authenticate(ctx context.Context, name, password string) (Credentials, error) {
	var out struct {
		AccessToken string
		User        struct {
			ID string `json:"Id"`
		}
	}
	e := c.Do(ctx, "POST", "/Users/AuthenticateByName", "", map[string]string{"Username": name, "Pw": password}, &out)
	if e == nil && (out.AccessToken == "" || out.User.ID == "") {
		e = errors.New("The media engine returned an incomplete sign-in")
	}
	return Credentials{out.User.ID, out.AccessToken}, e
}

type Item struct {
	ID                   string `json:"Id"`
	Name                 string
	Type                 string
	Overview             string
	OfficialRating       string
	ProductionYear       int
	RunTimeTicks         int64
	DateCreated          string
	Genres               []string
	ImageTags            map[string]string
	BackdropImageTags    []string
	ParentBackdropItemID string `json:"ParentBackdropItemId"`
	SeriesID             string `json:"SeriesId"`
	SeriesName           string
	IndexNumber          int
	ParentIndexNumber    int
	UserData             struct {
		PlaybackPositionTicks int64
		Played                bool
		IsFavorite            bool
		PlayCount             int
	}
	MediaSources []Source
	People       []Person
}
type Person struct {
	Name string
	Type string
	Role string
}
type Stream struct {
	Index                               int
	Type, Codec, Language, DisplayTitle string
	IsDefault                           bool
	IsExternal                          bool
	DeliveryUrl                         string
}
type Source struct {
	ID                                                            string `json:"Id"`
	Container, VideoType                                          string
	RunTimeTicks                                                  int64
	SupportsDirectPlay, SupportsDirectStream, SupportsTranscoding bool
	TranscodingUrl                                                string
	MediaStreams                                                  []Stream
}
type Items struct {
	Items            []Item
	TotalRecordCount int
}

const Fields = "Overview,Genres,DateCreated,MediaSources,People,PrimaryImageAspectRatio"

func (c *Client) Items(ctx context.Context, cred Credentials, q url.Values) (Items, error) {
	q.Set("UserId", cred.UserID)
	q.Set("Fields", Fields)
	if q.Get("Limit") == "" {
		q.Set("Limit", "100")
	}
	q.Set("EnableTotalRecordCount", "true")
	var out Items
	e := c.Do(ctx, "GET", "/Items?"+q.Encode(), cred.Token, nil, &out)
	return out, e
}
func (c *Client) Item(ctx context.Context, cred Credentials, id string) (Item, error) {
	var out Item
	e := c.Do(ctx, "GET", "/Users/"+url.PathEscape(cred.UserID)+"/Items/"+url.PathEscape(id)+"?Fields="+url.QueryEscape(Fields), cred.Token, nil, &out)
	return out, e
}
func (c *Client) Resume(ctx context.Context, cred Credentials) (Items, error) {
	var out Items
	e := c.Do(ctx, "GET", "/Users/"+url.PathEscape(cred.UserID)+"/Items/Resume?Limit=30&MediaTypes=Video&Fields="+url.QueryEscape(Fields), cred.Token, nil, &out)
	return out, e
}
