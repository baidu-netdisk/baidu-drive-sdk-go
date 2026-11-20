package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/auth"
)

func TestConfigFromEnv(t *testing.T) {
	values := map[string]string{
		"BAIDU_APP_KEY":      "app-key",
		"BAIDU_SECRET_KEY":   "secret-key",
		"BAIDU_REDIRECT_URI": "http://127.0.0.1:8787/oauth/callback",
	}
	config, err := configFromEnv(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if config.AppKey != "app-key" || config.SecretKey != "secret-key" || config.RedirectURI != values["BAIDU_REDIRECT_URI"] || !config.OpenBrowser {
		t.Fatalf("config = %+v", config)
	}
}

func TestConfigFromEnvMissing(t *testing.T) {
	if _, err := configFromEnv(func(string) string { return "" }); err == nil {
		t.Fatal("expected missing configuration error")
	}
}

func TestRunUsesStoredCredential(t *testing.T) {
	store := auth.NewMemoryStore()
	credential := &auth.Credential{
		AccessToken: "access-token",
		ExpiresAt:   time.Now().Add(time.Hour),
		Scope:       "basic netdisk",
		CreatedAt:   time.Now(),
	}
	if err := store.Save(context.Background(), "example", credential); err != nil {
		t.Fatal(err)
	}
	config := auth.Config{
		AppKey:      "app-key",
		SecretKey:   "secret-key",
		RedirectURI: "http://127.0.0.1:8787/oauth/callback",
		Store:       store,
		StoreKey:    "example",
	}
	var output bytes.Buffer
	if err := run(context.Background(), config, &output); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte("operating system store")) {
		t.Fatalf("output = %q", output.String())
	}
}
