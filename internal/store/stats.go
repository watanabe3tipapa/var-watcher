package store

import (
	"fmt"
	"time"
)

// StatsBucket は集計の 1 バケット。
type StatsBucket struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// StatsSource はエンジン(source)別の集計行。
type StatsSource struct {
	Source string `json:"source"`
	Count  int64  `json:"count"`
}

// StatsTopPath は変更が多いパスの集計行。
type StatsTopPath struct {
	Path  string `json:"path"`
	Count int64  `json:"count"`
}

// Stats はダッシュボード用の集計結果のまとまり。
type Stats struct {
	Total     int64          `json:"total"`
	Hourly    []StatsBucket  `json:"hourly"`
	Weekly    []StatsBucket  `json:"weekly"`
	BySource  []StatsSource  `json:"by_source"`
	TopPaths  []StatsTopPath `json:"top_paths"`
	WindowSec int            `json:"window_sec"`
}

// Stats(Slots) は最近 hours 時間の集計を返す。
func (s *Store) Stats(hours int) (Stats, error) {
	if hours <= 0 {
		hours = 24
	}
	now := time.Now()
	since := now.Add(-time.Duration(hours) * time.Hour)
	sinceMs := since.UnixMilli()

	out := Stats{WindowSec: hours * 3600}

	// 合計
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM logs WHERE ts >= ?`, sinceMs,
	).Scan(&out.Total); err != nil {
		return out, fmt.Errorf("store: stats total: %w", err)
	}

	// 時間帯別(24 バケット)
	hourly, err := s.aggregateHours(sinceMs)
	if err != nil {
		return out, err
	}
	out.Hourly = hourly

	// 曜日別(直近 hours 時間 = 日に満たない場合も 0 埋め)
	weekly, err := s.aggregateWeekday(sinceMs)
	if err != nil {
		return out, err
	}
	out.Weekly = weekly

	// source 別
	srcs, err := s.aggregateSource(sinceMs)
	if err != nil {
		return out, err
	}
	out.BySource = srcs

	// TOP 書き込みパス
	paths, err := s.aggregateTopPaths(sinceMs)
	if err != nil {
		return out, err
	}
	out.TopPaths = paths

	return out, nil
}

// aggregateHours は過去 24 時間を 1 時間バケットで集計する(0 埋め)。
func (s *Store) aggregateHours(sinceMs int64) ([]StatsBucket, error) {
	rows, err := s.db.Query(`
		SELECT (ts / 3600000) AS bucket, COUNT(*)
		FROM logs
		WHERE ts >= ?
		GROUP BY bucket
		ORDER BY bucket`, sinceMs)
	if err != nil {
		return nil, fmt.Errorf("store: stats hourly: %w", err)
	}
	defer rows.Close()

	byBucket := map[int64]int64{}
	for rows.Next() {
		var b, c int64
		if err := rows.Scan(&b, &c); err != nil {
			return nil, fmt.Errorf("store: stats hourly scan: %w", err)
		}
		byBucket[b] = c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	start := sinceMs / 3600000
	out := make([]StatsBucket, 0, 24)
	for i := int64(0); i < 24; i++ {
		b := start + i
		out = append(out, StatsBucket{
			Label: time.UnixMilli(b * 3600000).Format("15:00"),
			Count: byBucket[b],
		})
	}
	return out, nil
}

// aggregateWeekday は曜日別の合計(直近 7 日)を返す。
func (s *Store) aggregateWeekday(sinceMs int64) ([]StatsBucket, error) {
	rows, err := s.db.Query(`
		SELECT strftime('%w', ts / 1000, 'unixepoch') AS dow, COUNT(*)
		FROM logs
		WHERE ts >= ?
		GROUP BY dow
		ORDER BY dow`, sinceMs)
	if err != nil {
		return nil, fmt.Errorf("store: stats weekly: %w", err)
	}
	defer rows.Close()

	counts := map[string]int64{}
	for rows.Next() {
		var dow string
		var c int64
		if err := rows.Scan(&dow, &c); err != nil {
			return nil, fmt.Errorf("store: stats weekly scan: %w", err)
		}
		counts[dow] = c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	names := []string{"日", "月", "火", "水", "木", "金", "土"}
	out := make([]StatsBucket, 0, 7)
	for i, name := range names {
		out = append(out, StatsBucket{
			Label: name,
			Count: counts[fmt.Sprintf("%d", i)],
		})
	}
	return out, nil
}

// aggregateSource は source(エンジン)別の行数(DateOrder 降順)を返す。
func (s *Store) aggregateSource(sinceMs int64) ([]StatsSource, error) {
	rows, err := s.db.Query(`
		SELECT source, COUNT(*) AS c
		FROM logs
		WHERE ts >= ?
		GROUP BY source
		ORDER BY c DESC`, sinceMs)
	if err != nil {
		return nil, fmt.Errorf("store: stats source: %w", err)
	}
	defer rows.Close()

	out := []StatsSource{}
	for rows.Next() {
		var r StatsSource
		if err := rows.Scan(&r.Source, &r.Count); err != nil {
			return nil, fmt.Errorf("store: stats source scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// aggregateTopPaths は message の先頭パス部を集計して上位 10 件を返す。
func (s *Store) aggregateTopPaths(sinceMs int64) ([]StatsTopPath, error) {
	rows, err := s.db.Query(`
		SELECT substr(message, 1, instr(message, ' ') - 1) AS p, COUNT(*) AS c
		FROM logs
		WHERE ts >= ? AND message != ''
		GROUP BY p
		ORDER BY c DESC
		LIMIT 10`, sinceMs)
	if err != nil {
		return nil, fmt.Errorf("store: stats paths: %w", err)
	}
	defer rows.Close()

	out := []StatsTopPath{}
	for rows.Next() {
		var r StatsTopPath
		if err := rows.Scan(&r.Path, &r.Count); err != nil {
			return nil, fmt.Errorf("store: stats paths scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
