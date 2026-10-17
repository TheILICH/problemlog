// chi router and http handlers
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"problemlog/internal/store"
)

type Server struct {
	store *store.Store
}

func New(s *store.Store) http.Handler {
	srv := &Server{store: s}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)    // one log line per request
	r.Use(middleware.Recoverer) // panic becomes 500, server stays alive
	r.Use(middleware.Timeout(15 * time.Second))

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) { writeError(w, http.StatusNotFound, "route not found") })
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	})

	r.Get("/", srv.index)
	r.Get("/health", srv.health)
	r.Get("/stats", srv.stats)
	r.Get("/review", srv.review)

	r.Get("/problems", srv.listProblems)
	r.Post("/problems", srv.createProblem)
	r.Get("/problems/{id}", srv.getProblem)
	r.Patch("/problems/{id}", srv.patchProblem)
	r.Delete("/problems/{id}", srv.deleteProblem)
	r.Get("/problems/{id}/attempts", srv.listAttempts)
	r.Post("/problems/{id}/attempts", srv.addAttempt)

	return r
}

func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service": "problem-log",
		"routes": []string{
			"GET    /health",
			"GET    /problems?tag=&status=&min_rating=&max_rating=",
			"POST   /problems",
			"GET    /problems/{id}",
			"PATCH  /problems/{id}",
			"DELETE /problems/{id}",
			"GET    /problems/{id}/attempts",
			"POST   /problems/{id}/attempts",
			"GET    /stats",
			"GET    /review",
		},
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "db": "down"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "db": "up"})
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.Stats(r.Context())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) review(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.Review(r.Context())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) listProblems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.ProblemFilter{
		Tag:    strings.ToLower(strings.TrimSpace(q.Get("tag"))),
		Status: strings.ToLower(strings.TrimSpace(q.Get("status"))),
	}
	if f.Status != "" && !validStatus(f.Status) {
		writeError(w, http.StatusBadRequest, "status must be one of: todo, attempted, solved")
		return
	}
	var ok bool
	if f.MinRating, ok = intParam(w, q.Get("min_rating"), "min_rating"); !ok {
		return
	}
	if f.MaxRating, ok = intParam(w, q.Get("max_rating"), "max_rating"); !ok {
		return
	}

	problems, err := s.store.ListProblems(r.Context(), f)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, problems)
}

type createProblemRequest struct {
	Platform   string   `json:"platform"`
	ExternalID string   `json:"external_id"`
	Title      string   `json:"title"`
	Rating     *int     `json:"rating"`
	Tags       []string `json:"tags"`
	Notes      string   `json:"notes"`
}

func (s *Server) createProblem(w http.ResponseWriter, r *http.Request) {
	var req createProblemRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	in := store.NewProblem{
		Platform:   strings.ToLower(strings.TrimSpace(req.Platform)),
		ExternalID: strings.TrimSpace(req.ExternalID),
		Title:      strings.TrimSpace(req.Title),
		Rating:     req.Rating,
		Notes:      strings.TrimSpace(req.Notes),
	}
	if in.Platform == "" {
		in.Platform = "codeforces"
	}

	switch {
	case in.ExternalID == "":
		writeError(w, http.StatusBadRequest, "external_id is required (for example \"4A\")")
		return
	case len(in.ExternalID) > 50:
		writeError(w, http.StatusBadRequest, "external_id must be at most 50 characters")
		return
	case in.Title == "":
		writeError(w, http.StatusBadRequest, "title is required")
		return
	case len(in.Title) > 200:
		writeError(w, http.StatusBadRequest, "title must be at most 200 characters")
		return
	case len(in.Platform) > 30:
		writeError(w, http.StatusBadRequest, "platform must be at most 30 characters")
		return
	case len(in.Notes) > 2000:
		writeError(w, http.StatusBadRequest, "notes must be at most 2000 characters")
		return
	}
	if msg := validateRating(in.Rating); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	tags, msg := normalizeTags(req.Tags)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	in.Tags = tags

	p, err := s.store.CreateProblem(r.Context(), in)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) getProblem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	p, err := s.store.GetProblem(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// pointers so we can tell "not sent" from "sent empty"
type patchProblemRequest struct {
	Title  *string   `json:"title"`
	Rating *int      `json:"rating"`
	Tags   *[]string `json:"tags"`
	Status *string   `json:"status"`
	Notes  *string   `json:"notes"`
}

func (s *Server) patchProblem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req patchProblemRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	patch := store.ProblemPatch{Rating: req.Rating}
	if req.Title != nil {
		t := strings.TrimSpace(*req.Title)
		if t == "" || len(t) > 200 {
			writeError(w, http.StatusBadRequest, "title must be 1-200 characters")
			return
		}
		patch.Title = &t
	}
	if req.Notes != nil {
		n := strings.TrimSpace(*req.Notes)
		if len(n) > 2000 {
			writeError(w, http.StatusBadRequest, "notes must be at most 2000 characters")
			return
		}
		patch.Notes = &n
	}
	if req.Status != nil {
		st := strings.ToLower(strings.TrimSpace(*req.Status))
		if !validStatus(st) {
			writeError(w, http.StatusBadRequest, "status must be one of: todo, attempted, solved")
			return
		}
		patch.Status = &st
	}
	if msg := validateRating(req.Rating); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if req.Tags != nil {
		tags, msg := normalizeTags(*req.Tags)
		if msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		patch.Tags = &tags
	}

	p, err := s.store.UpdateProblem(r.Context(), id, patch)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deleteProblem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteProblem(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var validVerdicts = map[string]bool{"AC": true, "WA": true, "TLE": true, "MLE": true, "RE": true, "CE": true}

type attemptRequest struct {
	Verdict     string     `json:"verdict"`
	Minutes     int        `json:"minutes"`
	AttemptedAt *time.Time `json:"attempted_at"` // optional, for logging old attempts
}

func (s *Server) addAttempt(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req attemptRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	verdict := strings.ToUpper(strings.TrimSpace(req.Verdict))
	if !validVerdicts[verdict] {
		writeError(w, http.StatusBadRequest, "verdict must be one of: AC, WA, TLE, MLE, RE, CE")
		return
	}
	if req.Minutes < 0 || req.Minutes > 1440 {
		writeError(w, http.StatusBadRequest, "minutes must be between 0 and 1440")
		return
	}

	res, err := s.store.AddAttempt(r.Context(), id, store.NewAttempt{
		Verdict:     verdict,
		Minutes:     req.Minutes,
		AttemptedAt: req.AttemptedAt,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) listAttempts(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	attempts, err := s.store.ListAttempts(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, attempts)
}

func validStatus(s string) bool { return s == "todo" || s == "attempted" || s == "solved" }

func validateRating(r *int) string {
	if r != nil && (*r < 800 || *r > 3500) {
		return "rating must be between 800 and 3500"
	}
	return ""
}

// lowercase, trim, remove duplicates
func normalizeTags(in []string) ([]string, string) {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || seen[t] {
			continue
		}
		if len(t) > 40 {
			return nil, "each tag must be at most 40 characters"
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) > 10 {
		return nil, "at most 10 tags are allowed"
	}
	return out, ""
}

// bad id -> writes 400 and returns false
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id must be a positive integer")
		return 0, false
	}
	return id, true
}

func intParam(w http.ResponseWriter, raw, name string) (*int, bool) {
	if raw == "" {
		return nil, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, name+" must be an integer")
		return nil, false
	}
	return &n, true
}

// unknown fields are rejected so a typo like "ratting" gives 400, not silence
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "request body is empty")
		} else {
			writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		}
		return false
	}
	if dec.More() {
		writeError(w, http.StatusBadRequest, "invalid JSON body: unexpected data after the JSON object")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// store errors -> http status codes
func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "problem not found")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "a problem with this platform and external_id already exists")
	default:
		log.Printf("internal error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
