package ws

import (
	"testing"
	"time"
)

func TestInMemoryBrokerPubSub(t *testing.T) {
	b := NewInMemoryBroker()
	sub := b.Subscribe("test")
	defer sub.Close()

	b.Publish("test", Message{Type: "ping", Payload: "hello"})

	select {
	case msg := <-sub.C:
		if msg.Type != "ping" || msg.Channel != "test" {
			t.Fatalf("unexpected message: %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestInMemoryBrokerUnsubscribe(t *testing.T) {
	b := NewInMemoryBroker()
	sub := b.Subscribe("ch")
	sub.Close()

	b.Publish("ch", Message{Type: "x"})
	// no panic, no deadlock
}

func TestInMemoryBrokerMultiSub(t *testing.T) {
	b := NewInMemoryBroker()
	s1 := b.Subscribe("ch")
	s2 := b.Subscribe("ch")
	defer s1.Close()
	defer s2.Close()

	b.Publish("ch", Message{Type: "m"})

	for _, s := range []*Subscription{s1, s2} {
		select {
		case msg := <-s.C:
			if msg.Type != "m" {
				t.Fatalf("unexpected: %+v", msg)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout")
		}
	}
}
