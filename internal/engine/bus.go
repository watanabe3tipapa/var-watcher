package engine

import (
	"sync"
	"time"
)

type LogLine struct {
	TS      time.Time `json:"ts"`
	Source  string    `json:"source"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type Bus struct {
	mu   sync.Mutex
	subs map[chan LogLine]struct{}
}

func NewBus() *Bus {
	return &Bus{subs: make(map[chan LogLine]struct{})}
}

func (b *Bus) Publish(line LogLine) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- line:
		default:
			// バッファ枯渇時は最古を捨てて入れる(購読者を詰まらせない)
			select {
			case <-ch:
			default:
			}
			ch <- line
		}
	}
}

func (b *Bus) Subscribe() (<-chan LogLine, func()) {
	ch := make(chan LogLine, 512)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	unsub := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs, ch)
	}
	return ch, unsub
}
