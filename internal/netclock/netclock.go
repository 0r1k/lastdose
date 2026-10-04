// Package netclock provides trusted time taken from the network instead of
// the system clock, so changing the computer's date cannot fake progress.
//
// After a sync the clock advances using Go's monotonic clock, which is not
// affected by wall-clock changes. Resyncs fix drift and time lost while the
// machine was suspended (the monotonic clock stops during suspend on Linux).
package netclock

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Sample is one network time reading.
type Sample struct {
	// Net is the network time at the instant Local was taken.
	Net time.Time
	// Local is time.Now() at that instant; it carries a monotonic reading.
	Local time.Time
	// Precise is false for second-resolution sources such as HTTP Date.
	Precise bool
	Source  string
}

// Source fetches one sample.
type Source func(ctx context.Context) (Sample, error)

// Status describes the last successful sync.
type Status struct {
	Synced bool
	Source string
	// Offset is network time minus the system clock at sync time.
	Offset time.Duration
	// At is the local (monotonic) moment of the sync.
	At time.Time
}

// Clock is safe for concurrent use.
type Clock struct {
	sources []Source

	mu     sync.Mutex
	base   time.Time // network time at mono
	mono   time.Time // local time.Now() matching base
	status Status
}

// DefaultSources queries public NTP servers, with HTTPS Date headers as a
// fallback for networks that block UDP port 123.
func DefaultSources() []Source {
	return []Source{
		NTP("time.cloudflare.com"),
		NTP("time.google.com"),
		NTP("pool.ntp.org"),
		HTTPDate("https://www.cloudflare.com"),
		HTTPDate("https://www.google.com"),
	}
}

// New returns an unsynced clock using the given sources.
func New(sources ...Source) *Clock { return &Clock{sources: sources} }

// Now returns trusted time, or false if the clock has never synced.
func (c *Clock) Now() (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.status.Synced {
		return time.Time{}, false
	}
	return c.base.Add(time.Since(c.mono)), true
}

// Status reports the last successful sync.
func (c *Clock) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

// NeedsResync reports whether the wall clock and the monotonic clock diverged
// since the last sync: the system time was changed or the machine slept.
func (c *Clock) NeedsResync() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.status.Synced {
		return true
	}
	now := time.Now()
	wall := now.Round(0).Sub(c.mono.Round(0))
	mono := now.Sub(c.mono)
	return (wall - mono).Abs() > 2*time.Second
}

// Sync queries all sources concurrently and adopts the median of the precise
// samples (or of all samples if no precise source answered).
func (c *Clock) Sync(ctx context.Context) error {
	if len(c.sources) == 0 {
		return errors.New("no time sources configured")
	}
	type result struct {
		s   Sample
		err error
	}
	ch := make(chan result, len(c.sources))
	for _, src := range c.sources {
		go func() {
			s, err := src(ctx)
			ch <- result{s, err}
		}()
	}
	var precise, coarse []Sample
	var errs []string
	for range c.sources {
		r := <-ch
		switch {
		case r.err != nil:
			errs = append(errs, r.err.Error())
		case r.s.Precise:
			precise = append(precise, r.s)
		default:
			coarse = append(coarse, r.s)
		}
	}
	samples := precise
	if len(samples) == 0 {
		samples = coarse
	}
	if len(samples) == 0 {
		return fmt.Errorf("no time server reachable (%s)", strings.Join(errs, "; "))
	}

	// Project every sample onto one local instant before comparing them.
	ref := time.Now()
	sort.Slice(samples, func(i, j int) bool {
		return project(samples[i], ref).Before(project(samples[j], ref))
	})
	best := samples[len(samples)/2]

	c.mu.Lock()
	defer c.mu.Unlock()
	c.base, c.mono = best.Net, best.Local
	c.status = Status{
		Synced: true,
		Source: best.Source,
		Offset: best.Net.Sub(best.Local.Round(0)),
		At:     best.Local,
	}
	return nil
}

func project(s Sample, ref time.Time) time.Time { return s.Net.Add(ref.Sub(s.Local)) }

// ntpEpochOffset is the number of seconds between 1900-01-01 and 1970-01-01.
const ntpEpochOffset = 2208988800

// NTP queries an SNTP server (RFC 4330); host may include a port.
func NTP(host string) Source {
	addr := host
	if _, _, err := net.SplitHostPort(host); err != nil {
		addr = net.JoinHostPort(host, "123")
	}
	return func(ctx context.Context) (Sample, error) {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		var d net.Dialer
		conn, err := d.DialContext(ctx, "udp", addr)
		if err != nil {
			return Sample{}, fmt.Errorf("%s: %w", host, err)
		}
		defer conn.Close()
		if dl, ok := ctx.Deadline(); ok {
			_ = conn.SetDeadline(dl)
		}

		req := make([]byte, 48)
		req[0] = 0x23 // LI=0, version 4, mode 3 (client)
		t1 := time.Now()
		if _, err := conn.Write(req); err != nil {
			return Sample{}, fmt.Errorf("%s: %w", host, err)
		}
		resp := make([]byte, 48)
		n, err := conn.Read(resp)
		t4 := time.Now()
		if err != nil {
			return Sample{}, fmt.Errorf("%s: %w", host, err)
		}
		if n < 48 || resp[0]&0x07 != 4 || resp[1] == 0 || resp[1] > 15 {
			return Sample{}, fmt.Errorf("%s: bad NTP response", host)
		}
		t2 := ntpTime(resp[32:40]) // server receive
		t3 := ntpTime(resp[40:48]) // server transmit
		if t3.IsZero() {
			return Sample{}, fmt.Errorf("%s: empty NTP timestamp", host)
		}
		// Network delay excluding server processing; half of it is the trip back.
		delay := t4.Sub(t1) - t3.Sub(t2)
		if delay < 0 {
			delay = 0
		}
		return Sample{Net: t3.Add(delay / 2), Local: t4, Precise: true, Source: host}, nil
	}
}

func ntpTime(b []byte) time.Time {
	sec := binary.BigEndian.Uint32(b[0:4])
	frac := binary.BigEndian.Uint32(b[4:8])
	if sec == 0 && frac == 0 {
		return time.Time{}
	}
	nsec := (int64(frac) * 1e9) >> 32
	return time.Unix(int64(sec)-ntpEpochOffset, nsec).UTC()
}

// HTTPDate reads the Date header of an HTTPS response (1 second resolution).
func HTTPDate(url string) Source {
	return func(ctx context.Context) (Sample, error) {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
		if err != nil {
			return Sample{}, err
		}
		t1 := time.Now()
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return Sample{}, err
		}
		resp.Body.Close()
		t4 := time.Now()
		date, err := http.ParseTime(resp.Header.Get("Date"))
		if err != nil {
			return Sample{}, fmt.Errorf("%s: no usable Date header", url)
		}
		// The header is truncated to whole seconds: add half a second plus
		// half the round trip as the best estimate.
		net := date.Add(500*time.Millisecond + t4.Sub(t1)/2)
		host := strings.TrimPrefix(url, "https://")
		return Sample{Net: net, Local: t4, Source: host}, nil
	}
}
