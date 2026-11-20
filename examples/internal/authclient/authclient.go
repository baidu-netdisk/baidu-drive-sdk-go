// Package authclient keeps the runnable examples focused on their API call.
// The complete two-call authorization flow is shown in examples/auth.
package authclient

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/auth"
)

// New loads a stored developer credential or waits for directory consent.
func New(ctx context.Context, getenv func(string) string, output io.Writer, options ...api.Option) (*api.Client, error) {
	config := auth.Config{
		AppKey:        getenv("BAIDU_APP_KEY"),
		SecretKey:     getenv("BAIDU_SECRET_KEY"),
		RedirectURI:   getenv("BAIDU_REDIRECT_URI"),
		OpenBrowser:   true,
		ClientOptions: options,
	}
	if config.AppKey == "" || config.SecretKey == "" || config.RedirectURI == "" {
		return nil, fmt.Errorf("set BAIDU_APP_KEY, BAIDU_SECRET_KEY, and BAIDU_REDIRECT_URI")
	}
	authContext, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	session, err := auth.Begin(authContext, config)
	if err != nil {
		return nil, err
	}
	if session.AuthorizationURL != "" {
		if session.BrowserOpenError != nil {
			fmt.Fprintf(output, "Could not open the system browser automatically: %v\n", session.BrowserOpenError)
		} else {
			fmt.Fprintln(output, "Authorization URL sent to the system browser.")
		}
		fmt.Fprintln(output, "Manual fallback URL:")
		fmt.Fprintln(output, session.AuthorizationURL)
	}
	result, err := session.Wait(authContext)
	if err != nil {
		return nil, err
	}
	return result.Client, nil
}
