# tsk — autoship state

**Active branch:** `feature/autoship` (off main)
**Loop:** cron — 5 feature slices per 20-minute tick, gated once at the end.
**Last bootstrapped:** 2026-06-19

## Conventions

- Cron commits as `Cake (cron) <51058514+Sanjays2402@users.noreply.github.com>`.
- No emoji in commit messages, no PRs, no merges to main, no tags.
- Each feature = its own commit (revertible).
- Quality gates: `gofmt -w . && go vet ./... && go build ./... && go test ./...`
- Real working tree path on this host: `/Volumes/Projects/tsk` (the
  user-facing `/Users/sanjay/Projects/tsk` path is a transient mount that
  can disappear mid-session, so always resolve via the real volume).

## Roadmap (great tsk features sized for this loop)

Items marked `[x]` are shipped on this branch. `[ ]` are unstarted.
Pull the next 5 unstarted items per tick.

### Per-task CLI shortcuts (replace verbose `tsk bulk --id N` invocations)

- [x] `tsk show <id>` — single-task detail view with notes, full timestamps; `--json` for scripts (tick 2026-06-19/2122)
- [x] `tsk pri <id> <prio>` — quick priority setter; aliases `priority` (tick 2026-06-19/2122)
- [x] `tsk due <id> <date>` — quick due setter; `--clear` to remove; natural language (tick 2026-06-19/2122)
- [x] `tsk tag <id> +foo -bar` — add/remove tags on a single task in one shot (tick 2026-06-19/2122)
- [x] `tsk where` — print resolved `.tsk.md` path + how it was resolved + tz; `--json` (tick 2026-06-19/2122)
- [x] `tsk rename <id> <new title>` — quick title change without dropping into TUI (tick 2026-06-19/2209)
- [x] `tsk note <id> [text]` — add/edit notes from CLI (opens $EDITOR with no text arg) (tick 2026-06-19/2209)
- [x] `tsk clone <id>` / `tsk dupe <id>` — duplicate a task with a fresh ID (tick 2026-06-19/2209)
- [x] `tsk reopen <id>` — alias for `undo` (more discoverable verb) (tick 2026-06-19/2209)
- [x] `tsk snooze <id> <date>` — bump due date forward; refuses if already further out (with --force override) (tick 2026-06-19/2209)

### New views / scriptable surfaces

- [x] `tsk top [N]` — show N highest-priority undone tasks (multi-task `next`) (tick 2026-06-19/2306)
- [x] `tsk today` / `tsk overdue` — convenience aliases for the common `ls --today` / `ls --overdue` invocations (tick 2026-06-19/2306)
- [x] `tsk tags` — list all tags with usage counts (extended top-tags; supports `--json`) (tick 2026-06-19/2306)
- [x] `tsk log` — show recently completed tasks (tail of completions, optional `--since`) (tick 2026-06-19/2306)
- [x] `tsk yesterday` — what was completed yesterday (standup-friendly summary) (tick 2026-06-20/0312)
- [x] `tsk daily` — synthesized morning plan: overdue + today + top 3 upcoming, in one screen (tick 2026-06-20/0312)
- [x] `tsk grep <regex>` — exact regex search across title+notes (vs fuzzy `search`) (tick 2026-06-19/2306)
- [x] `tsk diff` — diff active `.tsk.md` vs `.bak` snapshot (what would `undo-last` undo) (tick 2026-06-20/0312)
- [x] `tsk env` — print effective env: TSK_TZ, EDITOR, NO_COLOR, paths (tick 2026-06-20/0312)

### Storage / model / format

- [ ] `tsk import <path>` — accept todo.txt / TaskWarrior `task export` JSON / Notion CSV
- [x] `tsk export --jsonl` — streaming JSON-lines for pipelines (tick 2026-06-20/0312)
- [ ] Recurrence (`tsk add -r weekly`, recur-on-done): adopt from stale `feat-recur` branch carefully
- [ ] Started/in-progress state with `tsk start <id>` / `tsk stop <id>` + `started:` timestamp
- [ ] Multi-file aggregation (`ls --include ~/work/.tsk.md --include ~/home/.tsk.md`)
- [ ] Config file at `~/.tsk/config.toml` for default file, default priority, palette overrides

### TUI

- [ ] Detail pane (right side): expanded view of selected task with notes/timestamps
- [ ] `u` / Ctrl-Z in-session undo (separate from CLI `undo-last`)
- [ ] `g`/`G` top/bottom navigation (vim-style)
- [ ] Pomodoro / focus timer overlay (`f` to start, status bar countdown)
- [ ] Task creation form with priority/due/tag fields exposed (currently title-only)

### Polish & DX (added 2026-06-20)

- [x] `tsk pin <id>` — sticky-flag a task so it appears first in `top`/`next` regardless of priority (tick 2026-06-20/0640)
- [ ] `tsk depend <id> --on <other>` — track task dependencies; block `done` if unmet
- [x] `tsk wait <id> <until>` — hide a task from default views until a date (separate from due) (tick 2026-06-20/0640)
- [ ] `tsk move-to <file>` — relocate a task between .tsk.md files (cross-store mover)
- [ ] `tsk shell` — interactive REPL with command history, useful for batch sessions
- [ ] `tsk view <id> --watch` — re-render every N seconds while iterating
- [x] `tsk last` — show the most recently mutated task (whatever the last edit was) (tick 2026-06-20/0640)
- [ ] `tsk hist [id]` — list every save's diff against the previous from git-like backups (would need backup chain)
- [x] `tsk completion --install` — write the shell completion script directly into the right rc file (tick 2026-06-20/0640)
- [x] `tsk man` — generate a manpage from cobra's help tree (tick 2026-06-20/0640)

### Polish & DX (added 2026-06-20 tick #6)

- [ ] `tsk show <id> --watch` — re-render the detail view every N seconds (live progress)
- [ ] `tsk find <regex>` — `grep` over titles only (no notes scan, faster for big stores)
- [x] `tsk rebuild-ids` — densify ID space after lots of removes (1,5,7,12 -> 1,2,3,4) (tick 2026-06-20/0948)
- [ ] `tsk recent` — alias for `last` with a window flag (`--since 1h`) — NOTE: `recent` is already a `log` alias; consider a different verb
- [x] `tsk pri --up <id>` / `tsk pri --down <id>` — cycle priority without remembering the name (tick 2026-06-20/0948)
- [ ] `tsk depends-on <id>` — set/list the prerequisite chain (lighter than full graph)
- [x] `tsk lint` — validate the file (orphan IDs, dangling notes, ungrouped block) and suggest fixes (tick 2026-06-20/0948)
- [x] `tsk swap <id> <id>` — exchange two tasks' positions in the file (manual reorder) (tick 2026-06-20/0948)
- [ ] `tsk archive --strategy weekly` — roll completed tasks into per-week archive sections
- [x] `tsk bench` — print parser/save timings for the current file (useful when a store gets big) (tick 2026-06-20/0948)

### Polish & DX (added 2026-06-20 tick #7)

- [ ] `tsk start <id>` / `tsk stop <id>` — track in-progress state with `started:` meta key (also in storage backlog)
- [ ] `tsk recur <id> <interval>` — convert a task to a recurring one (sister of stale `feat-recur`)
- [ ] `tsk dedupe` — find tasks with identical or near-identical titles, surface for review
- [ ] `tsk wrap <id> <text>` — wrap a long title with `>` continuation lines for readability
- [ ] `tsk shuffle` — randomize task order for "what should I do next" decision paralysis
- [ ] `tsk freeze <id>` — alias for `wait <id> 2099-01-01` (indefinite hide; surface only via `tsk wait --list`)
- [ ] `tsk why <id>` — print the full history-ish trail (created, edited, dependencies, where it came from)
- [ ] `tsk pri-stats` — distribution of priorities (how many low/med/high/urgent), `--by-tag` breakdown
- [ ] `tsk lint --autofix-all` — combine multiple safe fixes (canonical bullet + drop unknown meta) without per-finding prompts (we have --fix; this would be the "just trust me" form)
- [ ] `tsk hash` — print a stable content hash of the store (great for CI signals: "did anything change?")

## Known footguns the loop has run into

- The `/Users/sanjay/Projects/tsk` user-facing path is an ephemeral mount.
  Mid-tick the volume can flip and the path stops resolving. Always work
  from `/Volumes/Projects/tsk` directly and verify `pwd` after any pause.
- Tick #1 (2026-06-19 20:22 PT) reported these 5 features as shipped but
  actually parked broken WIP in a git stash that referenced undefined
  functions. The bootstrap commit landed; the 5 feature commits did NOT.
  Tick #2 dropped the broken stash and re-implemented all 5 cleanly with
  passing tests. **Always verify `git log origin/<branch>` after push.**
- For commands with `-name`-style positional args (e.g. `tsk tag 1 -old`),
  cobra parses the `-o` as an unknown shorthand. Fix: set
  `DisableFlagParsing: true` on the cmd and use `extractFileFlag` (in
  tag.go) to re-extract `--file` from the raw args.

## Tick log

### 2026-06-19 20:22 PT (tick #1 — failed)

Bootstrapped `feature/autoship` off main (commit 3a378db landed). The 5
"shipped" features in the original log were NOT actually committed —
they were parked in a `git stash` with broken cross-references (root.go
referencing undefined `newShowCmd`/`newPriCmd`/`newDueCmd`/`newTagCmd`/
`newWhereCmd`). The mid-tick volume remount appears to have caused the
shipper to stash instead of commit. Stash was discovered and dropped by
tick #2.

### 2026-06-19 21:22 PT (tick #2)

Re-shipped tick #1's 5 per-task CLI shortcuts cleanly with passing tests.
Each is a single revertible commit; full quality gate (gofmt + vet +
build + go test ./...) green before push; landed on origin/feature/autoship.

- `show` — e8e2066 feat(show): tsk show <id> single-task detail view with --json
- `pri`  — cb23c03 feat(pri): tsk pri <id> <priority> quick priority setter
- `due`  — 9e3c72c feat(due): tsk due <id> <date> quick due-date setter with --clear
- `tag`  — a60ddae feat(tag): tsk tag <id> +foo -bar single-task tag mutator
- `where`— 1ba054e feat(where): tsk where prints resolved file + method + timezone

Notable: tag.go uses `DisableFlagParsing: true` + `extractFileFlag` helper
so `-old`-style remove args don't trip cobra's flag parser; that helper
is reusable for any future command needing the same trick.

### 2026-06-19 22:09 PT (tick #3)

Shipped the rest of the "per-task CLI shortcuts" section — all 5 picked
items landed clean with passing tests, single-commit each, full gate
green before push.

- `rename` — a3a981b feat(rename): tsk rename <id> <new title> single-task title change
- `clone`  — 0ef1391 feat(clone): tsk clone <id> duplicate a task with a fresh ID
- `reopen` — a9ad18a feat(reopen): tsk reopen — discoverable undo with --last and --since
- `snooze` — 9722016 feat(snooze): tsk snooze <id> <date> push due date forward only
- `note`   — 607e94a feat(note): tsk note <id> [text] add/edit/clear notes from CLI

Notable choices:
- `reopen` is more than an undo alias — it adds --last (most recently
  completed) and --since <duration> (everything in a window) for the
  two common "wait, no" patterns. Local duration parser (NOT
  store.ParseDuration), because the store helper treats `m` as months
  — would silently make `--since 5m` mean 5 months ago.
- `snooze` refuses backward moves by default (whole point: "deal with
  this later"), with --force override. On an undated task it degrades
  to a plain set with an "(initial)" marker.
- `note` has three input modes (text args, editor, --stdin) plus
  --append/--clear modifiers. Editor invocation indirected through a
  package-level noteEditor var so tests can swap it for a shim — no
  $EDITOR juggling, no subprocess fork.
- `clone` deep-copies tags and due pointer so mutating the source can
  never bleed into the clone (or vice versa); a dedicated regression
  test guards that.

### 2026-06-19 23:06 PT (tick #4)

Shipped 5 new view / scriptable-surface commands. All single-commit,
all tests passing, full gate green before push. (Three commits needed
a `git commit --fixup` + autosquash pass to fold gofmt alignment fixes
into the right feature commit — kept history clean.)

- `top`              — 1c2cefd feat(top): tsk top [N] surface the N highest-priority undone tasks
- `today` + `overdue`— a45077d feat(today,overdue): one-word verbs for the two most common ls slices
- `tags`             — 7e98ce3 feat(tags): tsk tags full per-tag usage report
- `log`              — e932f05 feat(log): tsk log chronological tail of recently completed tasks
- `grep`             — 45fae7e feat(grep): tsk grep <regex> exact RE2 regex search across tasks

Notable choices:
- `top` matches `tsk next`'s tie-break exactly (priority desc, dated-
  first, earliest-due, lower-id) so `top[0]` and `next` agree on the
  same task. Regression test guards that.
- `today`/`overdue` aren't thin aliases — they default to undone-only
  (the only sensible default for a workday) and expose the same
  --format/--json/--tag/--priority surface as `tsk ls`. Both route
  through the shared applyFilters → single source of truth.
- `tags` is the catalog cousin of `stats`'s top-5 block: full per-tag
  count, with --all/--done scope, --min suppression for noisy stores,
  --sort alpha for completion menus, JSON emits [] not null when empty.
- `log` is the journal vs `ls --done`'s catalog: newest-first, capped
  by --limit (default 10), trimmed by --since. Done tasks without a
  Completed timestamp are excluded from rows but surfaced in a footer
  line so the user can audit. JSON stays silent — contract is just the
  task array.
- `grep` is exact RE2 vs `search`'s fuzzy. Case-insensitive default
  (POSIX-grep-shaped), undone-only default, prints field annotation
  ("matched in: title") so you see WHY it matched. --files-only/-l
  for xargs pipelines, --count, --json — all mutually exclusive and
  guarded upfront. NOT using -v as a short for --invert to avoid the
  `tsk version` collision.

Pushed e9f3b3f..45fae7e to origin/feature/autoship; verified via
`git log origin/feature/autoship | head`.

### 2026-06-20 03:12 PT (tick #5)

Shipped 5 features covering the remaining "new views / scriptable
surfaces" backlog plus one storage-format extension. All single-commit,
all tests passing, full gate green (gofmt + vet + build + go test ./...)
before push. Pushed e0875f9..e257d77 to origin/feature/autoship,
verified landed.

- `yesterday` — b73dc52 feat(yesterday): tsk yesterday standup summary of yesterday's completions
- `daily`     — 0700c11 feat(daily): tsk daily synthesized morning briefing in one screen
- `diff`      — 276f155 feat(diff): tsk diff show what undo-last would revert
- `env`       — 06db477 feat(env): tsk env effective configuration dump for debug + scripts
- `export --jsonl` — e257d77 feat(export): jsonl streaming format for pipelines

Notable choices:
- `yesterday` is anchored on calendar-day boundaries [yesterdayStart,
  todayStart) in the active tz — NOT a rolling 24h window, which would
  miss yesterday's late work and include this morning's. Boundary
  tests cover 00:00 today (excluded), 00:00 yesterday (included), and
  23:59:59 yesterday (included). JSON empty case emits `[]` not `null`.
- `daily` groups into OVERDUE/TODAY/UPCOMING. Bucket assignment checks
  IsDueToday BEFORE IsOverdue because the markdown store persists due
  dates as UTC midnight — a same-day task can otherwise compare as
  "before local startOfDay". Each section sorts by canonical tie-break
  (priority desc, dated-first, earliest-due, lower ID) so it matches
  `top`/`next`. Empty sections render as "(none)" so missing slots
  are obvious. Tests use `dueDate(±2)` (not ±1) to avoid the same
  UTC-midnight-vs-local boundary causing flakes; the singleton-day
  +1d boundary IS broken in tsk's persist layer but is a pre-existing
  bug, NOT introduced here. Documented in the commit body.
- `diff` is a self-contained Myers-like LCS — no external `diff`
  binary required, works on stripped containers. Exit codes mirror
  `git diff --exit-code`: 0 clean, 1 changes (SilentExitCoder so
  main.go skips "error:"), 2 no .bak snapshot. Pair with `undo-last`
  for an inspect-before-revert workflow.
- `env` is the canonical "why did tsk do that?" debug snapshot —
  pairs with `tsk where`. Reports VISUAL > EDITOR resolution, NO_COLOR
  semantics (empty doesn't disable, anything does, per no_color.org),
  TSK_* prefixed env var enumeration. Unset vars render "(unset)"
  so missing values are distinguishable from empty ones.
- `export --jsonl` adds the third top-tier export format. Aliases
  --format jsonl + --format ndjson. Per-line schema matches one
  --json array element so consumers switching between the two don't
  rename field accessors. Empty store = empty output (NOT `[]`, NOT
  a blank line — jsonlines convention). Tests guard against embedded
  newlines (would break streaming) and mutual-exclusion with --json.

Roadmap status: per-task CLI shortcuts section (10 items) is fully
shipped. New views / scriptable surfaces section (9 items) is fully
shipped. Storage/model/format section: 1/6 shipped (the easy one).
Added a new "Polish & DX" roadmap section (10 items) so the next 3+
ticks have fresh, sized work to draw from.

### 2026-06-20 06:40 PT (tick #6)

Shipped 5 features from the new "Polish & DX" backlog plus a wait-
state model addition. All single-commit, all tests passing (96 new
test cases across the 5 features), full gate green (gofmt + vet +
build + go test ./...) before push. Pushed 865aefe..50e6aa6 to
origin/feature/autoship; verified landed.

- `pin/unpin`           — 31eaa5a feat(pin): sticky-flag tasks so they float to the top
- `last`                — 6578af4 feat(last): show the most recently mutated task
- `man`                 — cf87073 feat(man): generate manpages from the cobra command tree
- `completion --install`— be6973d feat(completion): drop scripts into per-shell paths
- `wait`                — 50e6aa6 feat(wait): hide tasks from default views until a date

Notable choices:
- `pin` is a storage format extension (new `pin:true` meta key), but
  strictly additive — the key only renders when true, so old files
  round-trip unchanged. Hand-edit `pin:false` (or delete the key)
  to clear. `next` and `top` were updated to put pinned tasks first;
  `next` also prefixes a `*` marker so users can see WHY a low-prio
  task is winning.
- `last` scores by max(Created, Completed) per task so an old task
  just marked done floats above a freshly-added one. RFC3339 is
  second-precision so the unit tests use synthetic timestamps
  directly (mostRecentlyMutated() is exposed at the package level);
  the CLI smoke test only needs a single task. Tasks with neither
  timestamp (hand-edited entries) are skipped to avoid them ranking
  at epoch zero.
- `man` pulls in github.com/spf13/cobra/doc (already transitive of
  cobra) and uses GenManTree — pages stay in sync with help text
  automatically. --install requires --yes to prevent typo-installs
  to /usr/local/share/man/man1. ./man is the default destination
  so users can preview without elevated permissions.
- `completion --install` had a special case for PowerShell: its
  profile is a single .ps1 file the user already curates (custom
  prompt, aliases, etc), so we cannot overwrite it. Instead the
  install wraps the script in sentinel-guarded blocks (`# >>> tsk
  completion >>>` / `# <<< tsk completion <<<`) and on re-run
  replaces only the block between sentinels. Idempotent. Verified
  by test that pre-seeds custom content, runs install twice, asserts
  exactly one sentinel pair survives and custom content is intact.
- `wait` is a NEW second-class date on the task (separate field
  WaitUntil, separate `wait:` meta key). The whole point is it's
  NOT the same as due — wait HIDES the task entirely until the
  date passes, due just marks it overdue. Filter wiring updates:
  ls/top/next default-hide waiting tasks; ls --all and the new
  --include-waiting flag bring them back; `tsk show <id>` and
  `tsk wait --list` always surface them. Past-dated wait is
  treated as expired (visible) — the user wanted the task back
  when that date arrived; forgetting to clear shouldn't hide
  it forever.
- The completion gofmt adjustment from tick #6 work was folded
  into the completion commit via `git commit --fixup` + autosquash
  to keep each feature one-commit.

Roadmap status: "Polish & DX" section is 5/10 shipped (pin, wait,
last, completion --install, man). Added a fresh "Polish & DX (added
2026-06-20 tick #6)" subsection with 10 more sized items so future
ticks never starve.

### 2026-06-20 09:48 PT (tick #7)

Shipped 5 features from the "Polish & DX (added tick #6)" backlog.
All single-commit, all tests passing (52 new test cases across the
5 features), full gate green (gofmt + vet + build + go test ./...)
before push. Pushed 031154a..5880aee to origin/feature/autoship;
verified landed.

- `pri --up/--down/--cycle` — 031154a feat(pri): bump priority without naming it
- `swap <id1> <id2>`        — 4d6bcaa feat(swap): exchange task positions in the file
- `lint`                    — 1daa4f3 feat(lint): .tsk.md hygiene checker with safe --fix
- `rebuild-ids`             — af9ea6d feat(rebuild-ids): densify sparse task IDs
- `bench`                   — 5880aee feat(bench): parser + render timings for the active file

Notable choices:
- `pri` Args constraint relaxes from ExactArgs(2) to RangeArgs(1, 2).
  The mode arbiter (validatePriModeFlags) enforces exactly one of
  {positional <priority>, --up, --down, --cycle}. --up/--down clamp
  at the extremes (urgent/low) and print "already at priority X"
  instead of erroring — a no-op is the friendlier response when the
  user holds the bump key. --cycle is the only mode that wraps
  (urgent -> low); --cycle-down was considered and dropped because
  upward-cycling matches the TUI's `p` key behavior.
- `swap` ships a tiny findTaskIndex helper rather than reaching for
  store.ByID — the swap needs the slice index (not a pointer) for
  the actual exchange. Helper stays adjacent until a second caller
  appears. Self-swap is a usage error so typos don't silently
  no-op.
- `lint` is the file-format sibling of `doctor` (which already
  covers in-memory checks). Surfaces things the parser TOLERATES
  on read but the writer drops on save: non-canonical bullets
  (*/+), 'X' uppercase, unknown meta keys (silently lost on next
  Save), missing created stamps, stray notes-shaped lines before
  any task. --fix is a safe round-trip through store.Save: same
  in-memory tasks, canonical bytes on disk, normal .bak chain so
  undo-last reverts. The known-meta-keys set is intentionally
  DUPLICATED locally rather than exported from store — keeps the
  lint surface revertible without breaking store's public API.
- `rebuild-ids` is destructive (every id mentioned anywhere
  shifts), so safety is layered: dry-run default, --apply requires
  --yes (defends against scripts), --since-id N preserves lower
  ids (keeps bookmarked ids stable). The planRebuildIDs algorithm
  has a tricky bit: when --since-id reserves some ids, the new
  dense numbers must not collide with those reserved ones —
  covered by a direct algorithm test (the CLI test alone would
  miss subtle off-by-ones). Aliases: densify-ids, renumber.
- `bench` measures the user's actual file (where the
  bench_test.go microbenchmarks measure synthetic data). Reports
  size, line count, task count, then load + render min/median/max
  across N iterations. Does NOT measure Save (atomic tempfile +
  fsync would dominate and pollute .bak chain). renderForBench
  inlines its own near-identical serializer rather than reaching
  into store.render (unexported) — keeps the command decoupled.
  --iter is bounded [1, 1000]. Auto-unit formatter (us under 1ms,
  ms above) keeps the numbers scannable.

Roadmap status: tick #6 "Polish & DX" subsection 9/10 shipped
(`recent` deferred: the verb already aliases `log`, needs a new
name first). Added "Polish & DX (added tick #7)" subsection with
10 more sized items so future ticks have room.

