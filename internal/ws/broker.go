package ws

import "sync"

type Message struct {
	Channel string `json:"channel"`
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type Subscription struct {
	C      <-chan Message
	cancel func()
}

func (s *Subscription) Close() { s.cancel() }

type Broker interface {
	Publish(channel string, msg Message)
	Subscribe(channel string) *Subscription
}

type InMemoryBroker struct {
	mu   sync.RWMutex
	subs map[string][]chan Message
}

func NewInMemoryBroker() *InMemoryBroker {
	return &InMemoryBroker{subs: make(map[string][]chan Message)}
}

func (b *InMemoryBroker) Publish(channel string, msg Message) {
	msg.Channel = channel
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs[channel] {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (b *InMemoryBroker) Subscribe(channel string) *Subscription {
	ch := make(chan Message, 64)
	b.mu.Lock()
	b.subs[channel] = append(b.subs[channel], ch)
	b.mu.Unlock()

	return &Subscription{
		C: ch,
		cancel: func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			subs := b.subs[channel]
			for i, s := range subs {
				if s == ch {
					b.subs[channel] = append(subs[:i], subs[i+1:]...)
					close(ch)
					return
				}
			}
		},
	}
}
