package zdxclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRecordFlushesBatch(t *testing.T) {
	var (
		mu       sync.Mutex
		received []batch
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Errorf("bad auth header: %q", got)
		}
		b, _ := io.ReadAll(r.Body)
		var payload batch
		if err := json.Unmarshal(b, &payload); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		mu.Lock()
		received = append(received, payload)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"accepted":` + itoa(len(payload.Events)) + `}`))
	}))
	defer srv.Close()

	c, err := New(Config{
		Endpoint:      srv.URL,
		Token:         "secret-token",
		Component:     "tester",
		Environment:   "unit",
		Host:          "host-A",
		FlushInterval: 50 * time.Millisecond,
		MaxBatch:      2,
		BufferSize:    32,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(context.Background())

	c.Record("op.a", 12*time.Millisecond, Tags{"tag": "x"})
	c.Record("op.b", 7*time.Millisecond, nil) // triggers MaxBatch flush
	c.Record("op.c", 3*time.Millisecond, nil) // picked up by timer

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		mu.Lock()
		total := 0
		for _, p := range received {
			total += len(p.Events)
		}
		mu.Unlock()
		if total == 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	var allEvents []event
	for _, p := range received {
		if p.Component != "tester" || p.Environment != "unit" || p.Host != "host-A" {
			t.Errorf("batch defaults wrong: %+v", p)
		}
		allEvents = append(allEvents, p.Events...)
	}
	if len(allEvents) != 3 {
		t.Fatalf("want 3 events got %d (%v)", len(allEvents), received)
	}
}

func TestOverflowDrops(t *testing.T) {
	// Server that hangs forever so nothing drains.
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)

	c, err := New(Config{
		Endpoint:      srv.URL,
		Token:         "t",
		FlushInterval: time.Hour, // ensure only MaxBatch triggers flushes
		MaxBatch:      4,
		BufferSize:    4,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Use a short deadline: the server blocks forever, so Close would hang
	// until close(block) fires (a later defer). A 3s deadline prevents
	// the test from taking 30+ seconds in slow CI environments.
	closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer closeCancel()
	defer c.Close(closeCtx)

	// Pump way more than capacity: buffer (4) + in-flight batch (4) ≈ 8 survive;
	// the rest must be counted as dropped.
	for i := 0; i < 200; i++ {
		c.Record("op", time.Millisecond, nil)
	}
	time.Sleep(50 * time.Millisecond)

	if c.Dropped() == 0 {
		t.Fatal("expected Dropped > 0, got 0")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c, err := New(Config{Endpoint: srv.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
