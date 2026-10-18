package levenshtein_router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func testMiddleware(t *testing.T, config *Config) http.Handler {
	t.Helper()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	middleware, err := New(nil, next, config, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return middleware
}

func TestServeHTTPRedirectsToClosestHost(t *testing.T) {
	handler := testMiddleware(t, &Config{
		Hosts:        []string{"foo.localhost", "bar.localhost"},
		MaxDistance:  2,
		RedirectCode: http.StatusPermanentRedirect,
	})

	request := httptest.NewRequest(http.MethodGet, "http://fo.localhost/dashboard?tab=one", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusPermanentRedirect {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusPermanentRedirect)
	}
	if got, want := recorder.Header().Get("Location"), "http://foo.localhost/dashboard?tab=one"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestServeHTTPUsesFirstHostForEqualDistance(t *testing.T) {
	handler := testMiddleware(t, &Config{
		Hosts:        []string{"foo.localhost", "fop.localhost"},
		MaxDistance:  1,
		RedirectCode: http.StatusTemporaryRedirect,
	})

	request := httptest.NewRequest(http.MethodGet, "http://fo.localhost/", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got, want := recorder.Header().Get("Location"), "http://foo.localhost/"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestServeHTTPPassesExactAndDistantHostsToNext(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{
		{name: "exact", host: "foo.localhost"},
		{name: "too distant", host: "completely-different.localhost"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := testMiddleware(t, &Config{
				Hosts:        []string{"foo.localhost", "bar.localhost"},
				MaxDistance:  1,
				RedirectCode: http.StatusTemporaryRedirect,
			})
			request := httptest.NewRequest(http.MethodGet, "http://"+test.host+"/", nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
			}
		})
	}
}

func TestServeHTTPPreservesForwardedSchemeAndPort(t *testing.T) {
	handler := testMiddleware(t, &Config{
		Hosts:        []string{"foo.localhost:8443"},
		MaxDistance:  1,
		RedirectCode: http.StatusFound,
	})
	request := httptest.NewRequest(http.MethodGet, "http://fo.localhost:8080/path", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got, want := recorder.Header().Get("Location"), "https://foo.localhost:8443/path"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{name: "no hosts", config: &Config{}},
		{name: "negative distance", config: &Config{Hosts: []string{"foo.localhost"}, MaxDistance: -1}},
		{name: "non redirect status", config: &Config{Hosts: []string{"foo.localhost"}, RedirectCode: http.StatusOK}},
		{name: "host with path", config: &Config{Hosts: []string{"foo.localhost/path"}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(nil, http.NotFoundHandler(), test.config, "test")
			if err == nil {
				t.Fatal("New() error = nil, want error")
			}
		})
	}
}

func TestLevenshtein(t *testing.T) {
	if got, want := levenshtein("fo", "foo"), 1; got != want {
		t.Fatalf("levenshtein() = %d, want %d", got, want)
	}
	if got, want := levenshtein("büro", "buro"), 1; got != want {
		t.Fatalf("levenshtein() = %d, want %d", got, want)
	}
}
