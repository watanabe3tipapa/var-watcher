package web

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/watanabe3tipapa/var-watcher/internal/alert"
	"github.com/watanabe3tipapa/var-watcher/internal/engine"
	"github.com/watanabe3tipapa/var-watcher/internal/store"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Server struct {
	engine *engine.Engine
	store  *store.Store
	alerts *alert.Manager
	static fs.FS
}

func NewServer(e *engine.Engine) *Server {
	return &Server{engine: e}
}

// SetStore はログ永続化ストアを登録する。/api/logs は store が無いと無効を返す。
func (s *Server) SetStore(st *store.Store) {
	s.store = st
}

// SetAlerts はアラートマネージャを登録する。/api/alerts のフィードに使う。
func (s *Server) SetAlerts(am *alert.Manager) {
	s.alerts = am
}

func (s *Server) Run(addr string) error {
	srv := &http.Server{Addr: addr, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	return srv.ListenAndServe()
}

// Handler はルーティング済みの http.Handler を返す(テスト用にも使う)。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/watchers", s.handleList)
	mux.HandleFunc("POST /api/watchers/{name}/start", s.handleStart)
	mux.HandleFunc("POST /api/watchers/{name}/stop", s.handleStop)
	mux.HandleFunc("GET /api/logs", s.handleLogs)
	mux.HandleFunc("GET /api/alerts", s.handleAlerts)
	mux.HandleFunc("GET /ws/logs", s.handleWS)

	// 静的ファイル(embed.FS)。無ければ index フォールバックは不要(embed 前提)。
	if s.static != nil {
		mux.Handle("/", http.FileServer(http.FS(s.static)))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"name":    "var-watcher",
				"message": "frontend not embedded; run `make frontend` and rebuild",
			})
		})
	}
	return mux
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.List())
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.engine.Start(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.engine.Stop(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	q := store.Query{}
	if v := r.URL.Query().Get("source"); v != "" {
		q.Source = v
	}
	if v := r.URL.Query().Get("q"); v != "" {
		q.Keyword = v
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			q.Limit = n
		}
	}
	for name, dst := range map[string]**time.Time{
		"since": &q.Since,
		"until": &q.Until,
	} {
		if v := r.URL.Query().Get(name); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid " + name + ": " + v})
				return
			}
			*dst = &t
		}
	}

	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false, "logs": []store.Line{}})
		return
	}
	lines, err := s.store.Query(q)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "logs": lines})
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if s.alerts == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false, "fired": []alert.Fired{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "fired": s.alerts.History()})
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ch, unsub := s.engine.Logs()
	defer unsub()

	for line := range ch {
		data, err := json.Marshal(line)
		if err != nil {
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return
		}
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
