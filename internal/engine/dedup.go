package engine

import (
	"container/list"
	"sync"
	"time"
)

// deduper は同一イベントの重複報告を時間ウィンドウ内で除去する LRU バッファ。
// 複数エンジン同時稼働時に同じファイル変更が複数回記録されるのを防ぐ。
type deduper struct {
	mu     sync.Mutex
	window time.Duration
	max    int
	ll     *list.List
	m      map[string]*list.Element
}

type dedupRec struct {
	key string
	ts  time.Time
}

// newDeduper は重複排除バッファを生成する。window <= 0 の場合は nil(無効)。
func newDeduper(window time.Duration, max int) *deduper {
	if window <= 0 {
		return nil
	}
	if max <= 0 {
		max = 4096
	}
	return &deduper{
		window: window,
		max:    max,
		ll:     list.New(),
		m:      make(map[string]*list.Element),
	}
}

// Allow は key が window 内に既に登録されていれば false を返して破棄し、
// それ以外は登録して true を返す。超過分は LRU 順に追い出す。
func (d *deduper) Allow(key string, now time.Time) bool {
	if d == nil {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	if el, ok := d.m[key]; ok {
		rec := el.Value.(*dedupRec)
		if now.Sub(rec.ts) < d.window {
			d.ll.MoveToFront(el)
			return false
		}
		rec.ts = now
		d.ll.MoveToFront(el)
		return true
	}

	if d.max > 0 && d.ll.Len() >= d.max {
		if back := d.ll.Back(); back != nil {
			d.ll.Remove(back)
			delete(d.m, back.Value.(*dedupRec).key)
		}
	}
	el := d.ll.PushFront(&dedupRec{key: key, ts: now})
	d.m[key] = el
	return true
}
