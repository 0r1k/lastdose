// Package tz resolves IANA time zone names: detected online from the public
// IP address, configured on the local machine, or entered by hand.
package tz

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalName is the sentinel for "whatever the machine uses" when the IANA
// name cannot be detected.
const LocalName = "Local"

// Common is a short list of zones offered in the picker besides the local one.
var Common = []string{
	"UTC",
	"Europe/Moscow",
	"Europe/Kyiv",
	"Europe/Minsk",
	"Europe/Bucharest",
	"Europe/Berlin",
	"Europe/London",
	"Asia/Almaty",
	"Asia/Tbilisi",
	"Asia/Yekaterinburg",
	"Asia/Novosibirsk",
	"Asia/Vladivostok",
	"America/New_York",
	"America/Los_Angeles",
}

// Detect returns the IANA name of the machine's zone, or LocalName.
func Detect() string {
	if env := strings.TrimPrefix(os.Getenv("TZ"), ":"); env != "" {
		if _, err := time.LoadLocation(env); err == nil {
			return env
		}
	}
	if p, err := filepath.EvalSymlinks("/etc/localtime"); err == nil {
		if i := strings.Index(p, "zoneinfo/"); i >= 0 {
			name := strings.TrimPrefix(p[i+len("zoneinfo/"):], "posix/")
			if _, err := time.LoadLocation(name); err == nil {
				return name
			}
		}
	}
	if b, err := os.ReadFile("/etc/timezone"); err == nil {
		name := strings.TrimSpace(string(b))
		if _, err := time.LoadLocation(name); err == nil {
			return name
		}
	}
	return LocalName
}

// lookupURLs return the caller's IANA zone, guessed from the public IP, as
// plain text.
var lookupURLs = []string{
	"https://ipinfo.io/timezone",
	"https://ipapi.co/timezone",
}

// Online detects the zone from the public IP address via geolocation services.
func Online(ctx context.Context) (string, error) {
	var errs []error
	for _, url := range lookupURLs {
		name, err := fetchZone(ctx, url)
		if err == nil {
			return name, nil
		}
		errs = append(errs, err)
	}
	return "", errors.Join(errs...)
}

func fetchZone(ctx context.Context, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(string(b))
	if _, err := time.LoadLocation(name); err != nil || name == "" || name == LocalName {
		return "", fmt.Errorf("%s: unexpected answer %q", url, name)
	}
	return name, nil
}

// Load resolves a stored name; empty and LocalName map to time.Local.
func Load(name string) (*time.Location, error) {
	if name == "" || name == LocalName {
		return time.Local, nil
	}
	return time.LoadLocation(name)
}
