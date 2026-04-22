package ws

import (
	"os"
	"testing"
	"time"
)

// TestValkeyBrokerFanout verifies spec 107: during a rolling deploy with both slots running,
// WS events published via valkey reach subscribers on either slot.
func TestValkeyBrokerFanout(t *testing.T) {
	addr := os.Getenv("ZDX_VALKEY_ADDR")
	if addr == "" {
		t.Skip("ZDX_VALKEY_ADDR not set; skipping valkey integration test")
	}

	b1, err := NewValkeyBroker(addr)
	if err != nil {
		t.Fatalf("broker1 connect: %v", err)
	}
	defer b1.Close()

	b2, err := NewValkeyBroker(addr)
	if err != nil {
		t.Fatalf("broker2 connect: %v", err)
	}
	defer b2.Close()

	sub1 := b1.Subscribe("fanout-test")
	defer sub1.Close()
	sub2 := b2.Subscribe("fanout-test")
	defer sub2.Close()

	// Give both brokers time to establish their PSubscribe before publishing.
	time.Sleep(50 * time.Millisecond)

	b1.Publish("fanout-test", Message{Type: "ping", Payload: "hello"})

	timeout := time.After(2 * time.Second)
	received := 0
	for received < 2 {
		select {
		case msg := <-sub1.C:
			if msg.Type != "ping" {
				t.Fatalf("sub1 unexpected message: %+v", msg)
			}
			received++
		case msg := <-sub2.C:
			if msg.Type != "ping" {
				t.Fatalf("sub2 unexpected message: %+v", msg)
			}
			received++
		case <-timeout:
			t.Fatalf("timeout: only %d/2 subscribers received the event", received)
		}
	}
}
