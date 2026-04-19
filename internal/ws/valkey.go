package ws

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

type ValkeyBroker struct {
	rdb  *redis.Client
	mem  *InMemoryBroker
	ctx  context.Context
	stop context.CancelFunc
}

func NewValkeyBroker(addr string) (*ValkeyBroker, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	b := &ValkeyBroker{
		rdb:  rdb,
		mem:  NewInMemoryBroker(),
		ctx:  ctx,
		stop: cancel,
	}
	go b.listen()
	return b, nil
}

func (b *ValkeyBroker) Publish(channel string, msg Message) {
	msg.Channel = channel
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	b.rdb.Publish(b.ctx, "zdx:ws:"+channel, data)
}

func (b *ValkeyBroker) Subscribe(channel string) *Subscription {
	return b.mem.Subscribe(channel)
}

func (b *ValkeyBroker) listen() {
	ps := b.rdb.PSubscribe(b.ctx, "zdx:ws:*")
	defer ps.Close()
	ch := ps.Channel()
	for {
		select {
		case <-b.ctx.Done():
			return
		case m, ok := <-ch:
			if !ok {
				return
			}
			var msg Message
			if err := json.Unmarshal([]byte(m.Payload), &msg); err != nil {
				continue
			}
			channel := m.Channel[len("zdx:ws:"):]
			b.mem.Publish(channel, msg)
		}
	}
}

func (b *ValkeyBroker) Close() {
	b.stop()
	b.rdb.Close()
}

func NewBroker(valkeyAddr string) Broker {
	if valkeyAddr == "" {
		return NewInMemoryBroker()
	}
	vb, err := NewValkeyBroker(valkeyAddr)
	if err != nil {
		log.Printf("ws: valkey connection failed (%s), falling back to in-memory broker: %v", valkeyAddr, err)
		return NewInMemoryBroker()
	}
	log.Printf("ws: using valkey broker at %s", valkeyAddr)
	return vb
}
