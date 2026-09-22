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
	mu    sync.Mutex
	subs  map[chan LogLine]struct{}
	dedup *deduper
}

func NewBus() *Bus {
	return &Bus{subs: make(map[chan LogLine]struct{})}
}

// EnableDedup は重複排除を設定する。window <= 0 のとき無効化する。
func (b *Bus) EnableDedup(window time.Duration, max int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dedup = newDeduper(window, max)
}

func (b *Bus) Publish(line LogLine) {
	if line.TS.IsZero() {
		line.TS = time.Now()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.dedup != nil && !b.dedup.Allow(dedupKey(line), line.TS) {
		return
	}
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

// dedupKey は重複判定のキーを組み立てる。同一の Level + Message を同一イベントとみなす。
func dedupKey(line LogLine) string {
	return line.Level + "\x00" + line.Message
}
