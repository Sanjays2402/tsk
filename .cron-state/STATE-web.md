# tsk autoship — WEB FRONTEND loop state

This file is owned by a second cron loop dedicated to building a real WEB
UI for tsk, additively. It lives alongside `STATE.md` (which is owned by
the original CLI-feature autoship loop) so both loops can coexist on
`main` without trampling each other.

Owner: Cake (cron), 20-min web-mission tick. Branch: **main**.
Mission override (set 2026-06-23 by Sanjay): build a real web frontend for
tsk, additively. The TUI/CLI keep working; `.tsk.md` stays the source of
truth.

## Stack decision (locked this tick)

- **Server**: new `tsk serve` Cobra subcommand. Pure Go `net/http`, reuses
  the existing `internal/store` for all reads/writes. JSON API at
  `/api/...`.
- **Embedding**: web client compiles to `internal/serve/web_dist/` and is
  shipped inside the Go binary via `go:embed`. Zero external runtime deps.
- **Client**: Vite + TypeScript + **vanilla DOM** (no React/Vue). One
  small framework-free SPA — keeps the bundle tiny (~3KB gz JS) and
  matches the keyboard-first, lean spirit of the TUI. Linear/Raycast-
  quality polish via hand-crafted CSS using the amber/gold tokens
  (`web/src/tokens.css`).
- **Auth**: bound to `127.0.0.1` by default, no auth. `--addr` can
  override for trusted networks; token auth is roadmap (F20).
- **Lifecycle**: server is local-first, single-user, single-process. File
  is re-read on every request — the .tsk.md may have changed under us
  (someone edited by hand or the CLI/TUI wrote). Writes go through
  atomic `store.Save`.

## Architecture sketch

```
cmd/tsk/main.go
  └─ commands.NewRoot()
      ├─ existing CLI verbs (add/ls/done/...) — owned by the CLI loop
      ├─ tui (default no-arg)
      └─ serve  ← THIS loop adds + grows this
            └─ internal/serve.Run(addr, file)
                  ├─ JSON API: GET /api/tasks, POST /api/tasks,
                  │            PATCH /api/tasks/:id, DELETE /api/tasks/:id,
                  │            POST /api/tasks/:id/toggle, GET /api/stats
                  └─ SPA: GET /  → web/dist/index.html  (go:embed)
                          GET /assets/*  → web/dist/assets/*
```

## Roadmap — 22 features (frontend mission)

### Foundation (tick T1)
- [x] **F1** Stack pick + STATE-web.md bootstrap.
- [x] **F2** `tsk serve` Cobra command + `internal/serve` package with
      full CRUD JSON API and stats endpoint.
- [x] **F3** Vite scaffold: `web/package.json`, `web/tsconfig.json`,
      `web/vite.config.ts`, `web/index.html`, `web/src/main.ts`,
      `web/src/tokens.css` (amber/gold + NO_COLOR-spirit dark/light).
      Pre-built `web_dist/` checked in so `go build` works without npm.
      Embedded via `go:embed`.
- [x] **F4** End-to-end vertical: SPA loads `/api/tasks` on boot, renders
      a first-pass list with priority chips, due badges, tag pills, done
      state. Empty state included. Keyboard-friendly focus ring.
- [x] **F5** Toggle done via the UI: click checkbox → `POST /api/tasks/:id/toggle`
      → optimistic update + server confirm. Round-trips to .tsk.md on disk.

### Next tick (T2) — make it usable
- [x] **F6** Add-task form (header bar, `n` shortcut, Linear-ish slim
      input with priority/due/tag chips). (tick T2 2026-06-24)
- [x] **F7** Inline edit title (double-click or `e` on selected row).
      (tick T2 2026-06-24)
- [x] **F8** Delete with undo (toast + 5s timer). (tick T2 2026-06-24)
- [x] **F9** Sections: Overdue / Today / Upcoming / No Due / Done —
      matching the TUI's mental model. (tick T2 2026-06-24)
- [x] **F10** Keyboard nav: j/k to move selection, space/enter to toggle,
      `?` for help overlay (mirror the TUI hotkeys). Also e=edit, x=delete,
      u=undo, g/G=first/last. (tick T2 2026-06-24)

### T3 — search, filter, polish
- [x] **F11** Filter bar: tag chips multi-select, priority filter, search
      box with fuzzy matching. (tick T3 2026-06-24)
- [x] **F12** Due-date picker that accepts natural-language strings
      (`tomorrow`, `fri`, `in 3d`, `eow`) — server validates via the
      existing `dateparse` package; UI shows the parsed date back.
      (tick T3 2026-06-24)
- [x] **F13** Stats sidebar: total / done / overdue / completion %,
      streak, top tags. Reuse `computeStatsDTO`. (tick T3 2026-06-24)
- [x] **F14** Theme toggle (auto/light/dark) honoring system + storing
      pref in localStorage. Dark = amber-on-charcoal.
      Light = ochre-on-cream. (tick T3 2026-06-24)
- [x] **F15** Tag pages: click a tag → filtered view; URL hash `#tag/dev`.
      (tick T3 2026-06-24)

### T4 — power user
- [ ] **F16** Bulk select (shift+click range, multi-toggle/multi-delete).
- [ ] **F17** Drag-to-reorder within a section, with persistence (writes
      back to .tsk.md preserving order — `store.Tasks` slice IS the order).
- [ ] **F18** Cmd-K command palette: every action, fuzzy-find, keyboard-only.
- [ ] **F19** Export buttons (JSON / CSV / Markdown) — hit existing
      exporters via `/api/export?format=...`.
- [ ] **F20** Token auth for `--addr 0.0.0.0` use: `tsk serve --token=...`
      enforces `Authorization: Bearer` on `/api/*`.

### T5 — production
- [ ] **F21** SSE live-reload: server watches `.tsk.md` mtime; pushes
      change events to connected clients so multi-tab / external edits
      show up.
- [ ] **F22** PWA manifest + offline cache shell. Add-to-home on
      iOS/Android.

When fewer than 5 remain, append more (mobile-touch tweaks, settings
drawer, recurring tasks UI, archive view, etc.).

## Conventions

- All commits on **main**, signed as `Cake (cron)`. No emoji in git.
- Each feature = its own commit, revertible.
- Quality gates run once at end of batch:
  `gofmt -w . && go vet ./... && go build ./... && go test ./...`
  plus, whenever `web/` is touched:
  `npm --prefix web install` (first-tick / when deps change) and
  `npm --prefix web run build`.

## Coexistence with the CLI autoship loop

`STATE.md` (sibling) is owned by the other cron loop adding CLI verbs.
Both loops:
- write under different filename namespaces (`STATE.md` vs
  `STATE-web.md`, `sessions/<date>-cli` vs `sessions/<date>-web`),
- add disjoint files (CLI loop touches `internal/commands/<verb>.go`;
  web loop owns `internal/serve/`, `web/`, and registers a single
  `newServeCmd()` line in `internal/commands/root.go`),
- when they collide on `root.go`, the web loop appends its line below
  the CLI loop's edits.

If a tick produces a conflict, the web loop rebases and re-applies its
slim diff on top of HEAD before pushing.

## Tick log

(append new entries at the bottom, newest last)

### T1 — 2026-06-24 03:24 PT — frontend bootstrap (5/5)

- F1 chore(cron): bootstrap STATE-web.md + web roadmap
- F2 feat(serve): JSON API + `tsk serve` over .tsk.md
- F3 feat(web): Vite+TS SPA scaffold with amber/gold tokens
- F4 feat(web): live task list (priority chips, due badges, tags)
- F5 feat(web): click checkbox to toggle done w/ optimistic update

End-of-batch gates: gofmt clean, vet clean, go test ./... ok across all
packages, npm web build ok (2.78KB gz JS, 2.25KB gz CSS). Live
end-to-end test: server + embedded SPA serve from a single binary,
toggle round-trips through atomic `store.Save` preserving the existing
.tsk.md storage format byte-for-byte.

Roadmap status: F1-F5 done. F6-F22 unstarted, queued for T2+.

### T2 — 2026-06-24 04:44 PT — make it usable (5/5)

Workdir note: the canonical workdir `/Volumes/Projects/tsk` (an APFS
sparseimage on an external SSD) was NOT mounted this tick — the SSD was
physically absent (`diskutil list external physical` empty, mount script
hung waiting for it). Worked from the fully-synced internal worktree
`/Users/sanjay/Projects/tsk-features/main`, which shares the same origin
and was at origin/main (35c50c7) with a clean tree. Pushed from there; a
clean fast-forward, no divergence. This is the right fallback whenever the
SSD is unplugged.

- F6 feat(web): quick-add composer with inline !prio @due #tag syntax (0f9291d)
- F9 feat(web): group list into Overdue/Today/Upcoming/No Due/Done (9c4644d)
- F10 feat(web): keyboard nav + selection model + help overlay (7861731)
- F8 feat(web): delete a task with a 5s undo toast (d6ca24c)
- F7 feat(web): inline title edit on double-click or `e` (9a5ef86)

The backend CRUD API was already complete from T1, so all five slices are
frontend. Each pure module (quickadd, composer, sections, keynav, toast,
edit) is unit-tested under Node's native TS runner (`node --test`) — 48
frontend tests total, added this tick along with a test tsconfig,
@types/node (dev-only, not bundled), and npm scripts (test, typecheck:test,
check). Gates: gofmt/vet/build clean, go test ./... ok (incl. internal/serve),
web typecheck + 48 tests + build all green (5.98KB gz JS). Verified the full
add -> toggle -> edit -> delete flow end-to-end against a live `tsk serve`,
all writing .tsk.md in the existing hand-editable format with the CLI/TUI
contract intact.

Roadmap status: F1-F10 done. F11-F22 unstarted, queued for T3+ (filter bar,
due picker, stats sidebar, theme toggle, tag pages, then power-user/prod).

### T3 — 2026-06-24 07:06 PT — search, filter, polish (5/5)

Workdir note: the canonical `/Volumes/Projects/tsk` (external SSD sparseimage)
was again NOT mounted this tick (the SSD is physically absent; `/Volumes` only
has Macintosh HD). Worked from the fully-synced internal worktree
`/Users/sanjay/Projects/tsk-features/main` (same origin, was exactly at
origin/main 2689071, clean tree). Pushed as a clean fast-forward, zero
divergence. This is now the documented fallback (see T2 note).

- F11 feat(web): filter bar with fuzzy search, priority + tag facets (06610a2)
- F12 feat(web): natural-language due-date picker with live preview (20061b3)
- F13 feat(web): stats sidebar with completion donut, metrics + top tags (7cd31b0)
- F14 feat(web): theme toggle (auto / light / dark) with persistence (2c839ec)
- F15 feat(web): tag pages with hash routing (#tag/<name>) (fc26b3a)

F12 is the only slice with a backend change: a new read-only GET
/api/parse-date endpoint over internal/dateparse (soft-fails with 200+ok:false
so the live picker doesn't spam 400s). Everything else is pure frontend over
the existing JSON API. Each slice carries a pure, dependency-free logic module
unit-tested under node --test: filter.ts (16), duepicker.ts (13), stats.ts (10),
theme.ts (8), router.ts (11) = 58 new frontend tests (106 total), plus 4 new Go
tests on internal/serve for parse-date.

Gates (run once at end of batch): gofmt -l clean, go vet ./... clean, go build
./... ok, go test ./... ok across all packages (incl. internal/serve with the
embedded T3 bundle), web `npm run check` (tsc app + tsc test + node --test) 106
pass, `npm run build` ok — JS 10.3KB gz, CSS 4.8KB gz, 18 modules.

End-to-end proof: built the binary, ran tsk init + serve on a temp store with 3
tagged tasks, then exercised the live surface: /api/parse-date resolved eom ->
2026-06-30 "in 6d", "in 3d" -> 2026-06-27, garbage -> soft-fail with the helpful
"try: tomorrow, fri, ..." message; /api/stats returned the top-tags feed the F13
sidebar renders; the SPA index served the freshly-built hashed assets (200, both
JS+CSS), and the bundle contained the new slice code (parse-date, tag/ routing,
tsk.theme/tsk.stats keys). Toggle + NL-date PATCH round-tripped to .tsk.md in the
exact existing hand-editable format — the CLI/TUI storage contract is intact.

Roadmap status: F1-F15 done. F16-F22 unstarted, queued for T4 (bulk select,
drag-reorder, Cmd-K palette, export buttons, token auth) + T5 (SSE live-reload,
PWA).
