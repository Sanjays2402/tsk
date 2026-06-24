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
- [ ] **F6** Add-task form (header bar, `n` shortcut, Linear-ish slim
      input with priority/due/tag chips).
- [ ] **F7** Inline edit title (double-click or `e` on selected row).
- [ ] **F8** Delete with undo (toast + 5s timer).
- [ ] **F9** Sections: Overdue / Today / Upcoming / No Due / Done —
      matching the TUI's mental model.
- [ ] **F10** Keyboard nav: j/k to move selection, space/enter to toggle,
      `?` for help overlay (mirror the TUI hotkeys).

### T3 — search, filter, polish
- [ ] **F11** Filter bar: tag chips multi-select, priority filter, search
      box with fuzzy matching.
- [ ] **F12** Due-date picker that accepts natural-language strings
      (`tomorrow`, `fri`, `in 3d`, `eow`) — server validates via the
      existing `dateparse` package; UI shows the parsed date back.
- [ ] **F13** Stats sidebar: total / done / overdue / completion %,
      streak, top tags. Reuse `computeStatsDTO`.
- [ ] **F14** Theme toggle (auto/light/dark) honoring system + storing
      pref in localStorage. Dark = amber-on-charcoal.
      Light = ochre-on-cream.
- [ ] **F15** Tag pages: click a tag → filtered view; URL hash `#tag/dev`.

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
