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
- [x] **F16** Bulk select (shift+click range, multi-toggle/multi-delete).
      (tick T4 2026-06-24)
- [x] **F17** Drag-to-reorder within a section, with persistence (writes
      back to .tsk.md preserving order — `store.Tasks` slice IS the order).
      (tick T4 2026-06-24)
- [x] **F18** Cmd-K command palette: every action, fuzzy-find, keyboard-only.
      (tick T4 2026-06-24)
- [x] **F19** Export buttons (JSON / CSV / Markdown) — hit existing
      exporters via `/api/export?format=...`. (tick T4 2026-06-24)
- [x] **F20** Token auth for `--addr 0.0.0.0` use: `tsk serve --token=...`
      enforces `Authorization: Bearer` on `/api/*`. (tick T4 2026-06-24)

### T5 — production
- [x] **F21** SSE live-reload: server watches `.tsk.md` mtime; pushes
      change events to connected clients so multi-tab / external edits
      show up. (tick T5 2026-06-24)
- [x] **F22** PWA manifest + offline cache shell. Add-to-home on
      iOS/Android. (tick T5 2026-06-24)

### T6 — depth (queued so the loop never starves)
- [x] **F23** Notes editor: expand a task to edit its multi-line notes
      (the `.tsk.md` 6-space continuation lines) in a textarea, PATCH via
      the existing `notes` field. (tick T5 2026-06-24)
- [x] **F24** Settings drawer: persist per-client prefs (default sort,
      density compact/comfortable, show/hide done) in localStorage.
      (tick T5 2026-06-24)
- [x] **F25** Saved filters / views: name a filter+tag+priority combo and
      recall it from a chip row or the Cmd-K palette. (tick T5 2026-06-24)
- [ ] **F26** Dependency awareness in the UI: show `depends:` blockers on a
      row (the model already parses DependsOn), grey out blocked tasks, and
      surface "blocked by #N" — read-only first, then edit.
- [ ] **F27** Pinned tasks: respect the model's `Pinned` flag — a pin
      toggle on the row and a Pinned section that floats to the top.
- [ ] **F28** Mobile/touch polish: long-press to bulk-select, larger hit
      targets, a bottom action sheet instead of the floating bar.
- [ ] **F29** Inline priority cycling: click the priority chip to cycle
      L->M->H->U with an optimistic PATCH (no full edit needed).
- [ ] **F30** Search highlighting: in filter results, mark the matched
      substring/subsequence in the title so it's obvious why a row matched.

### T7 — depth (appended T5 2026-06-24 so the loop never starves)

Fresh slices. F26-F30 are the standing T6 backlog; these extend it with
follow-ons sensible after the T5 production cluster (live-reload, PWA,
notes, settings, saved views) plus long-tail UI the surface still lacks.

- [ ] **F31** Notes in the command palette + detail: a "view notes" preview
      and quick-jump; surface a notes snippet on the row (one-line, faded)
      when present so you don't have to open the editor to remember context.
- [ ] **F32** Saved-view enhancements: reorder views (drag the chips), an
      "update this view to the current filter" action, and persist the active
      view in the URL hash (`#view/<id>`) so it's shareable/bookmarkable.
- [ ] **F33** Live-reload polish: a subtle toast ("file changed on disk —
      refreshed") when an external edit lands, and a per-tab "pause live"
      toggle for when you're hand-editing the .tsk.md and don't want churn.
- [ ] **F34** Settings: a "reset to defaults" button, an export/import of the
      whole client config (settings + saved views) as a JSON blob, and a
      compact-mode density that also hides the meta cluster until hover.
- [ ] **F35** PWA depth: a real install button (beforeinstallprompt capture)
      in the settings drawer, plus an offline banner when the SSE stream is
      down AND the network is unreachable (distinguish "server restarting"
      from "you're offline").
- [ ] **F36** Bulk edit beyond toggle/delete: bulk set priority, bulk add/
      remove a tag, bulk set due — extend the floating bar with a small
      action menu over the existing bulk-selection model.
- [ ] **F37** Row context menu (right-click / long-press): every per-row
      action (edit, due, notes, pin, clone, delete) in one menu, sharing the
      command dispatch the palette already uses.
- [ ] **F38** Quick-add upgrades: a `depends:#N` token in the composer, an
      autocomplete dropdown for `#tags` (from collectTags) and `@due`
      presets, and a multi-line paste that splits into N tasks.

When fewer than 5 remain, append more (recurring-task UI, archive view,
quick-jump to a tag from the palette, undo stack beyond single delete, etc.).

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

### T4 — 2026-06-24 08:07 PT — power user (5/5)

Workdir note: the canonical `/Volumes/Projects/tsk` (external SSD sparseimage)
was again NOT mounted this tick — `/Volumes` held only Macintosh HD and
`diskutil mount Projects` failed with "Failed to find disk" (the SSD is
physically absent). Worked from the fully-synced internal worktree
`/Users/sanjay/Projects/tsk-features/main` (same origin, was exactly at
origin/main 69b49f8, clean tree, .cron-state tracked in git so no state
divergence). Pushed as a clean fast-forward (69b49f8..2ee4265), zero
divergence. This remains the documented fallback (see T2/T3 notes).

- F16 feat(web): bulk select rows for multi-toggle / multi-delete (27d23eb)
- F17 feat(web): drag-to-reorder tasks, persisted to .tsk.md (88308ff)
- F18 feat(web): Cmd-K command palette, fuzzy find + run any action (277498c)
- F19 feat(web): export tasks as JSON / CSV / Markdown (560db29)
- F20 feat(serve): optional bearer-token auth for off-loopback serving (3e8d053)
- (+ chore 2ee4265: rebuilt embedded SPA bundle for all five slices)

Backend changes this tick: store.Move(moved, before) reorder primitive (7
tests) + POST /api/tasks/:id/move (F17); GET /api/export?format=json|csv|md
mirroring the CLI exporters with Content-Disposition (F19, 8 tests); a
requireAuth/tokenBootstrap middleware layer gating /api/* behind a constant-
time bearer token with an HttpOnly SameSite=Strict cookie bootstrap, the
?token= query rejected on /api/* to avoid leaks (F20, 11 tests). Everything
else is pure frontend over the existing JSON API. Each frontend slice ships a
dependency-free logic module unit-tested under node --test: bulkselect.ts (16),
reorder.ts (14), palette.ts (14), export.ts (6) = 50 new web tests (156 total).

Gates (run once at end of batch): gofmt -l clean, go vet ./... clean, go build
./... ok, go test ./... ok across ALL packages (incl. internal/serve with the
freshly-embedded T4 bundle, and internal/commands 56.9s), web `npm run check`
156 pass, `npm run build` ok — JS 14.0KB gz, CSS 5.5KB gz, 22 modules.

End-to-end proof: built the binary, ran tsk init + 3 tasks, then `tsk serve
--addr 127.0.0.1:7891 --token testtok123` and exercised the live surface:
no-token /api/tasks -> 401 with WWW-Authenticate; Bearer -> 200 list;
?token= on /api/* -> 401 (rejected as designed); POST /api/tasks/3/move
{before:1} reordered charlie->alpha->bravo and the .tsk.md on disk rewrote to
that exact file order; /api/export csv+markdown returned the right shapes with
priority glyphs and Content-Disposition; GET /?token= -> 303 to clean / with an
HttpOnly tsk_token cookie that then round-tripped /api/tasks -> 200; the SPA
shell stayed public (200 with no token); and the served bundle contained the
new slice code (data-bulk-toggle, reorder, data-cmd-id, data-export-format,
/move). The hand-editable .tsk.md storage contract is intact — reorder only
rewrites task-line order, nothing else.

Roadmap status: F1-F20 done. F21-F22 queued for T5 (SSE live-reload, PWA);
F23-F30 appended this tick for T6+ (notes editor, settings drawer, saved
views, dependency UI, pinned tasks, mobile polish, inline priority cycle,
search highlighting) so the loop never starves.

### T5 — 2026-06-24 09:06 PT — production (5/5)

Workdir note: the canonical `/Volumes/Projects/tsk` (external SSD sparseimage)
was again NOT mounted this tick — `/Volumes` held only Macintosh HD and the
SSD is physically absent. Worked from the fully-synced internal worktree
`/Users/sanjay/Projects/tsk-features/main` (same origin, was exactly at
origin/main a67d69d, clean tree, .cron-state tracked in git so no state
divergence). Pushed as a clean fast-forward (a67d69d..25e2b3f), zero
divergence. This remains the documented fallback (see T2/T3/T4 notes).

- F21 feat(serve): SSE live-reload so external edits flow into open tabs (1a2d324)
- F22 feat(web): PWA manifest + offline service worker shell (b346e8e)
- F23 feat(web): multi-line notes editor with PATCH round-trip (2c78203)
- F24 feat(web): settings drawer for per-client preferences (0612443)
- F25 feat(web): saved views — name a filter combo, recall from chips/Cmd-K (8efa8a0)
- (+ chore 25e2b3f: rebuilt embedded SPA bundle for all five slices)

Backend changes this tick: GET /api/events, a Server-Sent Events stream that
fingerprints .tsk.md (mtime+size+existence) on a 1s os.Stat poll and emits a
"change" event when it moves — dependency-free (no fsnotify), behind requireAuth
like every /api/* route, with a 15s keep-alive comment (F21); plus a
.webmanifest MIME registration in embed.go so the PWA manifest serves as
application/manifest+json (F22). F23 reuses the existing PATCH `notes` field
(no backend change). F24/F25 are pure client-side localStorage prefs that never
touch .tsk.md. Each frontend slice ships a dependency-free logic module
unit-tested under node --test: live.ts (8), pwa.ts (7), notes.ts (12),
settings.ts (8), views.ts (14) = 49 new web tests (205 total). Go side adds
events_test.go (statSig/sigChanged + a race-clean ready-then-change stream test
via a mutex-guarded flush recorder + context-cancel + goroutine join).

Gates (run once at end of batch): gofmt -l clean, go vet ./... clean, go build
./... ok, go test -race ./... ok across ALL packages (incl. internal/serve with
the freshly-embedded T5 bundle and the new SSE handler, internal/commands ~60s),
web `npm run check` 205 pass, `npm run build` ok — JS 18.1KB gz, CSS 6.58KB gz,
27 modules.

End-to-end proof: built the binary, ran tsk init + 2 tasks + `tsk serve`. F22:
/manifest.webmanifest -> 200 application/manifest+json, /sw.js -> 200
text/javascript, /icon.svg + /icon-maskable.svg -> 200 image/svg+xml, index
carries the manifest link. F21: a curl -N on /api/events emitted `event: ready`
then `event: change` with a bumped mtime+size the instant `tsk add` rewrote the
file. F23: a multi-line notes PATCH (with an interior blank line) round-tripped
to .tsk.md as 6-space continuation lines and `tsk show 1` read the same notes
back — the hand-editable storage contract is intact. The served bundle contains
the new slice code (data-live, serviceWorker, data-notes, data-settings,
data-view-recall).

Roadmap status: F1-F25 done. F26-F30 (T6) + F31-F38 (T7, appended this tick)
unstarted — dependency UI, pinned tasks, mobile polish, inline priority cycle,
search highlighting, then notes-on-row, saved-view URL hash, live-reload toast,
config export/import, install button, bulk edit, context menu, quick-add
upgrades — so future ticks have ample sized work.
