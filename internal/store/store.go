package store

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Line は永続化する 1 イベント。engine.LogLine と同型の転写で、store は engine に依存しない。
type Line struct {
	ID      int64     `json:"id"`
	TS      time.Time `json:"ts"`
	Source  string    `json:"source"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// Query は /api/logs の検索条件。
type Query struct {
	Since   *time.Time
	Until   *time.Time
	Source  string
	Keyword string
	Limit   int
}

type Store struct {
	db *sql.DB
}

const (
	defaultLimit = 200
	maxLimit     = 5000
)

// Open は SQLite を開きスキーマを初期化する。親ディレクトリが無ければ作成する。
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("store: create dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(8)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: wal: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: busy_timeout: %w", err)
	}
	s := &Store{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) init() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS logs (
			id      INTEGER PRIMARY KEY AUTOINCREMENT,
			ts      INTEGER NOT NULL,
			source  TEXT NOT NULL,
			level   TEXT NOT NULL,
			message TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_logs_ts     ON logs(ts);
		CREATE INDEX IF NOT EXISTS idx_logs_source ON logs(source);
	`); err != nil {
		return fmt.Errorf("store: init: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Append(line Line) error {
	_, err := s.db.Exec(
		`INSERT INTO logs (ts, source, level, message) VALUES (?, ?, ?, ?)`,
		line.TS.UnixMilli(), line.Source, line.Level, line.Message,
	)
	if err != nil {
		return fmt.Errorf("store: append: %w", err)
	}
	return nil
}

// Prune は olderThan より古い行を削除し、削除件数を返す。
func (s *Store) Prune(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).UnixMilli()
	res, err := s.db.Exec(`DELETE FROM logs WHERE ts < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("store: prune: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// buildWhere は Query から WHERE 節とバインド引数を組み立てる。
func buildWhere(q Query) (string, []any) {
	where := []string{}
	args := []any{}
	if q.Since != nil {
		where = append(where, "ts >= ?")
		args = append(args, q.Since.UnixMilli())
	}
	if q.Until != nil {
		where = append(where, "ts <= ?")
		args = append(args, q.Until.UnixMilli())
	}
	if q.Source != "" {
		where = append(where, "source = ?")
		args = append(args, q.Source)
	}
	if q.Keyword != "" {
		where = append(where, "message LIKE ? ESCAPE '\\'")
		args = append(args, "%"+escapeLike(q.Keyword)+"%")
	}
	if len(where) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

// Query は条件に一致するログを新しい順(ts DESC, id DESC)で返す。
func (s *Store) Query(q Query) ([]Line, error) {
	clause, args := buildWhere(q)
	limit := q.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	qry := `SELECT id, ts, source, level, message FROM logs`
	if clause != "" {
		qry += clause
	}
	qry += ` ORDER BY ts DESC, id DESC LIMIT ?`
	args = append(args, limit)

	return s.query(qry, args...)
}

// query は取得クエリを実行し、行を新しい順でスキャンして返す。
func (s *Store) query(qry string, args ...any) ([]Line, error) {
	rows, err := s.db.Query(qry, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query: %w", err)
	}
	defer rows.Close()

	out := []Line{}
	for rows.Next() {
		var l Line
		var ms int64
		if err := rows.Scan(&l.ID, &ms, &l.Source, &l.Level, &l.Message); err != nil {
			return nil, fmt.Errorf("store: scan: %w", err)
		}
		l.TS = time.UnixMilli(ms)
		out = append(out, l)
	}
	return out, rows.Err()
}

// ExportAll は条件に一致する全ログを古い順(ts ASC)で返す。エクスポート用。
func (s *Store) ExportAll(q Query) ([]Line, error) {
	clause, args := buildWhere(q)
	qry := `SELECT id, ts, source, level, message FROM logs`
	if clause != "" {
		qry += clause
	}
	qry += ` ORDER BY ts ASC, id ASC`
	return s.query(qry, args...)
}

// WriteJSON は行を JSON 配列として書き込む。エクスポート用。
func (s *Store) WriteJSON(w io.Writer, lines []Line) error {
	enc := json.NewEncoder(w)
	return enc.Encode(lines)
}

// WriteCSV は行を CSV(ヘッダー付き)として書き込む。エクスポート用。
func (s *Store) WriteCSV(w io.Writer, lines []Line) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{"ts", "source", "level", "message"}); err != nil {
		return fmt.Errorf("store: csv header: %w", err)
	}
	for _, l := range lines {
		if err := cw.Write([]string{
			l.TS.UTC().Format(time.RFC3339Nano),
			l.Source,
			l.Level,
			l.Message,
		}); err != nil {
			return fmt.Errorf("store: csv row: %w", err)
		}
	}
	return cw.Error()
}

// escapeLike は LIKE の特殊文字をエスケープする。
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
