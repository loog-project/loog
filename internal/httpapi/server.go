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
	"github.com/rs/zerolog"

	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"github.com/loog-project/loog/internal/store/segment"
	"github.com/loog-project/loog/internal/util"
)

type Server struct {
	store         *segment.Store
	defaultFilter string
	compress      bool
	log           zerolog.Logger
}

// NewServer builds a Server bound to the recorder's live segment store, so
// extraction can include the active (locked) segment and there is no blind
// window for the most recent data. logger receives request/access logs.
func NewServer(store *segment.Store, defaultFilter string, compress bool, logger zerolog.Logger) *Server {
	if defaultFilter == "" {
		defaultFilter = "All()"
	}
	return &Server{
		store:         store,
		defaultFilter: defaultFilter,
		compress:      compress,
		log:           logger,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	// /healthz is hit by the liveness/readiness probes on a tight interval, so
	// it is intentionally not access-logged.
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/segments", s.withAccessLog("segments", s.handleSegments))
	mux.HandleFunc("/extract", s.withAccessLog("extract", s.handleExtract))
	return mux
}

// statusWriter wraps http.ResponseWriter to capture the status code and the
// number of bytes written, for access logging.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if sw.status == 0 {
		sw.status = http.StatusOK
	}
	n, err := sw.ResponseWriter.Write(b)
	sw.bytes += int64(n)
	return n, err
}

// withAccessLog wraps a handler to log one line per request: who called, what
// they asked for (path + query), the response status, bytes, and duration.
func (s *Server) withAccessLog(name string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		s.log.Info().
			Str("handler", name).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("query", r.URL.RawQuery).
			Str("remote", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Msg("request received")

		h(sw, r)

		s.log.Info().
			Str("handler", name).
			Str("method", r.Method).
			Str("remote", r.RemoteAddr).
			Int("status", sw.status).
			Int64("bytes", sw.bytes).
			Dur("took", time.Since(start)).
			Msg("request completed")
	}
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
	segs, err := segment.ListSegments(s.store.Dir(), s.store.ActivePath())
	if err != nil {
		s.log.Error().Err(err).Msg("list segments failed")
		http.Error(w, "list segments: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.log.Info().Int("segments", len(segs)).Msg("listing segments")
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
		s.log.Warn().Err(err).
			Str("from", q.Get("from")).Str("to", q.Get("to")).Str("since", q.Get("since")).
			Msg("extract: invalid time range")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filterExpr := q.Get("filter")
	if filterExpr == "" {
		filterExpr = s.defaultFilter
	}
	program, err := expr.Compile(filterExpr, expr.Env(util.EventEntryEnv{}), expr.AsBool())
	if err != nil {
		s.log.Warn().Err(err).Str("filter", filterExpr).Msg("extract: invalid filter")
		http.Error(w, "compile filter: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Surface exactly what the client asked for, with resolved (absolute) bounds.
	effTo := to
	if effTo.IsZero() {
		effTo = time.Now()
	}
	s.log.Info().
		Str("remote", r.RemoteAddr).
		Str("from", timeStr(from)).
		Str("to", timeStr(effTo)).
		Str("since", q.Get("since")).
		Str("filter", filterExpr).
		Msg("extract requested")

	tmp, err := os.CreateTemp("", "loog-extract-*.loog")
	if err != nil {
		s.log.Error().Err(err).Msg("extract: temp file failed")
		http.Error(w, "temp file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	stats, err := segment.ExtractLive(r.Context(), s.store, tmpPath, segment.ExtractOptions{
		From:   from,
		To:     to,
		Filter: program,
		Bolt:   bboltStore.Options{Durable: true, Compress: s.compress},
	})
	if err != nil {
		s.log.Error().Err(err).Msg("extract failed")
		http.Error(w, "extract: "+err.Error(), http.StatusInternalServerError)
		return
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		s.log.Error().Err(err).Msg("extract: open output failed")
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

	logEvent := s.log.Info()
	if fi != nil {
		logEvent = logEvent.Int64("size_bytes", fi.Size())
	}
	logEvent.
		Int("segments_scanned", stats.SegmentsScanned).
		Int("revisions", stats.RevisionsWritten).
		Int("resources", stats.Resources).
		Msg("extract complete")

	if _, err := io.Copy(w, f); err != nil {
		s.log.Error().Err(err).Msg("error streaming extract to client")
	}
}

func timeStr(t time.Time) string {
	if t.IsZero() {
		return "beginning"
	}
	return t.UTC().Format(time.RFC3339)
}
