package api

import "net/http"

// tokenTransport injects access_token as a query parameter into every request.
// Baidu API uses ?access_token=xxx rather than Authorization header.
type tokenTransport struct {
	token string
	base  http.RoundTripper
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request per RoundTripper contract: must not modify the original.
	req2 := req.Clone(req.Context())
	q := req2.URL.Query()
	q.Set("access_token", t.token)
	req2.URL.RawQuery = q.Encode()
	return t.base.RoundTrip(req2)
}

// apiKeyTransport injects an API key as a query parameter into every request.
// Baidu API uses ?api_key=xxx as the API key authentication method.
type apiKeyTransport struct {
	apiKey string
	base   http.RoundTripper
}

func (t *apiKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request per RoundTripper contract: must not modify the original.
	req2 := req.Clone(req.Context())
	q := req2.URL.Query()
	q.Set("api_key", t.apiKey)
	req2.URL.RawQuery = q.Encode()
	return t.base.RoundTrip(req2)
}
