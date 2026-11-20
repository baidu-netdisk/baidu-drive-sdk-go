package auth

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// BrowserOpener starts an external user-agent for an authorization URL.
// Implementations must return promptly rather than waiting for the browser to
// exit.
type BrowserOpener func(ctx context.Context, authorizationURL string) error

func openSystemBrowser(ctx context.Context, authorizationURL string) error {
	if ctx == nil {
		return errors.New("context must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(authorizationURL) == "" {
		return errors.New("authorization URL must not be empty")
	}

	name, args, err := systemBrowserCommand(runtime.GOOS, authorizationURL)
	if err != nil {
		return err
	}
	command := exec.Command(name, args...)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func systemBrowserCommand(goos, authorizationURL string) (string, []string, error) {
	switch goos {
	case "darwin":
		return "open", []string{authorizationURL}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", authorizationURL}, nil
	case "linux", "freebsd", "netbsd", "openbsd":
		return "xdg-open", []string{authorizationURL}, nil
	default:
		return "", nil, fmt.Errorf("unsupported operating system %q", goos)
	}
}
