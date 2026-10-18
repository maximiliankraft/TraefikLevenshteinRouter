package levenshtein_router

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Config is the configuration of the Levenshtein redirect middleware.
type Config struct {
	// Hosts are the canonical hosts that may receive typo redirects.
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
	// MaxDistance is the maximum edit distance for a redirect.
	MaxDistance int `json:"maxDistance,omitempty" yaml:"maxDistance,omitempty"`
	// RedirectCode is the HTTP status used for redirects.
	RedirectCode int `json:"redirectCode,omitempty" yaml:"redirectCode,omitempty"`
}

// CreateConfig returns the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		MaxDistance:  2,
		RedirectCode: http.StatusPermanentRedirect,
	}
}

type host struct {
	match  string
	target string
}

// LevenshteinRouter redirects a request to the closest configured canonical
// host when the request host is within MaxDistance edits. Requests without a
// suitable match are passed to the next handler.
type LevenshteinRouter struct {
	next         http.Handler
	hosts        []host
	maxDistance  int
	redirectCode int
}

// New creates the middleware used by Traefik's plugin runtime.
func New(_ context.Context, next http.Handler, config *Config, _ string) (http.Handler, error) {
	if next == nil {
		return nil, fmt.Errorf("next handler is required")
	}
	if config == nil {
		return nil, fmt.Errorf("configuration is required")
	}
	if len(config.Hosts) == 0 {
		return nil, fmt.Errorf("at least one canonical host is required")
	}
	if config.MaxDistance < 0 {
		return nil, fmt.Errorf("maxDistance must not be negative")
	}
	if config.RedirectCode < http.StatusMultipleChoices || config.RedirectCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("redirectCode must be a 3xx status code")
	}

	hosts := make([]host, 0, len(config.Hosts))
	for _, candidate := range config.Hosts {
		canonical, err := normalizeCandidate(candidate)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, canonical)
	}

	return &LevenshteinRouter{
		next:         next,
		hosts:        hosts,
		maxDistance:  config.MaxDistance,
		redirectCode: config.RedirectCode,
	}, nil
}

func (l *LevenshteinRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requested := normalizeHost(r.Host)
	if requested == "" {
		l.next.ServeHTTP(w, r)
		return
	}

	var closest *host
	closestDistance := l.maxDistance + 1
	for i := range l.hosts {
		distance := levenshtein(requested, l.hosts[i].match)
		if distance <= l.maxDistance && distance < closestDistance {
			closest = &l.hosts[i]
			closestDistance = distance
		}
	}

	if closest == nil || closest.match == requested {
		l.next.ServeHTTP(w, r)
		return
	}

	http.Redirect(w, r, redirectURL(r, closest.target), l.redirectCode)
}

func normalizeCandidate(value string) (host, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return host{}, fmt.Errorf("canonical hosts must not be empty")
	}

	parsed, err := url.Parse("//" + value)
	if err != nil || parsed.Host != value || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return host{}, fmt.Errorf("invalid canonical host %q", value)
	}
	if parsed.Hostname() == "" {
		return host{}, fmt.Errorf("invalid canonical host %q", value)
	}

	return host{
		match:  normalizeHost(parsed.Host),
		target: value,
	}, nil
}

func normalizeHost(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	parsed, err := url.Parse("//" + value)
	if err != nil || parsed.Hostname() == "" {
		return strings.TrimSuffix(value, ".")
	}
	return strings.TrimSuffix(parsed.Hostname(), ".")
}

func redirectURL(r *http.Request, targetHost string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}

	target := &url.URL{
		Scheme:   scheme,
		Host:     targetHost,
		Path:     r.URL.Path,
		RawPath:  r.URL.RawPath,
		RawQuery: r.URL.RawQuery,
	}
	if r.URL.ForceQuery {
		target.ForceQuery = true
	}
	return target.String()
}

func levenshtein(left, right string) int {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	if len(leftRunes) == 0 {
		return len(rightRunes)
	}
	if len(rightRunes) == 0 {
		return len(leftRunes)
	}

	previous := make([]int, len(rightRunes)+1)
	current := make([]int, len(rightRunes)+1)
	for j := range previous {
		previous[j] = j
	}

	for i, leftRune := range leftRunes {
		current[0] = i + 1
		for j, rightRune := range rightRunes {
			cost := 0
			if leftRune != rightRune {
				cost = 1
			}
			current[j+1] = min(
				current[j]+1,
				previous[j+1]+1,
				previous[j]+cost,
			)
		}
		previous, current = current, previous
	}

	return previous[len(rightRunes)]
}

func min(values ...int) int {
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}
