-- +goose Up
-- Schema: one table for problems, one table for the attempts made on them.

CREATE TABLE problems (
    id          BIGSERIAL PRIMARY KEY,
    platform    TEXT        NOT NULL DEFAULT 'codeforces',
    external_id TEXT        NOT NULL,                  -- e.g. '4A' on Codeforces
    title       TEXT        NOT NULL,
    rating      INT         CHECK (rating BETWEEN 800 AND 3500),
    tags        TEXT[]      NOT NULL DEFAULT '{}',
    status      TEXT        NOT NULL DEFAULT 'todo'
                            CHECK (status IN ('todo', 'attempted', 'solved')),
    notes       TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (platform, external_id)
);

CREATE TABLE attempts (
    id           BIGSERIAL PRIMARY KEY,
    problem_id   BIGINT      NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    verdict      TEXT        NOT NULL CHECK (verdict IN ('AC', 'WA', 'TLE', 'MLE', 'RE', 'CE')),
    minutes      INT         NOT NULL DEFAULT 0 CHECK (minutes >= 0),
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX attempts_problem_idx ON attempts (problem_id, attempted_at DESC);
CREATE INDEX problems_tags_idx    ON problems USING GIN (tags);

-- +goose Down
DROP TABLE attempts;
DROP TABLE problems;
