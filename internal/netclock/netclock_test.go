package netclock

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"
)

func fixed(t time.Time, precise bool) Source {
	return func(context.Context) (Sample, error) {
		return Sample{Net: t, Local: time.Now(), Precise: precise, Source: "fake"}, nil
	}
}

func failing(context.Context) (Sample, error) { return Sample{}, errors.New("down") }

func TestNowFollowsNetworkNotSystem(t *testing.T) {
	net := time.Date(2031, 5, 6, 7, 8, 9, 0, time.UTC)
	c := New(fixed(net, true))
	if _, ok := c.Now(); ok {
		t.Fatal("unsynced clock must not report time")
	}
	if err := c.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	now, ok := c.Now()
	if !ok || now.Sub(net) < 0 || now.Sub(net) > time.Second {
		t.Fatalf("now = %v, want ~%v", now, net)
	}
	if off := c.Status().Offset; off < 4*365*24*time.Hour {
		t.Fatalf("offset to the system clock should be years, got %v", off)
	}
}

func TestMedianAndPrecisePreferred(t *testing.T) {
	base := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	c := New(
		fixed(base, true),
		fixed(base.Add(time.Hour), true), // a liar
		fixed(base.Add(2*time.Second), true),
		fixed(base.Add(-48*time.Hour), false), // coarse, ignored
		failing,
	)
	if err := c.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	now, _ := c.Now()
	if d := now.Sub(base); d < 2*time.Second || d > 3*time.Second {
		t.Fatalf("median not used: %v from base", d)
	}
}

func TestAllSourcesFail(t *testing.T) {
	c := New(failing, failing)
	if err := c.Sync(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if _, ok := c.Now(); ok {
		t.Fatal("clock must stay unsynced")
	}
}

func TestNTPAgainstLocalServer(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer pc.Close()
	serverTime := time.Date(2029, 2, 3, 4, 5, 6, 500_000_000, time.UTC)
	go func() {
		buf := make([]byte, 48)
		_, addr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		resp := make([]byte, 48)
		resp[0] = 0x24 // version 4, mode 4 (server)
		resp[1] = 2    // stratum
		put := func(b []byte, t time.Time) {
			binary.BigEndian.PutUint32(b[0:4], uint32(t.Unix()+ntpEpochOffset))
			binary.BigEndian.PutUint32(b[4:8], uint32((int64(t.Nanosecond())<<32)/1e9))
		}
		put(resp[32:40], serverTime)
		put(resp[40:48], serverTime)
		pc.WriteTo(resp, addr)
	}()
	s, err := NTP(pc.LocalAddr().String())(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if d := s.Net.Sub(serverTime); d < 0 || d > 100*time.Millisecond {
		t.Fatalf("ntp time %v, want ~%v", s.Net, serverTime)
	}
}
