# BigDataCorp Tech Challenge

Pipeline that takes a JSONL input and transforms it into two CSV files.

## Stack

- Go 1.26, stdlib only, no external deps, no concurrency. Module `bigdatacorp-test`.
- "Efficient for large files" here means **streaming**: read line-by-line with
  constant memory (`bufio.Scanner`), never load the whole file. Not parallelism.
- The scanner buffer is raised to ~1 MB. A club with a large `players` array exceeds
  the 64 KB default, which would otherwise kill the run mid-file.

## File Structure

```
main.go                   # entry point: arg parsing, file open/close, exit codes. No business logic.
domain/domain.go          # types + rules: structs, mappers, predicates. No I/O, minimum imports.
domain/domain_test.go     # table-driven tests for the rules
pipeline/pipeline.go      # data flow: scan, unmarshal call, filter, serialize, write.
pipeline/pipeline_test.go # table-driven tests over *bytes.Buffer
```

Note: struct definitions with `json` tags live in `domain` (the struct *is* the
model; tags are metadata, no import needed). The `json.Unmarshal` *call* lives in
`pipeline` — it's part of reading the stream.

Fields present in the input but absent from SPEC.md (`titles`, `nationality`,
`market_value`) get no struct fields at all — `encoding/json` drops unknown keys.

## Core invariants

Two structural rules the rest of the design leans on.

- **`domain` cannot fail.** No mapper or predicate in `domain` returns an `error`.
  Normalization is total: a bad date yields `""`, never a failure. This is what makes
  "a bad date never drops a record" true by construction rather than by discipline.
  All *skipping* lives in `pipeline`; all *normalization* lives in `domain`.
- **Trimming happens in exactly one place.** `(*Club).Normalize()` runs once per line,
  immediately after unmarshal and before any guard, filter or mapper. It trims
  `club_id`, `championship`, and each `player_id` — after it runs, the trimmed value
  *is* the value. `strings.TrimSpace` appears nowhere else in the codebase. Every other
  field reaches the CSV verbatim.

## Key business rules

Rules about what the data means — these live in `domain/` (no I/O).

- **Filter (`championship`):** only clubs whose championship is exactly `SERIE A` or
  `SERIE B` are exported. **Case-sensitive** — `serie a` is excluded. Surrounding
  whitespace is not significant: `" SERIE A "` passes (already trimmed by `Normalize`),
  and the **trimmed** value is what gets written to `Campeonato`. Predicate:
  `(*Club).IsExportable()`. A club failing this produces no row in `clubs.csv`, and
  none of its players appear in `players.csv`.
- **1:N relation:** each `players.csv` row carries the `club_id` of its club. A club
  with no players produces no rows in `players.csv`, but is still written to
  `clubs.csv` if it passes the filter.
- **Cores:** join the `colors` array into one field using `|` (pipe), preserving
  order. `["preto","branco"]` -> `preto|branco`. Empty or absent array -> empty field.
- **Dates:** normalize to `yyyy-MM-dd`. **Only `yyyy-MM-dd` input is accepted** — a
  single `time.Parse` with layout `2006-01-02`. Zero-padded layout verbs require
  exactly two digits, so `2024-1-8` is rejected alongside `18/01/2024`, RFC3339
  timestamps, and impossible dates like `2024-02-30`. Absent, null, or unparseable ->
  empty field. **The row still appears** — a bad date never drops a record.
- **Empty fields:** a field that is absent, JSON `null`, or an empty string ->
  empty CSV field. (Applies to `nickname` in particular.)
- **Numbers are never fabricated.** `age`, `goals` and `shirt_number` are `*int`.
  Absent or null -> `nil` -> empty field. A literal `0` -> `0`. The two must never be
  conflated: a plain `int` would silently turn missing data into a fabricated zero.
  `0` is never treated as absent, even where it's implausible — a 0-year-old player is
  the data's problem, not ours.

## Pipeline behavior

How records flow and how output is written — these live in `pipeline/`.

- **CSV format:** UTF-8 (no BOM), header line, comma-delimited, RFC 4180 quoting and
  CRLF row terminators. Go defaults to UTF-8 and `encoding/csv` handles the quoting —
  never hand-join rows. The CRLF is *not* free: `csv.Writer` writes a bare `\n` unless
  `UseCRLF` is set, which `Run` does on both writers.

- **Order of checks per club — this order is load-bearing:**

  ```
  unmarshal              → invalid?  log(line), continue
  club.Normalize()
  !club.IsExportable()   → continue                    (silent, unconditionally)
  !club.HasID()          → log(line), continue
  write club row
  for i := range club.Players:
      !club.Players[i].IsExportable() → log(line, clubID), continue
      write player row
  ```

  The filter runs **first**, and its skip is silent under all circumstances — a
  non-SERIE club with a missing or blank `club_id` is still skipped silently, because
  we were never going to export it. Logging is reserved for data we wanted and
  couldn't use.

- **`IsExportable` is asymmetric — the name does not carry the logging obligation:**
  - `(*Club).IsExportable()` false -> skip **silently**. It covers the championship
    filter *only*; it says nothing about `club_id`. That's `(*Club).HasID()`, which is
    a separate predicate precisely because its skip *is* logged.
  - `(*Player).IsExportable()` false -> skip **and log**. It means "has a `player_id`".

- **Robustness — four distinct failure modes, do not conflate them:**
  - *Invalid JSON* (line doesn't parse): skip the whole line, continue, log to
    stderr with the line number.
  - *Incomplete record — missing an essential identifier:* absent, `null`, `""`, and
    whitespace-only all count as missing (after `Normalize`, all four are `""`). A club
    with no `club_id` is unusable (no key, no FK for its players) -> skip the whole club
    and its players. A player with no `player_id` -> skip that player only, keep the
    club and its other players.
  - *Bad/invalid date:* NOT a skip — empty field, row stays (see Dates rule).
  - *Absent/null optional field* (nickname, city, colors, age, goals, etc.): NOT a
    skip — empty field, row stays.
  - A club excluded by the championship filter is skipped **silently** — that's
    expected output, not an error, so it is not logged.

- **Blank lines** (empty or whitespace-only) are skipped silently — they are not
  reported as invalid JSON.

- **Exit code is 0 when lines were skipped.** Partial success is the specified
  behavior, not a failure. Only a missing argument, an unopenable input, or a write
  error exits non-zero.

## Testing

- Table-driven throughout, stdlib `testing` only.
- `pipeline.Run(in io.Reader, clubs, players, logw io.Writer) error` takes writers, not
  paths, so tests drive it with `*bytes.Buffer` and assert on all three outputs —
  including the diagnostic log. That sink is injectable rather than hardcoded to
  `os.Stderr` specifically so the "logged vs. silent" distinction is testable. `main`
  passes `os.Stderr`.
- The four failure modes have **no fixture file** — `sample_clubes.jsonl` is the happy
  path only. They live in test tables. EXAMPLES.md lists the expected behavior of each
  case; those rows and the test tables are meant to stay in sync.

## Usage

Input JSONL path is passed as a CLI argument:

```
go run . <input.jsonl>
```

Outputs `clubs.csv` and `players.csv` in the working directory, truncating them if
they already exist.

If the argument is missing or the file cannot be opened, print usage to stderr and
exit non-zero. (This is distinct from a malformed *line*: a missing file fails fast;
a bad line is skipped and processing continues.)
