// Package httpapi exposes a small read-only HTTP surface on the in-cluster
// recorder so operators can list segments and pull a time-range extract as a
// portable .loog file without shelling into the pod.
//
// NOTE: it is meant to be used using a port-forward. DO NOT EXPOSE THIS API!
package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/expr-lang/expr"
	"github.com/rs/zerolog/log"

	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"github.com/loog-project/loog/internal/store/segment"
	"github.com/loog-project/loog/internal/util"
)

type Server struct {
	dir           string
	activePathFn  func() string
	defaultFilter string
	compress      bool
}

func NewServer(dir string, activePathFn func() string, defaultFilter string, compress bool) *Server {
	if defaultFilter == "" {
		defaultFilter = "All()"
	}
	return &Server{
		dir:           dir,
		activePathFn:  activePathFn,
		defaultFilter: defaultFilter,
		compress:      compress,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/segments", s.handleSegments)
	mux.HandleFunc("/extract", s.handleExtract)
	return mux
}

func (s *Server) activePath() string {
	if s.activePathFn == nil {
		return ""
	}
	return s.activePathFn()
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok\n")
}

type segmentsResponse struct {
	Segments []segment.Info `json:"segments"`
}

func (s *Server) handleSegments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	segs, err := segment.ListSegments(s.dir, s.activePath())
	if err != nil {
		http.Error(w, "list segments: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(segmentsResponse{Segments: segs})
}

func (s *Server) handleExtract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	from, to, err := segment.ResolveRange(q.Get("from"), q.Get("to"), q.Get("since"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filterExpr := q.Get("filter")
	if filterExpr == "" {
		filterExpr = s.defaultFilter
	}
	program, err := expr.Compile(filterExpr, expr.Env(util.EventEntryEnv{}), expr.AsBool())
	if err != nil {
		http.Error(w, "compile filter: "+err.Error(), http.StatusBadRequest)
		return
	}

	tmp, err := os.CreateTemp("", "loog-extract-*.loog")
	if err != nil {
		http.Error(w, "temp file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	stats, err := segment.Extract(r.Context(), s.dir, tmpPath, segment.ExtractOptions{
		From:          from,
		To:            to,
		Filter:        program,
		ExcludeActive: s.activePath(),
		Bolt:          bboltStore.Options{Durable: true, Compress: s.compress},
	})
	if err != nil {
		http.Error(w, "extract: "+err.Error(), http.StatusInternalServerError)
		return
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		http.Error(w, "open extract: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = f.Close() }()

	fi, _ := f.Stat()
	filename := fmt.Sprintf("loog-%s.loog", time.Now().UTC().Format("20060102T150405Z"))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	if fi != nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fi.Size()))
	}
	w.Header().Set("X-Loog-Revisions", fmt.Sprintf("%d", stats.RevisionsWritten))
	w.Header().Set("X-Loog-Resources", fmt.Sprintf("%d", stats.Resources))

	if _, err := io.Copy(w, f); err != nil {
		log.Error().Err(err).Msg("Error streaming extract to client")
	}
}
