// Package auth implements the developer-facing Baidu Netdisk directory
// authorization flow. It starts a loopback callback server, validates the
// OAuth response, receives tokens or exchanges a one-time code, and stores the credential.
package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

const (
	defaultAuthorizationEndpoint = "https://pan.baidu.com/union/oauth/authorize"
	defaultTokenEndpoint         = "https://pan.baidu.com/xpan/oauth/get"
	defaultScope                 = "basic,netdisk"
	defaultKeyringService        = "com.baidu.netdisk.openapi-go"
	defaultHTTPTimeout           = 30 * time.Second
	defaultExpirySkew            = 5 * time.Minute
)

// Config contains the application settings registered in the Baidu Netdisk
// developer console.
type Config struct {
	AppKey      string
	SecretKey   string
	RedirectURI string
	// OpenBrowser asks Begin to launch the authorization URL in the system's
	// default external browser after the loopback listener is ready. A launch
	// failure is reported through Session.BrowserOpenError while the URL remains
	// available for manual use.
	OpenBrowser bool
	// BrowserOpener overrides the system browser launcher. It is primarily
	// useful for host integration and deterministic tests.
	BrowserOpener BrowserOpener

	// Debug enables security-redacted diagnostics for the authorization
	// lifecycle. It does not enable debug logging on the returned API client.
	Debug bool
	// Logger receives authorization diagnostics when Debug is true. When nil,
	// diagnostics are written to standard error. The SDK never logs credentials,
	// authorization codes, PKCE values, or complete OAuth URLs.
	Logger io.Writer

	// Store defaults to the operating system credential store. Tests and
	// short-lived tools can inject a different implementation explicitly.
	Store CredentialStore
	// StoreKey separates multiple users of the same application. When empty,
	// a stable key is derived from AppKey, RedirectURI, and Scope.
	StoreKey string
	// KeyringService overrides the operating system credential service name
	// when Store is nil.
	KeyringService string

	// Scope defaults to "basic,netdisk".
	Scope string
	// ExpirySkew controls how early a stored credential is treated as expired.
	// Zero uses five minutes.
	ExpirySkew time.Duration

	// AuthorizationEndpoint and TokenEndpoint are intended for compatible
	// Baidu environments and deterministic tests. Production endpoints must
	// use HTTPS; HTTP is accepted only for a loopback test server.
	AuthorizationEndpoint string
	TokenEndpoint         string
	OAuthHTTPClient       *http.Client

	// ClientOptions are applied before the newly issued access token when
	// constructing Result.Client.
	ClientOptions []api.Option
}

type normalizedConfig struct {
	appKey                string
	secretKey             string
	redirectURI           string
	redirectURL           *url.URL
	listenAddress         string
	store                 CredentialStore
	storeKey              string
	scope                 string
	expirySkew            time.Duration
	authorizationEndpoint *url.URL
	tokenEndpoint         *url.URL
	httpClient            *http.Client
	clientOptions         []api.Option
	openBrowser           bool
	browserOpener         BrowserOpener
	diagnostics           *diagnosticLogger
}

func normalizeConfig(ctx context.Context, cfg Config) (normalizedConfig, error) {
	if ctx == nil {
		return normalizedConfig{}, errors.New("baidupan auth: context must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return normalizedConfig{}, err
	}

	appKey := strings.TrimSpace(cfg.AppKey)
	secretKey := strings.TrimSpace(cfg.SecretKey)
	if appKey == "" || secretKey == "" {
		return normalizedConfig{}, errors.New("baidupan auth: AppKey and SecretKey are required")
	}

	redirectURL, listenAddress, err := parseLoopbackRedirectURI(cfg.RedirectURI)
	if err != nil {
		return normalizedConfig{}, err
	}
	authorizationEndpoint, err := parseEndpoint(cfg.AuthorizationEndpoint, defaultAuthorizationEndpoint, "authorization")
	if err != nil {
		return normalizedConfig{}, err
	}
	tokenEndpoint, err := parseEndpoint(cfg.TokenEndpoint, defaultTokenEndpoint, "token")
	if err != nil {
		return normalizedConfig{}, err
	}

	scope := strings.TrimSpace(cfg.Scope)
	if scope == "" {
		scope = defaultScope
	}
	if strings.ContainsAny(scope, "\r\n") {
		return normalizedConfig{}, errors.New("baidupan auth: Scope must not contain line breaks")
	}

	expirySkew := cfg.ExpirySkew
	if expirySkew == 0 {
		expirySkew = defaultExpirySkew
	}
	if expirySkew < 0 {
		return normalizedConfig{}, errors.New("baidupan auth: ExpirySkew must not be negative")
	}

	store := cfg.Store
	if store == nil {
		service := strings.TrimSpace(cfg.KeyringService)
		if service == "" {
			service = defaultKeyringService
		}
		store, err = NewSystemStore(service)
		if err != nil {
			return normalizedConfig{}, err
		}
	}
	storeKey := strings.TrimSpace(cfg.StoreKey)
	if storeKey == "" {
		storeKey = credentialStoreKey(appKey, redirectURL.String(), scope)
	}
	if len(storeKey) > 256 || strings.ContainsAny(storeKey, "\r\n") {
		return normalizedConfig{}, errors.New("baidupan auth: StoreKey must be at most 256 characters without line breaks")
	}

	httpClient := cfg.OAuthHTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	httpClientCopy := *httpClient
	httpClientCopy.Jar = nil
	httpClientCopy.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return normalizedConfig{
		appKey:                appKey,
		secretKey:             secretKey,
		redirectURI:           redirectURL.String(),
		redirectURL:           redirectURL,
		listenAddress:         listenAddress,
		store:                 store,
		storeKey:              storeKey,
		scope:                 scope,
		expirySkew:            expirySkew,
		authorizationEndpoint: authorizationEndpoint,
		tokenEndpoint:         tokenEndpoint,
		httpClient:            &httpClientCopy,
		clientOptions:         append([]api.Option(nil), cfg.ClientOptions...),
		openBrowser:           cfg.OpenBrowser,
		browserOpener:         cfg.BrowserOpener,
		diagnostics:           newDiagnosticLogger(cfg.Debug, cfg.Logger),
	}, nil
}

func parseLoopbackRedirectURI(raw string) (*url.URL, string, error) {
	redirectURI := strings.TrimSpace(raw)
	u, err := url.Parse(redirectURI)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil {
		return nil, "", errors.New("baidupan auth: RedirectURI must be an HTTP loopback URL without credentials")
	}
	if u.RawQuery != "" || u.ForceQuery || strings.Contains(redirectURI, "#") || u.RawPath != "" {
		return nil, "", errors.New("baidupan auth: RedirectURI must not contain an encoded path, query, or fragment")
	}
	host := strings.ToLower(u.Hostname())
	if !isLoopbackHost(host) {
		return nil, "", errors.New("baidupan auth: RedirectURI host must be localhost, 127.0.0.1 or ::1")
	}
	port := u.Port()
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, "", errors.New("baidupan auth: RedirectURI must include a port between 1 and 65535")
	}
	if u.Path == "" || u.Path == "/" {
		return nil, "", errors.New("baidupan auth: RedirectURI must include a callback path")
	}
	if host == "localhost" {
		host = "127.0.0.1"
	}
	return u, net.JoinHostPort(host, strconv.Itoa(portNumber)), nil
}

func parseEndpoint(raw, fallback, name string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = fallback
	}
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return nil, fmt.Errorf("baidupan auth: %s endpoint must be an absolute URL without credentials, query, or fragment", name)
	}
	if u.Scheme != "https" {
		loopback := u.Scheme == "http" && isLoopbackHost(u.Hostname())
		if !loopback {
			return nil, fmt.Errorf("baidupan auth: %s endpoint must use HTTPS", name)
		}
	}
	return u, nil
}

func isLoopbackHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
