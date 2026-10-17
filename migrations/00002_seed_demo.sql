-- +goose Up
-- Demo data, so the API is not empty the first time it starts.
-- goose runs this file only once per database, so the teacher's own changes
-- are never overwritten when the containers are restarted or recreated.
-- Timestamps are relative to "now" so /review has something to show on day one.

INSERT INTO problems (platform, external_id, title, rating, tags, status, notes) VALUES
    ('codeforces', '4A',   'Watermelon',            800,  '{math,"brute force"}',                          'solved',    'Even and > 2 is the whole trick.'),
    ('codeforces', '71A',  'Way Too Long Words',    800,  '{strings}',                                     'solved',    ''),
    ('codeforces', '231A', 'Team',                  800,  '{"brute force",greedy}',                        'solved',    ''),
    ('codeforces', '50A',  'Domino piling',         800,  '{greedy,math}',                                 'solved',    'Area / 2, no need to place anything.'),
    ('codeforces', '580A', 'Kefa and First Steps',  900,  '{dp,implementation}',                           'solved',    'Longest non-decreasing run.'),
    ('codeforces', '455A', 'Boredom',               1500, '{dp}',                                          'attempted', 'Count values, then dp over the value axis.'),
    ('codeforces', '160A', 'Twins',                 900,  '{greedy,sortings}',                             'todo',      ''),
    ('codeforces', '466C', 'Number of Ways',        1700, '{"binary search","data structures",dp,"two pointers"}', 'todo', '');

INSERT INTO attempts (problem_id, verdict, minutes, attempted_at)
SELECT p.id, v.verdict, v.minutes, v.attempted_at
FROM problems p
JOIN (VALUES
    ('4A',   'AC',  12, now() - interval '40 days'),
    ('71A',  'AC',   5, now() - interval '21 days'),
    ('231A', 'WA',  10, now() - interval '6 days'),
    ('231A', 'AC',   7, now() - interval '6 days' + interval '20 minutes'),
    ('50A',  'AC',   9, now() - interval '2 days'),
    ('580A', 'WA',  25, now() - interval '1 day'),
    ('580A', 'AC',  18, now() - interval '1 day' + interval '40 minutes'),
    ('455A', 'WA',  45, now() - interval '3 days'),
    ('455A', 'TLE', 60, now() - interval '1 day')
) AS v (external_id, verdict, minutes, attempted_at)
  ON p.platform = 'codeforces' AND p.external_id = v.external_id;

-- +goose Down
DELETE FROM problems
WHERE platform = 'codeforces'
  AND external_id IN ('4A', '71A', '231A', '50A', '580A', '455A', '160A', '466C');
