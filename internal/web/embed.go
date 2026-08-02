package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// UseEmbedded は Vue のビルド成果物(frontend/dist)を静的配信に使う。
func (s *Server) UseEmbedded() {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	s.static = sub
}
