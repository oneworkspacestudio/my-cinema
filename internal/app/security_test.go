package app

import (
	"github.com/oneworkspacestudio/my-cinema/internal/jellyfin"
	"strings"
	"testing"
)

func TestMediaEnvironmentDoesNotAdoptCredentialsOrInventLayers(t *testing.T) {
	t.Setenv("WORKSPACE_OIDC_CLIENT_SECRET", "not-for-media")
	t.Setenv("OPENAI_API_KEY", "not-for-media")
	for _, layer := range []string{"", "RunAsInvoker", "DetectorsAppHealth"} {
		t.Setenv("__COMPAT_LAYER", layer)
		env := strings.Join(mediaEnvironment(), "\n")
		if strings.Contains(env, "not-for-media") || strings.Contains(env, "RunAsInvoker") {
			t.Fatal("unexpected media environment")
		}
		if strings.Contains(env, "__COMPAT_LAYER=") != (layer == "DetectorsAppHealth") {
			t.Fatal("app health inheritance changed")
		}
	}
}

func TestChildRatingBoundary(t *testing.T) {
	p := Profile{Kind: "child", Rating: 12}
	for _, tt := range []struct {
		rating string
		want   bool
	}{{"", false}, {"NR", false}, {"unknown", false}, {"GB-U", true}, {"GB-PG", true}, {"GB-12", true}, {"PG-13", false}, {"GB-15", false}, {"TV-MA", false}, {"TV-Y7", true}, {"18", false}} {
		if got := visible(p, jellyfin.Item{OfficialRating: tt.rating}); got != tt.want {
			t.Errorf("%q allowed=%v, want %v", tt.rating, got, tt.want)
		}
	}
	if !visible(Profile{Kind: "adult"}, jellyfin.Item{}) {
		t.Fatal("adult may choose unrated media")
	}
}
func TestNoMediaPrivilegeEscalation(t *testing.T) {
	for _, kind := range []string{"adult", "child"} {
		p := profilePolicy(kind, 12)
		for _, k := range []string{"IsAdministrator", "EnableContentDeletion", "EnableContentDownloading", "EnableLiveTvManagement", "EnableRemoteControlOfOtherUsers", "EnableSharedDeviceControl", "EnablePublicSharing", "EnableCollectionManagement"} {
			if p[k] != false {
				t.Fatal(kind, k)
			}
		}
		if kind == "child" && p["MaxParentalRating"] != 12 {
			t.Fatal("missing upstream rating boundary")
		}
	}
}
func TestOpaqueStreamResources(t *testing.T) {
	a := &App{engine: jellyfin.New("http://127.0.0.1:9120")}
	p := &Playback{ID: "play", Item: "abcdef", Resources: map[string]string{}}
	for _, raw := range []string{"https://evil.example/videos/abcdef/stream.mp4", "http://127.0.0.1:9120/System/Info", "/Videos/other/stream.mp4", "/Videos/abcdef/../../System/Info", "//evil.example/videos/abcdef/master.m3u8"} {
		if _, e := a.resource(p, raw, a.engine.URL); e == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
func TestMediaExtensionsAndContainment(t *testing.T) {
	for _, s := range []string{"movie.exe", "film.mp4.exe", "notes.txt"} {
		if isMedia(s) {
			t.Fatal(s)
		}
	}
	if !isMedia("Film.MKV") {
		t.Fatal("case insensitive media")
	}
	if inside("C:/Media", "C:/Media-Other") {
		t.Fatal("sibling folder accepted")
	}
}
