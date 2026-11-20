// examples/auth demonstrates the only supported developer authorization flow:
// directory consent, a loopback callback, and operating-system credential
// storage.
//
// Usage:
//
//	BAIDU_APP_KEY=xxx BAIDU_SECRET_KEY=yyy \
//	BAIDU_REDIRECT_URI=http://127.0.0.1:8787/oauth/callback \
//	go run ./examples/auth
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/auth"
)

func main() {
	config, err := configFromEnv(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if err := run(ctx, config, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func configFromEnv(getenv func(string) string) (auth.Config, error) {
	config := auth.Config{
		AppKey:      getenv("BAIDU_APP_KEY"),
		SecretKey:   getenv("BAIDU_SECRET_KEY"),
		RedirectURI: getenv("BAIDU_REDIRECT_URI"),
		OpenBrowser: true,
	}
	if config.AppKey == "" || config.SecretKey == "" || config.RedirectURI == "" {
		return auth.Config{}, fmt.Errorf("set BAIDU_APP_KEY, BAIDU_SECRET_KEY, and BAIDU_REDIRECT_URI")
	}
	return config, nil
}

func run(ctx context.Context, config auth.Config, output io.Writer) error {
	session, err := auth.Begin(ctx, config)
	if err != nil {
		return err
	}
	if session.AuthorizationURL != "" {
		if session.BrowserOpenError != nil {
			fmt.Fprintf(output, "Could not open the system browser automatically: %v\n", session.BrowserOpenError)
		} else {
			fmt.Fprintln(output, "Authorization URL sent to the system browser.")
		}
		fmt.Fprintln(output, "Manual fallback URL:")
		fmt.Fprintln(output, session.AuthorizationURL)
		fmt.Fprintln(output, "Waiting for the registered loopback callback...")
	} else {
		fmt.Fprintln(output, "Using the valid credential in the operating system store.")
	}

	result, err := session.Wait(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "Authorization ready; access expires at %s.\n", result.Credential.ExpiresAt.Format(time.RFC3339))
	return nil
}
