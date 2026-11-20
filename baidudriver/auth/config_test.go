package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNormalizeConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := normalizeConfig(context.Background(), Config{
		AppKey:      "  app-key  ",
		SecretKey:   "  secret-key  ",
		RedirectURI: "  http://127.0.0.1:18787/oauth/callback  ",
		Store:       NewMemoryStore(),
	})
	if err != nil {
		t.Fatalf("normalizeConfig() error = %v", err)
	}
	if cfg.appKey != "app-key" || cfg.secretKey != "secret-key" {
		t.Fatalf("credentials not trimmed: app=%q secret=%q", cfg.appKey, cfg.secretKey)
	}
	if cfg.redirectURI != "http://127.0.0.1:18787/oauth/callback" {
		t.Errorf("redirectURI = %q", cfg.redirectURI)
	}
	if cfg.listenAddress != "127.0.0.1:18787" {
		t.Errorf("listenAddress = %q", cfg.listenAddress)
	}
	if got := cfg.authorizationEndpoint.String(); got != defaultAuthorizationEndpoint {
		t.Errorf("authorization endpoint = %q", got)
	}
	if got := cfg.tokenEndpoint.String(); got != defaultTokenEndpoint {
		t.Errorf("token endpoint = %q", got)
	}
	if cfg.scope != defaultScope {
		t.Errorf("scope = %q, want %q", cfg.scope, defaultScope)
	}
	if cfg.expirySkew != defaultExpirySkew {
		t.Errorf("expirySkew = %v, want %v", cfg.expirySkew, defaultExpirySkew)
	}
	if cfg.storeKey != credentialStoreKey("app-key", cfg.redirectURI, defaultScope) {
		t.Errorf("storeKey = %q", cfg.storeKey)
	}
	if cfg.httpClient == nil || cfg.httpClient.Timeout != defaultHTTPTimeout {
		t.Errorf("default HTTP client = %#v", cfg.httpClient)
	}
	if cfg.openBrowser || cfg.browserOpener != nil {
		t.Error("browser launch should be opt-in")
	}
}

func TestNormalizeConfigPreservesExplicitOptions(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	httpClient := &http.Client{Timeout: 3 * time.Second, Jar: jar}
	cfg, err := normalizeConfig(context.Background(), Config{
		AppKey:                "app",
		SecretKey:             "secret",
		RedirectURI:           "http://[::1]:18788/oauth/callback",
		Store:                 store,
		StoreKey:              "account-a",
		Scope:                 "basic netdisk custom",
		ExpirySkew:            time.Minute,
		AuthorizationEndpoint: "http://127.0.0.1:18080/authorize",
		TokenEndpoint:         "http://localhost:18081/xpan/oauth/get",
		OAuthHTTPClient:       httpClient,
		OpenBrowser:           true,
		BrowserOpener:         func(context.Context, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("normalizeConfig() error = %v", err)
	}
	if cfg.listenAddress != "[::1]:18788" {
		t.Errorf("listenAddress = %q", cfg.listenAddress)
	}
	if cfg.store != store || cfg.storeKey != "account-a" {
		t.Error("explicit store settings were not preserved")
	}
	if cfg.scope != "basic netdisk custom" || cfg.expirySkew != time.Minute {
		t.Errorf("scope/skew = %q/%v", cfg.scope, cfg.expirySkew)
	}
	if cfg.httpClient == httpClient {
		t.Error("explicit OAuthHTTPClient was not copied before applying security policy")
	}
	if cfg.httpClient.Timeout != httpClient.Timeout || cfg.httpClient.CheckRedirect == nil {
		t.Error("copied OAuthHTTPClient did not preserve settings or reject redirects")
	}
	if cfg.httpClient.Jar != nil {
		t.Error("OAuth HTTP client retained a cookie jar")
	}
	if httpClient.CheckRedirect != nil || httpClient.Jar != jar {
		t.Error("normalizeConfig mutated the caller's OAuthHTTPClient")
	}
	if !cfg.openBrowser || cfg.browserOpener == nil {
		t.Error("explicit browser settings were not preserved")
	}
	if err := cfg.httpClient.CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Errorf("CheckRedirect() error = %v", err)
	}
}

func TestNormalizeConfigSelectsSystemStoreByDefault(t *testing.T) {
	t.Parallel()

	cfg, err := normalizeConfig(context.Background(), Config{
		AppKey:         "app",
		SecretKey:      "secret",
		RedirectURI:    "http://127.0.0.1:18792/oauth/callback",
		KeyringService: " custom-service ",
	})
	if err != nil {
		t.Fatalf("normalizeConfig() error = %v", err)
	}
	store, ok := cfg.store.(*SystemStore)
	if !ok {
		t.Fatalf("store type = %T, want *SystemStore", cfg.store)
	}
	if store.service != "custom-service" {
		t.Errorf("system store service = %q", store.service)
	}
}

func TestNormalizeConfigRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	valid := Config{
		AppKey:      "app",
		SecretKey:   "secret",
		RedirectURI: "http://127.0.0.1:18789/oauth/callback",
		Store:       NewMemoryStore(),
	}
	cases := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{name: "missing app key", edit: func(c *Config) { c.AppKey = " " }, want: "AppKey and SecretKey"},
		{name: "missing secret", edit: func(c *Config) { c.SecretKey = "" }, want: "AppKey and SecretKey"},
		{name: "https redirect", edit: func(c *Config) { c.RedirectURI = "https://127.0.0.1:18789/callback" }, want: "HTTP loopback"},
		{name: "remote redirect", edit: func(c *Config) { c.RedirectURI = "http://example.com:18789/callback" }, want: "host must be"},
		{name: "redirect credentials", edit: func(c *Config) { c.RedirectURI = "http://user@127.0.0.1:18789/callback" }, want: "without credentials"},
		{name: "missing port", edit: func(c *Config) { c.RedirectURI = "http://127.0.0.1/callback" }, want: "include a port"},
		{name: "zero port", edit: func(c *Config) { c.RedirectURI = "http://127.0.0.1:0/callback" }, want: "between 1 and 65535"},
		{name: "missing callback path", edit: func(c *Config) { c.RedirectURI = "http://127.0.0.1:18789/" }, want: "callback path"},
		{name: "redirect query", edit: func(c *Config) { c.RedirectURI += "?x=1" }, want: "query"},
		{name: "redirect fragment", edit: func(c *Config) { c.RedirectURI += "#fragment" }, want: "fragment"},
		{name: "encoded redirect path", edit: func(c *Config) { c.RedirectURI = "http://127.0.0.1:18789/oauth%2Fcallback" }, want: "encoded path"},
		{name: "authorization query", edit: func(c *Config) { c.AuthorizationEndpoint = "https://pan.baidu.com/oauth?x=1" }, want: "authorization endpoint"},
		{name: "authorization credentials", edit: func(c *Config) { c.AuthorizationEndpoint = "https://user@pan.baidu.com/oauth" }, want: "authorization endpoint"},
		{name: "insecure authorization", edit: func(c *Config) { c.AuthorizationEndpoint = "http://example.com/oauth" }, want: "must use HTTPS"},
		{name: "token fragment", edit: func(c *Config) { c.TokenEndpoint = "https://pan.baidu.com/token#x" }, want: "token endpoint"},
		{name: "insecure token", edit: func(c *Config) { c.TokenEndpoint = "http://example.com/token" }, want: "must use HTTPS"},
		{name: "scope newline", edit: func(c *Config) { c.Scope = "basic\nnetdisk" }, want: "Scope"},
		{name: "negative skew", edit: func(c *Config) { c.ExpirySkew = -time.Second }, want: "ExpirySkew"},
		{name: "store key newline", edit: func(c *Config) { c.StoreKey = "a\nb" }, want: "StoreKey"},
		{name: "store key too long", edit: func(c *Config) { c.StoreKey = strings.Repeat("a", 257) }, want: "StoreKey"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := valid
			tc.edit(&cfg)
			_, err := normalizeConfig(context.Background(), cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("normalizeConfig() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestNormalizeConfigContext(t *testing.T) {
	t.Parallel()

	cfg := Config{
		AppKey:      "app",
		SecretKey:   "secret",
		RedirectURI: "http://127.0.0.1:18790/oauth/callback",
		Store:       NewMemoryStore(),
	}
	if _, err := normalizeConfig(nil, cfg); err == nil || !strings.Contains(err.Error(), "context") {
		t.Fatalf("nil context error = %v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := normalizeConfig(cancelled, cfg); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context error = %v", err)
	}
}

func TestBuildAuthorizationURLExactParameters(t *testing.T) {
	t.Parallel()

	authorize, _ := url.Parse("https://pan.baidu.com/union/oauth/authorize")
	cfg := normalizedConfig{
		appKey:                "app key/+?",
		secretKey:             "must-not-appear",
		redirectURI:           "http://127.0.0.1:18791/oauth/callback",
		scope:                 "basic,netdisk",
		authorizationEndpoint: authorize,
	}
	const state = "state-value"
	raw := buildAuthorizationURL(cfg, state)
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", raw, err)
	}
	if u.Scheme != "https" || u.Host != "pan.baidu.com" || u.Path != "/union/oauth/authorize" {
		t.Errorf("authorization endpoint = %s://%s%s", u.Scheme, u.Host, u.Path)
	}
	want := url.Values{
		"authorization_type": {"directory"},
		"client_id":          {"app key/+?"},
		"redirect_uri":       {cfg.redirectURI},
		"response_type":      {"code"},
		"scope":              {"basic,netdisk"},
		"state":              {state},
	}
	if got := u.Query(); !reflect.DeepEqual(got, want) {
		t.Errorf("query = %#v, want %#v", got, want)
	}
	if strings.Contains(raw, cfg.secretKey) {
		t.Error("authorization URL exposes SecretKey")
	}
}

func TestRandomURLSafeEntropyAndAlphabet(t *testing.T) {
	t.Parallel()

	first, err := randomURLSafe(32)
	if err != nil {
		t.Fatalf("randomURLSafe() error = %v", err)
	}
	second, err := randomURLSafe(32)
	if err != nil {
		t.Fatalf("randomURLSafe() error = %v", err)
	}
	if len(first) != 43 || len(second) != 43 {
		t.Fatalf("encoded lengths = %d, %d; want 43", len(first), len(second))
	}
	if first == second {
		t.Fatal("two independently generated values are identical")
	}
	for _, value := range []string{first, second} {
		if strings.ContainsAny(value, "+/=") {
			t.Fatalf("value %q is not unpadded base64url", value)
		}
	}
}
