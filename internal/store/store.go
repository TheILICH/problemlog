// all sql queries live here, plain pgx and no orm
package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// used by /health
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

type Problem struct {
	ID         int64     `db:"id"          json:"id"`
	Platform   string    `db:"platform"    json:"platform"`
	ExternalID string    `db:"external_id" json:"external_id"`
	Title      string    `db:"title"       json:"title"`
	Rating     *int      `db:"rating"      json:"rating"` // pointer so null works
	Tags       []string  `db:"tags"        json:"tags"`
	Status     string    `db:"status"      json:"status"`
	Notes      string    `db:"notes"       json:"notes"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
}

// empty tags -> [] not null in json
func (p *Problem) normalize() {
	if p.Tags == nil {
		p.Tags = []string{}
	}
}

type NewProblem struct {
	Platform   string
	ExternalID string
	Title      string
	Rating     *int
	Tags       []string
	Notes      string
}

// nil field = keep old value
type ProblemPatch struct {
	Title  *string
	Rating *int
	Tags   *[]string
	Status *string
	Notes  *string
}

type ProblemFilter struct {
	Tag       string
	Status    string
	MinRating *int
	MaxRating *int
}

type Attempt struct {
	ID          int64     `db:"id"           json:"id"`
	ProblemID   int64     `db:"problem_id"   json:"problem_id"`
	Verdict     string    `db:"verdict"      json:"verdict"`
	Minutes     int       `db:"minutes"      json:"minutes"`
	AttemptedAt time.Time `db:"attempted_at" json:"attempted_at"`
}

type NewAttempt struct {
	Verdict     string
	Minutes     int
	AttemptedAt *time.Time // nil -> now()
}

type AttemptResult struct {
	Attempt
	ProblemStatus string `json:"problem_status"`
}

type TagStat struct {
	Tag    string `db:"tag"    json:"tag"`
	Solved int    `db:"solved" json:"solved"`
	Total  int    `db:"total"  json:"total"`
}

type RatingStat struct {
	Bucket string `json:"bucket"`
	Solved int    `json:"solved"`
	Total  int    `json:"total"`
}

type Stats struct {
	Total    int          `json:"total"`
	Solved   int          `json:"solved"`
	ByTag    []TagStat    `json:"by_tag"`
	ByRating []RatingStat `json:"by_rating"`
}

type ReviewItem struct {
	ID         int64  `db:"id"          json:"id"`
	Platform   string `db:"platform"    json:"platform"`
	ExternalID string `db:"external_id" json:"external_id"`
	Title      string `db:"title"       json:"title"`
	Rating     *int   `db:"rating"      json:"rating"`
	Reason     string `db:"reason"      json:"reason"`
}

const problemCols = `id, platform, external_id, title, rating, tags, status, notes, created_at`

func (s *Store) ListProblems(ctx context.Context, f ProblemFilter) ([]Problem, error) {
	query := "SELECT " + problemCols + " FROM problems"

	// user values only go through $n, never into the string (no sql injection)
	var conds []string
	var args []any
	add := func(format string, value any) {
		args = append(args, value)
		conds = append(conds, fmt.Sprintf(format, len(args)))
	}
	if f.Tag != "" {
		add("$%d = ANY(tags)", f.Tag)
	}
	if f.Status != "" {
		add("status = $%d", f.Status)
	}
	if f.MinRating != nil {
		add("rating >= $%d", *f.MinRating)
	}
	if f.MaxRating != nil {
		add("rating <= $%d", *f.MaxRating)
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY id"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	problems, err := pgx.CollectRows(rows, pgx.RowToStructByName[Problem])
	if err != nil {
		return nil, err
	}
	if problems == nil {
		problems = []Problem{}
	}
	for i := range problems {
		problems[i].normalize()
	}
	return problems, nil
}

func (s *Store) GetProblem(ctx context.Context, id int64) (Problem, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+problemCols+" FROM problems WHERE id = $1", id)
	if err != nil {
		return Problem{}, err
	}
	p, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Problem])
	if errors.Is(err, pgx.ErrNoRows) {
		return Problem{}, ErrNotFound
	}
	p.normalize()
	return p, err
}

func (s *Store) CreateProblem(ctx context.Context, in NewProblem) (Problem, error) {
	if in.Tags == nil {
		in.Tags = []string{}
	}
	rows, err := s.pool.Query(ctx, `
		INSERT INTO problems (platform, external_id, title, rating, tags, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+problemCols,
		in.Platform, in.ExternalID, in.Title, in.Rating, in.Tags, in.Notes)
	if err != nil {
		return Problem{}, err
	}
	p, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Problem])
	p.normalize()
	return p, mapError(err)
}

func (s *Store) UpdateProblem(ctx context.Context, id int64, patch ProblemPatch) (Problem, error) {
	// coalesce keeps the old value when the patch field is null
	rows, err := s.pool.Query(ctx, `
		UPDATE problems SET
			title  = COALESCE($2, title),
			rating = COALESCE($3, rating),
			tags   = COALESCE($4, tags),
			status = COALESCE($5, status),
			notes  = COALESCE($6, notes)
		WHERE id = $1
		RETURNING `+problemCols,
		id, patch.Title, patch.Rating, patch.Tags, patch.Status, patch.Notes)
	if err != nil {
		return Problem{}, err
	}
	p, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Problem])
	if errors.Is(err, pgx.ErrNoRows) {
		return Problem{}, ErrNotFound
	}
	p.normalize()
	return p, err
}

func (s *Store) DeleteProblem(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, "DELETE FROM problems WHERE id = $1", id) // attempts deleted too (on delete cascade)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListAttempts(ctx context.Context, problemID int64) ([]Attempt, error) {
	if _, err := s.GetProblem(ctx, problemID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, problem_id, verdict, minutes, attempted_at
		FROM attempts WHERE problem_id = $1
		ORDER BY attempted_at DESC, id DESC`, problemID)
	if err != nil {
		return nil, err
	}
	attempts, err := pgx.CollectRows(rows, pgx.RowToStructByName[Attempt])
	if err != nil {
		return nil, err
	}
	if attempts == nil {
		attempts = []Attempt{}
	}
	return attempts, nil
}

// one transaction: insert attempt + update status, both happen or none
// ac -> solved, other verdict -> todo becomes attempted, solved stays solved
func (s *Store) AddAttempt(ctx context.Context, problemID int64, in NewAttempt) (AttemptResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AttemptResult{}, err
	}
	// does nothing after a successful commit
	defer tx.Rollback(ctx) //nolint:errcheck

	var status string
	err = tx.QueryRow(ctx, `
		UPDATE problems
		SET status = CASE WHEN $2::text = 'AC' THEN 'solved'
		                  WHEN status = 'todo'  THEN 'attempted'
		                  ELSE status END
		WHERE id = $1
		RETURNING status`, problemID, in.Verdict).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return AttemptResult{}, ErrNotFound
	}
	if err != nil {
		return AttemptResult{}, err
	}

	rows, err := tx.Query(ctx, `
		INSERT INTO attempts (problem_id, verdict, minutes, attempted_at)
		VALUES ($1, $2, $3, COALESCE($4, now()))
		RETURNING id, problem_id, verdict, minutes, attempted_at`,
		problemID, in.Verdict, in.Minutes, in.AttemptedAt)
	if err != nil {
		return AttemptResult{}, err
	}
	a, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Attempt])
	if err != nil {
		return AttemptResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AttemptResult{}, err
	}
	return AttemptResult{Attempt: a, ProblemStatus: status}, nil
}

// counting is done in sql (group by / filter), not in go loops
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var st Stats

	err := s.pool.QueryRow(ctx, `
		SELECT count(*)::int, (count(*) FILTER (WHERE status = 'solved'))::int
		FROM problems`).Scan(&st.Total, &st.Solved)
	if err != nil {
		return Stats{}, err
	}

	// unnest = one row per tag
	rows, err := s.pool.Query(ctx, `
		SELECT tag,
		       (count(*) FILTER (WHERE status = 'solved'))::int AS solved,
		       count(*)::int AS total
		FROM problems, unnest(tags) AS tag
		GROUP BY tag
		ORDER BY total DESC, tag`)
	if err != nil {
		return Stats{}, err
	}
	if st.ByTag, err = pgx.CollectRows(rows, pgx.RowToStructByName[TagStat]); err != nil {
		return Stats{}, err
	}
	if st.ByTag == nil {
		st.ByTag = []TagStat{}
	}

	// integer division makes buckets of 200 (1500 -> 1400)
	type bucketRow struct {
		Lo     int `db:"lo"`
		Solved int `db:"solved"`
		Total  int `db:"total"`
	}
	rows, err = s.pool.Query(ctx, `
		SELECT (rating / 200) * 200 AS lo,
		       (count(*) FILTER (WHERE status = 'solved'))::int AS solved,
		       count(*)::int AS total
		FROM problems
		WHERE rating IS NOT NULL
		GROUP BY lo
		ORDER BY lo`)
	if err != nil {
		return Stats{}, err
	}
	buckets, err := pgx.CollectRows(rows, pgx.RowToStructByName[bucketRow])
	if err != nil {
		return Stats{}, err
	}
	st.ByRating = make([]RatingStat, 0, len(buckets))
	for _, b := range buckets {
		st.ByRating = append(st.ByRating, RatingStat{
			Bucket: fmt.Sprintf("%d-%d", b.Lo, b.Lo+199),
			Solved: b.Solved,
			Total:  b.Total,
		})
	}
	return st, nil
}

// last attempt failed, or last attempt is an ac older than 14 days
func (s *Store) Review(ctx context.Context) ([]ReviewItem, error) {
	rows, err := s.pool.Query(ctx, `
		WITH last_attempt AS (
			SELECT DISTINCT ON (problem_id) problem_id, verdict, attempted_at
			FROM attempts
			ORDER BY problem_id, attempted_at DESC, id DESC
		)
		SELECT p.id, p.platform, p.external_id, p.title, p.rating,
		       CASE WHEN l.verdict <> 'AC'
		            THEN 'last attempt failed (' || l.verdict || ')'
		            ELSE 'solved ' || extract(day FROM now() - l.attempted_at)::int || ' days ago'
		       END AS reason
		FROM problems p
		JOIN last_attempt l ON l.problem_id = p.id
		WHERE l.verdict <> 'AC'
		   OR l.attempted_at < now() - interval '14 days'
		ORDER BY (l.verdict = 'AC'), l.attempted_at`) // failed first, then oldest solves
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[ReviewItem])
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []ReviewItem{}
	}
	return items, nil
}

// 23505 = unique violation
func mapError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}
