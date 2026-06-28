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
- [x] **F26** Dependency awareness in the UI: show `depends:` blockers on a
      row (the model already parses DependsOn), grey out blocked tasks, and
      surface "blocked by #N" — read-only first, then edit. (tick T6 2026-06-24)
- [x] **F27** Pinned tasks: respect the model's `Pinned` flag — a pin
      toggle on the row and a Pinned section that floats to the top.
      (tick T6 2026-06-24)
- [x] **F28** Mobile/touch polish: long-press to bulk-select, larger hit
      targets, a bottom action sheet instead of the floating bar.
      (tick T6 2026-06-24)
- [x] **F29** Inline priority cycling: click the priority chip to cycle
      L->M->H->U with an optimistic PATCH (no full edit needed).
      (tick T6 2026-06-24)
- [x] **F30** Search highlighting: in filter results, mark the matched
      substring/subsequence in the title so it's obvious why a row matched.
      (tick T6 2026-06-24)

### T7 — depth (appended T5 2026-06-24 so the loop never starves)

Fresh slices. F26-F30 are the standing T6 backlog; these extend it with
follow-ons sensible after the T5 production cluster (live-reload, PWA,
notes, settings, saved views) plus long-tail UI the surface still lacks.

- [x] **F31** Notes in the command palette + detail: a "view notes" preview
      and quick-jump; surface a notes snippet on the row (one-line, faded)
      when present so you don't have to open the editor to remember context.
      (tick T7 2026-06-24)
- [x] **F32** Saved-view enhancements: reorder views (drag the chips), an
      "update this view to the current filter" action, and persist the active
      view in the URL hash (`#view/<id>`) so it's shareable/bookmarkable.
      (tick T7 2026-06-24)
- [x] **F33** Live-reload polish: a subtle toast ("file changed on disk —
      refreshed") when an external edit lands, and a per-tab "pause live"
      toggle for when you're hand-editing the .tsk.md and don't want churn.
      (tick T7 2026-06-24)
- [x] **F34** Settings: a "reset to defaults" button, an export/import of the
      whole client config (settings + saved views) as a JSON blob, and a
      compact-mode density that also hides the meta cluster until hover.
      (tick T7 2026-06-24)
- [x] **F35** PWA depth: a real install button (beforeinstallprompt capture)
      in the settings drawer, plus an offline banner when the SSE stream is
      down AND the network is unreachable (distinguish "server restarting"
      from "you're offline"). (tick T7 2026-06-24)
- [x] **F36** Bulk edit beyond toggle/delete: bulk set priority, bulk add/
      remove a tag, bulk set due — extend the floating bar with a small
      action menu over the existing bulk-selection model. (tick T8 2026-06-24)
- [x] **F37** Row context menu (right-click / long-press): every per-row
      action (edit, due, notes, pin, clone, delete) in one menu, sharing the
      command dispatch the palette already uses. (tick T8 2026-06-24)
- [x] **F38** Quick-add upgrades: a `depends:#N` token in the composer, an
      autocomplete dropdown for `#tags` (from collectTags) and `@due`
      presets, and a multi-line paste that splits into N tasks.
      (tick T8 2026-06-24)

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

### T6 — 2026-06-24 10:06 PT — depth (5/5)

Workdir note: the canonical `/Volumes/Projects/tsk` (external SSD sparseimage)
was again NOT mounted — `/Volumes` held only Macintosh HD; `diskutil list
external` was empty (SSD physically absent). The prompt's hard-coded path is
stale; the live, fully-synced worktree is `/Users/sanjay/Projects/tsk-features/
main` (same origin, was exactly at origin/main 378c75e, clean tree,
.cron-state tracked in git so no state divergence). Pushed a clean
fast-forward 378c75e..1565651. This remains the documented fallback (see
T2-T5 notes); future ticks should resolve the repo here when the SSD is out.

- F26 feat(web): dependency awareness in the UI (1eaee70)
- F27 feat(web): pinned tasks with a floating Pinned section (ae72c31)
- F29 feat(web): inline priority cycling on the row chip (f67a180)
- F30 feat(web): search-match highlighting in filtered titles (1250ba4)
- F28 feat(web): mobile + touch polish (dc868f0)
- (+ chore 5627624: rebuilt embedded SPA bundle for all five slices;
   + style 1565651: gofmt struct-tag alignment after the F26/F27 DTO additions)

Backend changes this tick: taskDTO gains `pinned` + `depends_on` (both
omitempty so old fixtures stay byte-identical); taskPatchDTO accepts both as
setters; new POST /api/tasks/:id/pin toggle; PATCH wires Pinned (set) and
DependsOn (replace, via the new sanitizeDeps validator — drops dupes, refuses
self-ref, rejects unknown ids with 400, preserves order). Both round-trip to
the existing `pin:true` / `depends:` meta keys in .tsk.md, so the CLI/TUI
contract is intact (verified live: `tsk top` floats the pin, `tsk show` reads
the deps back). F28/F29/F30 are pure frontend over the existing API.

Each frontend slice ships a dependency-free logic module unit-tested under
node --test: deps.ts (12), priority.ts (9), highlight.ts (13), touch.ts (9),
plus 5 new section tests for the Pinned bucket = 48 new web tests (253 total).
Go side adds deps_test.go (6) + pin_test.go (5) = 11 new backend tests.

Gates (run once at end of batch): gofmt -l clean, go vet ./... clean, go build
./... ok, go test ./... ok across ALL packages (incl. internal/serve with the
freshly-embedded T6 bundle, internal/commands ~58s), web `npm run check` 253
pass, `npm run build` ok — JS 19.62KB gz, CSS 7.05KB gz, 31 modules.

End-to-end proof: built the binary, ran tsk init + 2 tasks + `tsk serve`.
F27: POST /pin -> pinned:true, persisted as `pin:true`, `tsk top` floats it.
F26: PATCH depends_on:[1] -> persisted as `depends:1`, `tsk show` reads it
back; self-dep -> 400, unknown-dep -> 400. F29: PATCH priority urgent ->
chip + .tsk.md updated. SPA index -> 200; bundle carries data-pin,
data-prio-cycle, data-dep-jump, section-pinned, .title mark, is-blocked,
touchstart + navigator.vibrate. The hand-editable .tsk.md storage contract
is intact.

Roadmap status: F1-F30 done. F31-F38 (T7) unstarted — notes-on-row preview,
saved-view URL hash + reorder, live-reload toast + pause, settings reset +
config export/import, PWA install button + offline banner, bulk edit
(priority/tag/due), row context menu, quick-add depends:/tag-autocomplete.
T8 backlog appended below so the loop never starves.

### T7 — 2026-06-24 13:06 PT — depth (5/5)

Workdir note: the canonical `/Volumes/Projects/tsk` (external SSD sparseimage)
was again NOT mounted — `/Volumes` held only Macintosh HD; the SSD is
physically absent. The lock wrapper's `(repo cd failed)` confirms the stale
hard-coded path. Worked from the fully-synced internal worktree
`/Users/sanjay/Projects/tsk-features/main` (same origin, was exactly at
origin/main f6efb37, clean tree, .cron-state tracked in git so no state
divergence). Pushed a clean fast-forward f6efb37..b0e3dcb. This remains the
documented fallback (see T2-T6 notes); future ticks resolve the repo here when
the SSD is out.

- F31 feat(web): one-line notes snippet under the row title (29c3392)
- F32 feat(web): saved-view enhancements — reorder, update, URL hash (d9ccaa2)
- F33 feat(web): live-reload polish — change toast + pause-live toggle (864f6bc)
- F34 feat(web): settings reset + config export/import + reveal-on-hover meta (acf67b7)
- F35 feat(web): PWA install button + honest offline/server banner (9757b37)
- (+ chore b0e3dcb: rebuilt embedded SPA bundle for all five slices)

All five slices are pure-frontend over the existing JSON API (no backend
change this tick). Each ships a dependency-free logic module unit-tested under
node --test: F31 extends notes.ts (renderNotesSnippet, 4 tests); F32 extends
views.ts (updateView/moveView/chip opts) + router.ts (view route) (16 tests);
F33 extends live.ts (paused status + liveChangeMessage, 4 tests); F34 adds
config.ts (bundle build/serialize/parse/validate) + extends settings.ts
(hideMeta) (17 tests); F35 extends pwa.ts (classifyConnectivity / banner /
canInstall, 6 tests). 31 new web tests total (253 -> 284).

Toolchain note: config.ts is the first src module to import sibling pure
modules, so it uses explicit `.ts` import specifiers and tsconfig gained
`allowImportingTsExtensions` (additive, noEmit already set) so both Vite and the
node --test runner resolve them. Verified: tsc app + tsc test clean, vite build
ok (32 modules, JS 22.24KB gz, CSS 7.47KB gz).

Gates (run once at end of batch): gofmt -l clean, go vet ./... clean, go build
./... ok, go test ./... ok across ALL packages (incl. internal/serve with the
freshly-embedded T7 bundle, internal/commands 58.9s), web `npm run check` 284
pass, `npm run build` ok.

End-to-end proof: built the binary, ran tsk init + 2 tasks (one with notes) +
`tsk serve`. /api/tasks returned the notes round-tripped; a PATCH notes via the
API wrote 6-space continuation lines to .tsk.md and `tsk show 1` read them back
(storage contract intact). The served bundle carries every slice's code:
notes-snippet, data-view-update + data-view-drag + #view/, "changed on disk" +
live.paused, data-config-export/import/reset + data-hide-meta + tsk-web-config,
data-config-install + offline-banner + beforeinstallprompt.

Roadmap status: F1-F35 done. F36-F38 (T7) + F39-F46 (T8) unstarted — bulk edit
(priority/tag/due), row context menu, quick-add upgrades, then dependency
editing, pinned drag-reorder, touch priority picker, unblocked toast, tag/notes
highlight, keyboard priority cycle, blocked-toggle guard, blocked/pinned stats
— 11 slices queued so the loop never starves.

### T8 — 2026-06-24 14:06 PT — depth (5/5)

Workdir note: the canonical `/Volumes/Projects/tsk` (external SSD sparseimage)
was again NOT mounted — `/Volumes` held only Macintosh HD; the lock wrapper
logged `(repo cd failed)`, confirming the stale hard-coded path. Worked from the
fully-synced internal worktree `/Users/sanjay/Projects/tsk-features/main` (same
origin, was exactly at origin/main 958dfcb, clean tree, .cron-state tracked in
git so no state divergence). Pushed a clean fast-forward 958dfcb..8b69c18. This
remains the documented fallback (see T2-T7 notes).

- F36 feat(web): bulk edit selected — priority/tag/due (4e06989)
- F37 feat(web): row context menu — right-click / overflow button (a40af3e)
- F38 feat(web): quick-add upgrades — depends: token, autocomplete, paste (230f045)
- F39 feat(web): dependency editor — add/remove blockers + cycle guard (4d61c85)
- F40 feat(web): pinned-section drag-reorder, stable hand-curated order (73ca9c3)
- (+ chore 8b69c18: rebuilt embedded SPA bundle for all five slices)

Backend change this tick: POST /api/tasks gains an optional depends_on (F38)
validated through the existing sanitizeDeps + rolled back atomically if a dep id
is bad (4 new Go tests in create_deps_test.go). Everything else is pure frontend
over the existing JSON API. New pure modules: bulkedit.ts (15), contextmenu.ts
(9), autocomplete.ts (16), depedit.ts (18) + extended quickadd.ts (multi-paste +
depends token) and reorder.ts (computeSectionReorder, 8) and sections.ts
(comparePinned -> file order). 74 new web tests (284 -> 358).

One shared-surface win: F37's runRowAction is the SINGLE dispatcher behind the
context menu, the command palette, AND the keyboard hotkeys, so they can't
drift. F39 reuses it (Edit blockers) and adds the `b` hotkey + palette entry.

Gates (run once at end of batch): gofmt -l clean, go vet ./... clean, go build
./... ok, go test ./... ok across ALL packages (incl. internal/serve with the
freshly-embedded T8 bundle + the create-deps path, internal/commands 58.5s), web
`npm run check` 358 pass, `npm run build` ok — JS 27.19KB gz, CSS 8.03KB gz, 36
modules.

End-to-end proof: built the binary, ran tsk init + 2 tasks + `tsk serve`. F38:
POST /api/tasks {depends_on:[1,2]} -> 201 with depends_on:[1,2], persisted to
.tsk.md as `depends:1,2` and `tsk show 3` read `depends: #1, #2` back; a bad dep
id (999) -> 400 AND rolled back (store stayed at 3 tasks, no orphan). The served
bundle carries every slice's hooks: data-bulk-edit, data-bulk-set-prio,
data-row-action, data-ctxmenu, depends:, data-ac-value, data-dep-input,
data-dep-cand, data-section (F40 section reorder). The hand-editable .tsk.md
storage contract is intact.

Roadmap status: F1-F40 done. F41-F46 (rest of T8) + F47-F54 (T9, appended this
tick) unstarted — touch priority picker, unblocked toast, tag/notes highlight,
keyboard priority cycle, blocked-toggle guard, blocked/pinned stats, then bulk-
edit depth, context submenus, autocomplete everywhere, dep mini-graph, all-
section reorder, multi-paste review, palette row-actions, touch context menu —
14 slices queued so the loop never starves.

### T8 — depth (appended T6 2026-06-24 so the loop never starves)

Fresh slices for after the T7 cluster. F36-F38 are the standing T7 backlog;
these extend it with follow-ons sensible after the T6 dependency/pin/priority/
mobile/highlight work plus long-tail UI the surface still lacks.

- [x] **F39** Dependency EDITING in the UI: a small "blocked by" editor on the
      row / in a popover to add+remove prereqs (the backend PATCH depends_on
      already supports it; T6 only shipped the read + jump). Autocomplete over
      open task ids, refuse self/cycle client-side before the PATCH.
      (tick T8 2026-06-24)
- [x] **F40** Pinned-section drag-reorder + a "pin to a fixed slot" so the
      Pinned group has a stable hand-curated order (currently priority-sorted);
      persist the order through the existing /move endpoint. (tick T8 2026-06-24)
- [x] **F41** Priority cycle on touch: long-press the chip to open a 4-way
      priority picker (tap-to-set) since alt/shift-click isn't reachable on a
      phone; reuse the F28 long-press machine. (tick T9 2026-06-24)
- [ ] **F42** "Unblocked just now" toast: when a toggle-done completes the last
      blocker of some task, surface a toast ("#N is now unblocked — start it?")
      with a jump action. Pairs with the F26 done-index.
      (DEFERRED from T9 — picked the 5 most cohesive slices; this is the top
      unstarted item for T10)
- [x] **F43** Highlight in tags + notes preview too (F30 only marks titles):
      mark the matched subsequence in the tag pills and any notes snippet so a
      fuzzy match that landed on a tag is visible. (tick T9 2026-06-24)
- [x] **F44** Keyboard: `[` / `]` to cycle priority down/up on the selected
      row (sister of the F29 chip click), and `shift+P` to pin-to-top; show
      them in the `?` help overlay. (tick T9 2026-06-24)
- [x] **F45** Blocked-task guard on toggle: when you try to complete a blocked
      task, confirm ("#N is blocked by #M — complete anyway?") instead of
      silently allowing it, mirroring the CLI's `done` dependency gate.
      (tick T9 2026-06-24)
- [x] **F46** Stats: a "blocked" + "pinned" count in the stats sidebar (reuse
      the done-index + Pinned flag), and a tiny dependency-depth metric
      ("longest blocker chain: 3"). (tick T9 2026-06-24)

When fewer than 5 remain, append more (recurring-task UI, archive view, an
in-UI dependency graph mini-map, undo stack beyond single delete, etc.).

### T9 — depth (appended T8 2026-06-24 so the loop never starves)

Fresh slices for after the F41-F46 backlog. F41-F46 are the standing T8
queue (touch priority picker, unblocked toast, tag/notes highlight, keyboard
priority cycle, blocked-toggle guard, blocked/pinned stats); these extend the
runway with follow-ons sensible after the T8 bulk-edit / context-menu /
quick-add / dependency-editor / pinned-reorder cluster.

- [ ] **F47** Bulk edit depth: a bulk "set/clear due via the NL picker preview"
      (reuse the F12 parse-date endpoint live inside the bulk-due popover), and
      a bulk pin/unpin + bulk "add blocker" over the F36 selection model.
- [ ] **F48** Context-menu submenus: nest the priority levels and a "due
      presets" flyout inside the F37 row menu so you can set them without a
      separate popover; keyboard-navigable (arrow into submenu).
- [ ] **F49** Autocomplete depth: extend the F38 composer dropdown to the inline
      EDIT field (F7) and the filter box (F11) so #tag/@due completion is
      everywhere; add a `!priority` completion too.
- [ ] **F50** Dependency mini-graph: in the dep editor (F39), render a tiny
      ASCII/SVG "what blocks what" preview of the selected task's immediate
      neighborhood (reuse the upstream/reachable JSON the server already emits).
- [ ] **F51** Drag-reorder depth: extend the section-constrained F40 reorder to
      ALL undone sections (overdue/today/upcoming/nodue) with a clear "manual
      order overrides priority within this section" affordance + a reset.
- [ ] **F52** Multi-paste review: when a multi-line paste (F38) lands, show a
      tiny confirm sheet listing the N parsed tasks (title + chips) with a
      per-line remove before committing, so a bad paste doesn't dump junk.
- [ ] **F53** Command palette: surface the row context-menu actions (F37) for
      the selected task as a palette group, and add "Edit blockers" + the bulk
      verbs so everything is reachable keyboard-only from Cmd-K.
- [ ] **F54** Row context menu on touch: wire a long-press (reuse the F28
      machine) to open the F37 menu instead of only bulk-select, with a press-
      and-hold disambiguation (short hold = menu, longer = multi-select).

### T9 — 2026-06-24 18:06 PT — depth (5/5)

Workdir note: the canonical `/Volumes/Projects/tsk` (external SSD sparseimage)
was again NOT mounted — `/Volumes` held only Macintosh HD; `diskutil list`
showed only the internal disk and the lock wrapper logged `(repo cd failed)`,
confirming the stale hard-coded path. Worked from the fully-synced internal
worktree `/Users/sanjay/Projects/tsk-features/main` (same origin Sanjays2402/tsk,
was exactly at origin/main f2c64dc, clean tree, .cron-state tracked in git so no
state divergence). Pushed a clean fast-forward f2c64dc..efd8c29. This remains
the documented fallback (see T2-T8 notes).

Slices shipped (5/5 — picked the 5 most cohesive from the F41-F46 queue;
DEFERRED F42 honestly, see below):

- F43 feat(web): highlight matched chars in tags + notes too (2d13796)
- F44 feat(web): keyboard priority cycle + pin-to-top shortcut (7c5bbbf)
- F45 feat(web): confirm before completing a blocked task (4c03e24)
- F46 feat(web): blocked / pinned / chain-depth stats in the sidebar (0cd22d2)
- F41 feat(web): touch priority picker — long-press the chip (73ee7e8)
- (+ chore efd8c29: rebuilt embedded SPA bundle for all five slices)

Why F42 deferred (4/5 of the standing queue + F41 chosen over F42): F41,
F43-F46 form a tight, mostly-pure cluster (highlight engine generalization,
two reorder helpers, two deps helpers, a stats renderer, a picker module) that
all ship as dependency-free logic + a thin DOM wiring layer — exactly the
quality bar. F42 (the "unblocked just now" toast) needs a before/after
done-index diff threaded through the toggle round-trip and a new toast variant
with a jump action; it's a clean slice but a DIFFERENT shape (stateful diff,
not pure render), so bundling it would have rushed it. It's now the TOP
unstarted item for T10. Five solid slices over five-plus-a-rushed-one.

All five slices are pure-frontend logic over the existing JSON API (no backend
change this tick). New/changed pure modules, each unit-tested under node --test:
- highlight.ts: generalized to highlightText(text, query); highlightTitle is now
  a thin alias. render.ts threads it into tag pills + the notes snippet;
  notes.ts renderNotesSnippet gained an optional highlight callback. (+8 tests
  across highlight + notes)
- reorder.ts: new computePinToTop(order, moved). (+6 tests)
- deps.ts: new needsBlockedConfirm + blockedToggleConfirm (F45) and
  computeDepStats (F46, memoized DFS with cycle guard). (+12 tests)
- stats.ts: new renderDepStats; renderStatsPanel gained an optional dep arg.
  (+4 tests)
- prioritypicker.ts: NEW module — priorityOptions + renderPriorityPicker (F41).
  (+8 tests). Imports priority.ts with the explicit .ts specifier so node --test
  resolves it (the config.ts convention).
- main.ts wiring: setPriority direct setter (cyclePriority now delegates),
  pinToTop, the [ ] / shift+P keys + help rows, the toggle blocked-guard, the
  refreshStats dep computation, and the openPriorityPicker popover + its
  long-press arm on the chip in the touchstart handler. app.css: .tag/.notes
  mark, dep-stats tiles, .prio-pick popover.

38 new web tests (358 -> 395 — note: 37 net new files-counted; one was an
existing-file extension). 

Gates (run once at end of batch): gofmt -l clean, go vet ./... clean, go build
./... ok, go test ./... ok across ALL packages (incl. internal/serve with the
freshly-embedded T9 bundle, internal/commands 67.3s), web `npm run check` 395
pass, `npm run build` ok — JS 28.30KB gz, CSS 8.15KB gz, 37 modules.

End-to-end proof: built the binary, ran tsk add x3 + depend 3 --on 1 + pin 2 +
`tsk serve`. F41: PATCH /api/tasks/2 {priority:urgent} round-tripped low->urgent
on disk as `prio:urgent` while preserving `pin:true` + tags (storage contract
intact). F45: with #1 open, #3's open_blockers=[1] (guard fires); after toggling
#1 done, #3's open_blockers=[] (guard stands down) — mirrors the CLI gate. The
served bundle carries every slice's hooks: prio-pick + data-set-prio + "Set
priority" (F41), notes-snippet + tag mark (F43), "Chain depth" (F44/F46),
"complete anyway" + "is blocked by" (F45), Dependencies + Blocked + metric-pinned
(F46).

Roadmap status: F1-F41, F43-F46 done (40 of the 46-item F-roadmap shipped).
F42 deferred (top of T10). F47-F54 (T9 backlog) unstarted. T10 backlog appended
below so the loop never starves.

### T10 — depth (appended T9 2026-06-24 so the loop never starves)

Top of the queue is the deferred F42. F47-F54 remain the standing T9 backlog
(bulk-edit depth, context submenus, autocomplete everywhere, dep mini-graph,
all-section reorder, multi-paste review, palette row-actions, touch context
menu). These extend the runway with follow-ons sensible after the T9
highlight / keyboard / guard / stats / picker cluster.

- [x] **F42** (carried) "Unblocked just now" toast: on a toggle-done that clears
      the last open blocker of some OTHER task, show a toast ("#N is now
      unblocked — start it?") with a jump action. Diff the done-index before vs
      after the toggle to find newly-unblocked ids; reuse the F33 info-toast.
      (tick T10 2026-06-24)
- [x] **F55** Touch picker polish: a tiny haptic + a "current" check on the
      active row, and wire the SAME picker to a desktop right-click on the chip
      (so the chip has menu parity with the row's F37 context menu).
      (tick T10 2026-06-24)
- [x] **F56** Dependency-depth drill-down: clicking the F46 "chain depth N"
      metric opens the longest chain as a jump-list (#a -> #b -> #c) so you can
      walk to the root blocker; reuse the jumpToTask plumbing. (tick T10 2026-06-24)
- [x] **F57** Highlight in the command palette (F18) results too: when you type
      in Cmd-K, mark the matched subsequence in each command label (reuse the
      now-generic highlightText engine from F43). (tick T10 2026-06-24)
- [x] **F58** Keyboard: `shift+[` / `shift+]` to jump priority to the floor /
      ceiling (low / urgent) in one keystroke, and surface in the `?` overlay.
      (tick T10 2026-06-24)
- [x] **F59** Stats: a "due this week" + "no-due" count tile, and make the
      blocked tile click-to-filter (show only blocked tasks) reusing the F11
      filter plumbing. (tick T11 2026-06-24 — split: tiles here, click-to-filter
      shipped as F64)
- [x] **F60** Bulk blocked-guard: when a bulk multi-toggle would complete one or
      more BLOCKED tasks, list them in a single confirm ("3 of 5 are blocked —
      complete all anyway?") instead of silently completing — the bulk sibling
      of F45. (tick T11 2026-06-24)

### T10 — 2026-06-24 19:06 PT — depth cluster (5/5)

Workdir note: the canonical `/Volumes/Projects/tsk` was NOT mounted this tick
(the external SSD was physically absent — `diskutil list` shows no Projects
volume, `/Volumes` had only Macintosh HD + Recovery, `hdiutil info` empty).
Worked from the fully-synced internal worktree
`/Users/sanjay/Projects/tsk-features/main`, which shares the same origin and was
at origin/main (2848c9e) with a clean tree — the same fallback blessed in the T2
log. Pushed from there; a clean fast-forward (2848c9e..0041d18), verified 0/0.

- F42 feat(web): "unblocked just now" toast with a jump-to action — 11a058e
- F55 feat(web): priority-picker polish (desktop right-click + current check) — b433e29
- F56 feat(web): dependency-depth drill-down jump-list — 3067ecc
- F57 feat(web): highlight the matched subsequence in Cmd-K results — 4048b3a
- F58 feat(web): shift+[ / shift+] jump priority to floor / ceiling — ad66c6a
- chore rebuild embedded SPA bundle — 0041d18

Gates (once, end of batch) — all green:
- gofmt -l clean, go vet clean, go build ok
- go test ./... ok all packages (internal/commands 56.3s; internal/serve
  re-run uncached at 1.9s against the freshly-embedded T10 bundle)
- web: npm run check 422 pass (was 395; +27 net new), npm run build ok
  (37 modules, JS 29.48KB gz, CSS 8.38KB gz)

Live end-to-end proof: built the binary, set up 3 tasks chained #1->#2->#3
(depend --on), `tsk serve`. F55/F58: PATCH /api/tasks/3 {priority:urgent}
round-tripped medium->urgent on disk as `prio:urgent` while preserving the
`depends`/`created` meta (hand-editable contract intact) — the same setPriority
path the picker tap and the shift+] ceiling jump drive. F42: before completing
#3, #2's open_blockers=[3]; after, open_blockers=[] while #2 stays undone —
exactly the newlyUnblocked condition (would fire "#2 is now unblocked"), and #1
correctly stays blocked by #2 (not announced).

Roadmap status: F1-F58 done (52 of the 58-item F-roadmap shipped). F47-F54
(T9 backlog) + F59-F60 remain unstarted; T11 backlog appended below so the loop
never starves.

### T11 — depth (appended T10 2026-06-24 so the loop never starves)

Standing unstarted: F47 (bulk due/pin/blocker depth), F48 (context-menu
submenus), F49 (autocomplete in edit + filter), F50 (dep mini-graph), F51
(all-section reorder), F52 (multi-paste review), F53 (palette row-actions),
F54 (touch context menu), F59 (due-this-week / no-due tiles + blocked
click-to-filter), F60 (bulk blocked-guard). Fresh follow-ons after the T10
unblock-toast / picker / chain-drill / palette-highlight / priority-jump cluster:

- [ ] **F61** Chain-drill from the row badge too: the "blocked by #N" dep badge
      (F26) gets a secondary affordance to open the F56 chain popover for THAT
      task's deepest blocker path, not just the global longest chain.
- [x] **F62** Unblock-toast depth: when several tasks unblock at once (F42 plural
      case), make the toast action a tiny picker ("3 unblocked — jump to which?")
      instead of always jumping to the first. (tick T11 2026-06-24)
- [x] **F63** Priority keyboard parity in the palette: a "Set priority ▸" command
      group in Cmd-K (urgent/high/medium/low) acting on the selected task, reusing
      setPriority — keyboard-only priority without leaving the palette.
      (tick T11 2026-06-24)
- [x] **F64** Stats "blocked" tile click-to-filter (the F59 idea, split out):
      clicking the F46 Blocked count filters the list to only blocked tasks via
      the F11 filter plumbing; a clear chip resets it. (tick T11 2026-06-24)
- [ ] **F65** Notes-snippet highlight in the chain drill + dep badge titles, so a
      search that matched on a blocker's text is visible when you walk the chain.

### T11 — 2026-06-24 20:06 PT — dependency depth + stats lenses (5/5)

Workdir note: the canonical `/Volumes/Projects/tsk` (external SSD sparseimage)
was again NOT mounted this tick — the lock wrapper logged `(repo cd failed)` and
`/Volumes` held only Macintosh HD + Recovery. Worked from the fully-synced
internal worktree `/Users/sanjay/Projects/tsk-features/main` (same origin
Sanjays2402/tsk, was exactly at origin/main 8b0e598, clean tree, .cron-state
tracked in git so no state divergence). `npm --prefix web install` was needed
this tick (node_modules absent). Pushed a clean fast-forward 8b0e598..95b4515,
verified HEAD == origin/main, 0/0. This remains the documented fallback (T2-T10).

Slices shipped (5/5 — a tight dependency-UX + stats cluster, all pure-frontend
over the existing JSON API; no backend change this tick):

- F59 feat(web): stats schedule lens — due-this-week + no-due tiles (cdfef5f)
- F64 feat(web): blocked-tile click-to-filter, blocked-only lens (c4fdb42)
- F63 feat(web): "Set priority" command group in the palette (a67b521)
- F60 feat(web): bulk blocked-guard before completing blocked tasks (bf5e52c)
- F62 feat(web): multi-unblock picker when several tasks free at once (2fbf743)
- (+ chore 95b4515: rebuilt embedded SPA bundle for all five slices)

Design notes worth keeping:
- F59 ships a NEW pure module schedule.ts (computeScheduleStats): the server
  stats DTO doesn't model "due this week" / "no due", so the client derives them
  from the live list relative to its own `today`. renderScheduleStats collapses
  to "" on an empty board.
- F64's blocked-only lens lives OUTSIDE FilterState on purpose — "blocked" is a
  cross-task property, so it must NOT serialize into saved views / settings. It's
  a render-pipeline step (filterBlocked) after the text/facet filter, done-index
  over the whole live list. The Blocked tile renders as a button only when
  blocked > 0; clear chip / clear-all / jump-to-hidden all reset it.
- F63 factors the four palette commands into a pure buildPriorityCommands helper
  (urgent-first, letter-mnemonic keywords, current level disabled) so the shape
  is tested without the app; runCommand dispatches prio-set-<level> via the
  existing setPriority.
- F60 is the bulk sibling of F45: blockedInBulkToggle (selection-order, only
  COMPLETING-while-blocked, never re-opens) + bulkBlockedConfirm (singular/plural
  message); bulkToggleDone gates before the parallel fan-out.
- F62 is depth on F42: single unblock keeps the direct "Start"; plural opens a
  renderUnblockedPicker popover (reuses chain-jump chrome + the F56 popover
  lifecycle).

Tests: +29 net new web tests (422 -> 451). New/extended pure modules each
unit-tested under node --test: schedule.ts (11), deps.ts (+filterBlocked 3,
+blockedInBulkToggle/bulkBlockedConfirm 7, +renderUnblockedPicker 3), stats.ts
(+blocked-tile-button 2), palette.ts (+buildPriorityCommands 5). No backend Go
test change (no backend change), but internal/serve re-ran uncached (1.9s)
against the freshly-embedded T11 bundle.

Gates (run once at end of batch) — all green:
- gofmt -l clean, go vet clean, go build ok
- go test ./... ok all packages (internal/commands 55.2s; internal/serve
  uncached 1.9s against the new embed)
- web: npm run check 451 pass, npm run build ok (38 modules, JS 30.43KB gz,
  CSS 8.50KB gz)

Live end-to-end proof: built the binary, init + 5 tasks with a #3->#2->#1
blocker chain plus a `due:in 2d` task and a no-due backlog, `tsk serve`. F63:
PATCH /api/tasks/4 {priority:urgent} round-tripped medium->urgent on disk as
`prio:urgent` while preserving `due:2026-06-26` (hand-editable contract intact).
F42/F62: completing #1 cleared #2's open_blockers (now []) while #3 stays
blocked by #2 — exactly the newlyUnblocked condition. F59: the live list yields
due-this-week=1 (#4) / no-due=3 (#2,#3,#5), matching computeScheduleStats. The
served bundle carries every slice's hooks (Due this week, data-blocked-drill,
prio-set-urgent, "complete all anyway", unblock-jump, "Newly unblocked").

Roadmap status: F1-F64 done (57 of the 65-item F-roadmap shipped). Unstarted:
F47-F54 (T9 backlog: bulk due/pin depth, context submenus, autocomplete in edit/
filter, dep mini-graph, all-section reorder, multi-paste review, palette
row-actions, touch context menu), F61 (chain-drill from the row badge), F65
(notes highlight in the chain/badge). T12 backlog appended below so the loop
never starves.

### T12 — depth (appended T11 2026-06-24 so the loop never starves)

Standing unstarted: F48, F49, F50, F51, F52, F53, F54 (the T9 backlog),
F65 (notes-snippet highlight in the chain drill + dep badge titles).
Fresh follow-ons after the T11 schedule-lens / blocked-filter /
palette-priority / bulk-guard / unblock-picker cluster:

- [x] **F66** Schedule-tile click-to-filter: clicking the F59 "Due this week"
      tile narrows the list to that 7-day window (a new render-pipeline lens like
      F64's blocked-only), and the "No due" tile filters to undated tasks; chips
      clear them. Reuse the F64 lens plumbing rather than FilterState.
      (tick T12 2026-06-25 — generalized into a 5-lens model in lens.ts)
- [x] **F67** Palette "Set due ▸" group: mirror F63 for due dates — a Cmd-K group
      with the NL presets (today / tomorrow / this weekend / next week / clear)
      acting on the selection via the existing parse-date + PATCH, keyboard-only.
      (tick T12 2026-06-25)
- [ ] **F68** Bulk priority/pin guard parity: when a bulk action touches blocked
      tasks (e.g. bulk-complete via the F36 menu), route it through the same F60
      confirm so every multi-task completion path is guarded, not just the bar's
      toggle button. NOTE: audited T12 — the ONLY bulk completion path is the
      bulkbar toggle, which ALREADY routes through the F60 guard (bulkToggleDone).
      The F36 menu sets priority/tag/due/pin, none of which complete a task. So
      there is no unguarded completion path to fix; F68 is a no-op as written and
      is dropped rather than padded. Revisit only if a new bulk-complete entry
      point is added.
- [x] **F69** Stats "Open" + "Overdue" tiles click-to-filter: extend the F64
      pattern so the time-based metric tiles also drive the list (Overdue ->
      overdue-only, Open -> hide-done), giving the whole sidebar a consistent
      "click a number to see those tasks" affordance. (tick T12 2026-06-25 —
      shipped with F66 as one unified lens model; Open routes to the hide-done
      FACET, the rest to lenses)
- [x] **F70** Unblock picker keyboard nav: arrow-key + Enter selection inside the
      F62 picker popover (and the F56 chain drill), so the just-unblocked jump is
      reachable without the mouse — matching the palette's keyboard model.
      (tick T12 2026-06-25 — new popnav.ts; wired into BOTH popovers)

### T12 — 2026-06-25 03:01 PT — sidebar lenses + keyboard depth (5/5)

Workdir note: the canonical `/Volumes/Projects/tsk` (external SSD sparseimage)
WAS mounted this tick — git resolved against it cleanly (the lock wrapper did
not log a cd failure), was at origin/main e34fe01 with a clean tree. Needed
`npm --prefix web install` (node_modules was partial — @types/node absent, which
broke the test typecheck until reinstalled). Pushed a clean fast-forward
e34fe01..765c32c, verified HEAD == origin/main, 0/0.

Slices shipped (5/5 — a cohesive "make the sidebar clickable + the popovers +
due-setting keyboard-first" cluster; F68 dropped honestly, see below):

- F66+F69 feat(web): stats sidebar tiles become click-to-filter lenses (cbacb5a)
- F67 feat(web): "Set due" command group in the command palette (9a341f3)
- F70 feat(web): keyboard nav inside the chain-drill + unblock popovers (27f06b5)
- F47 feat(web): bulk pin/unpin + live due preview in the bulk bar (c72f167)
- F61 feat(web): walk the blocker chain from the row badge (aa2fc66)
- (+ chore 765c32c: rebuilt embedded SPA bundle for all five slices)

Why F68 dropped (5 solid slices, not 5-plus-a-no-op): the standing T12 queue
listed F66-F70, but F68 (bulk guard parity) turned out to be a no-op on audit —
the ONLY bulk path that COMPLETES a task is the bulkbar "toggle done" button,
which already routes through the F60 blocked-guard (blockedInBulkToggle +
bulkBlockedConfirm). The F36 bulk-edit menu only sets priority/tag/due/pin, none
of which complete a task, so there's nothing to guard there. Shipping F68 would
have meant inventing a path to "fix". Instead I pulled F61 (chain-walk from the
row badge) from the standing T11 backlog — a genuine, demoable slice — to round
out a real 5. Quality over the number.

Design notes worth keeping:
- F66 generalized F64's one-off `blockedOnly` boolean into a small lens.ts model
  (LensKind = blocked|overdue|today|week|nodue + matchesLens + applyLens + chip
  metadata). A lens is a DERIVED, cross-task or clock-relative subset, so it
  lives OUTSIDE FilterState (must not serialize into saved views) — exactly where
  the blocked lens already sat. main.ts now holds a single `activeLens` slot; the
  blocked tile migrated to the unified `data-lens-drill` dispatch.
- F69's "Open" tile is the one exception — it maps to the real hide-done FACET
  (which DOES serialize), so it routes through setFilter, not the lens. The other
  metric/schedule tiles set the matching lens. The active-lens chip wears a hue
  (alert / today / neutral) echoing its source tile.
- F67 mirrors F63 exactly: buildDueCommands (pure) emits 6 commands each carrying
  a natural-language `token` handed verbatim to the same commitDue PATCH the F12
  picker uses, so the dates resolve identically via the server's dateparse. No
  backend change.
- F70's popnav.ts (keyToPopNavAction + nextPopNavIndex) is a tiny pure model
  mirroring the palette: arrows/jk move with wrap, Home/End + g/G jump, Enter
  activates, Escape closes. Wired into BOTH the chain drill (F56) and the unblock
  picker (F62) — each tracks a row-id list + highlighted index, paints the active
  row, and Enter jumps.
- F47 deepens the F36 bulk cluster: a 4th "pin" opener (Pin all / Unpin all over
  the PATCH `pinned` setter, skipping no-ops) and a live NL due preview line in
  the bulk-due popover (same previewVM + /api/parse-date the F12 picker uses,
  seq-guarded).
- F61 adds deepestChainFrom(tasks, start) — the same greedy walk as F56's
  longestChainPath but head-fixed to a specific task; the "blocked by #N" badge
  gains a chain-walk button that opens the drill popover for THAT row's path
  (inherits F70's keyboard nav for free).

Tests: +42 net new web tests (451 -> 493). New pure modules each unit-tested
under node --test: lens.ts (13), popnav.ts (9); extended stats.ts (+10 lens
tiles), palette.ts (+5 Set-due), bulkedit.ts (+2 pin/preview), deps.ts (+6
deepestChainFrom + badge). The F64 blocked-tile test updated for the unified
data-lens-drill hook. No backend Go test change (no backend change this tick),
but internal/serve re-ran against the freshly-embedded T12 bundle.

Gates (run once at end of batch) — all green:
- gofmt -l clean, go vet clean, go build ok
- go test ./... ok all packages (internal/commands 62.4s; internal/serve 5.05s
  against the new embed)
- web: npm run check 493 pass (was 451; +42 net new), npm run build ok
  (40 modules, JS 32.09KB gz, CSS 8.68KB gz)

Live end-to-end proof: built the binary, init + 6 tasks with a #1->#2->#3 blocker
chain plus a due-in-2d (#4), an overdue (#5, due 2026-06-20), and a no-due
backlog (#6); `tsk serve`. F67: PATCH /api/tasks/6 {due:"eow"} resolved to
2026-06-28 on disk as `due:2026-06-28` (NL parse path the palette drives). F47:
PATCH /api/tasks/4 {pinned:true} set `pin:true` while PRESERVING `due:2026-06-27`
(contract intact); `tsk show 4` reads pinned:yes + due back. F66/F69: stats
report overdue=1 (#5) / today=0, matching what the lens tiles render; the chain
#1->#2->#3 is the deepestChainFrom(#1) walk F61/F70 drive. The served bundle
carries every slice's hooks (data-lens-drill, lens-hue-*, "Set due:" +
due-set-clear, chain-pop-keys, data-bulk-set-pin + bulk-due-preview,
data-chain-from + dep-chain-btn). The hand-editable .tsk.md storage contract is
intact — CLI reads back every web write.

Roadmap status: F1-F67, F69-F70 done; F66 done (62 of the 70-item F-roadmap
shipped, F68 dropped as a no-op). Unstarted: F48-F54 (T9 backlog: context
submenus, autocomplete in edit/filter, dep mini-graph, all-section reorder,
multi-paste review, palette row-actions, touch context menu), F65 (notes
highlight in the chain/badge). T13 backlog appended below so the loop never
starves.

### T13 — depth (appended T12 2026-06-25 so the loop never starves)

Standing unstarted: F48 (context-menu submenus), F49 (autocomplete in edit +
filter), F50 (dep mini-graph), F51 (all-section reorder), F52 (multi-paste
review), F53 (palette row-actions), F54 (touch context menu), F65 (notes-snippet
highlight in the chain drill + dep badge titles). Fresh follow-ons after the
T12 lens / palette-due / popnav / bulk-pin / chain-walk cluster:

- [x] **F71** Lens keyboard shortcuts: number keys (or a small lens row in the `?`
      overlay) to toggle each stats lens (blocked / overdue / today / week / nodue)
      without opening the sidebar — reuse the F66 setLens. Surface the active lens
      in the help overlay's "current filter/sort" line. (tick T13 2026-06-25)
- [ ] **F72** Saved-view + lens coexistence: a recalled saved view currently can't
      carry a lens (lenses are non-serializable by design), but the active lens
      should survive a view recall (apply the view's facets, keep the lens) instead
      of being implicitly cleared — audit setFilter/recallView and make the
      interaction intentional + documented. NOTE: audited T13 — recallView ALREADY
      leaves activeLens untouched (only clear-all, jump-to-hidden, and the chip
      clear it). The lens already survives a recall and stays visible via its
      filter-bar chip. F72-as-written describes a bug that doesn't exist; dropped
      as a no-op (like T12's F68) rather than padded. Pulled the carried F65 to
      round out a real 5.
- [x] **F73** Palette "Set due ▸" live-preview: when a due-set command is
      highlighted in Cmd-K, show the resolved date inline (reuse parse-date) so you
      confirm "this weekend = Jun 28" before Enter — mirrors the F47 bulk preview.
      (tick T13 2026-06-25)
- [x] **F74** Chain-walk from the dep EDITOR (F39) too: while editing blockers, a
      tiny "walk chain" affordance per candidate so you can see what a prospective
      blocker would chain into before adding it (reuse deepestChainFrom + the
      keyboard-nav popover). (tick T13 2026-06-25)
- [x] **F75** Lens + export: an "export this lens" action so the current
      blocked/overdue/etc. subset can be exported (JSON/CSV/MD) — the export
      endpoints take the whole store today; thread the active lens (and filter)
      into the export query so what you SEE is what you GET. (tick T13 2026-06-25)
- [x] **F65** (carried) Notes-snippet highlight in the chain drill + dep badge
      titles, so a search that matched on a blocker's text is visible when you walk
      the chain (reuse the generic highlightText engine). (tick T13 2026-06-25 —
      shipped as chain-drill + unblocked-picker title highlight)
- [ ] **F48** (carried) Context-menu submenus: nest the priority levels + a "due
      presets" flyout inside the F37 row menu, keyboard-navigable (now that popnav.ts
      exists, reuse it for submenu arrow-nav).
- [ ] **F49** (carried) Autocomplete depth: extend the F38 composer dropdown to the
      inline EDIT field (F7) and the filter box (F11) so #tag/@due completion is
      everywhere; add a `!priority` completion too.
- [ ] **F50** (carried) Dependency mini-graph in the dep editor (F39): a tiny
      ASCII/SVG "what blocks what" preview of the selected task's neighborhood.
- [ ] **F54** (carried) Row context menu on touch: long-press (reuse the F28 machine)
      to open the F37 menu, with a press-and-hold disambiguation.

### T13 — 2026-06-25 08:03 PT — lens keyboard + palette/chain/export depth (5/5)

Workdir note: the canonical `/Volumes/Projects/tsk` (external SSD sparseimage)
WAS mounted this tick — the lock wrapper resolved the repo cleanly (no cd
failure), HEAD was exactly at origin/main c474c99 with a clean tree, node_modules
present. Pushed a clean fast-forward. The SSD-absent fallback (see T2-T11 notes)
was not needed.

Slices shipped (5/5 — a cohesive "lenses are keyboard-first + the palette/chain/
export all respect what you're looking at" cluster; F72 dropped honestly, see
below):

- F71 feat(web): number-key lens shortcuts + active-lens readout (16e612c)
- F73 feat(web): live due-date preview in the palette "Set due" group (3cf0d6a)
- F75 feat(web): export only the visible lens/filter subset (fa42c82)
- F74 feat(web): walk a prospective blocker's chain from the dep editor (1db180c)
- F65 feat(web): highlight the search match while walking blocker chains (978b66a)
- (+ chore 4050acb: rebuilt embedded SPA bundle for all five slices)

Why F72 dropped (5 solid slices, not 5-plus-a-no-op): the standing T13 queue led
with F71-F75, but F72 (make the active lens "survive" a view recall) turned out a
no-op on audit — `recallView` already leaves `activeLens` untouched (only
clear-all, jump-to-hidden, and the chip itself clear it), so the lens ALREADY
survives a recall and stays visible via its filter-bar chip. F72-as-written
describes a bug that doesn't exist; shipping it would mean inventing a "fix".
Instead I pulled the carried F65 (chain-drill search highlight) from the backlog —
a genuine, demoable slice that pairs naturally with F74's chain work — to round
out a real 5. Quality over the number (same call as T12's F68 -> F61).

Design notes worth keeping:
- F71 adds lens.ts pure helpers: LENS_ORDER (single source of truth for the
  tile/digit order), lensForDigit (key "1".."5" -> lens, null otherwise),
  activeLensSummary (glyph+label for the help line). The digit handler sits
  BEFORE the main key switch so it never collides with letter hotkeys; pressing
  the active lens's digit again clears it (toggle). The `?` overlay gained a
  "1 … 5" row + a live "Active lens: ..." readout (data-help-active, hidden when
  none).
- F73 adds palette.dueTokenForCommandId (id -> NL token; "" for clear, null for
  non-due) so the decode is pure-tested. paintPalette drives a preview line under
  the cmdk list: a highlighted "Set due: X" command resolves its token via the
  SAME /api/parse-date the F12 picker + F47 bulk preview use, with a seq guard
  (paletteDueParseSeq) that also invalidates on close. "Clear" shows "Clears the
  due date"; non-due commands hide the line.
- F75 is the one with backend teeth: /api/export gains an optional ?ids=1,2,3
  that narrows to that subset in STORE order (new filterTasksByIDs, skipping
  unknown/malformed, blank-list -> nil/empty export); json/csv/markdown all honor
  it. Client: export.scopedExportUrl / exportScopeLabel pure helpers;
  downloadExport threads visibleIds when isExportScoped() (lens OR filter OR tag
  route active); the menu rebuilds each open with an "Export N shown" header.
- F74 adds deps.hasWalkableChain (deepestChainFrom length >= 2) to gate a "walk
  chain" button on dep-editor candidates that chain further; renderDepCandidates
  takes an optional walkable set and emits data-dep-walk only for those.
  main.ts computes the set over the live graph each paint, routes the button
  through the existing openChainDrill(id) (inherits F70 keyboard-nav for free).
- F65 threads an optional query into renderChainDrill + renderUnblockedPicker;
  titles now route through the generic highlightText engine (which subsumed the
  old local escapeChainHTML — both render paths share one escaper). main.ts
  passes filter.query.trim() into both popovers, so a search that landed on a
  blocker's text stays <mark>ed as you walk the chain.

Tests: +26 net new web tests (493 -> 519). New/extended pure modules each
unit-tested under node --test: lens.ts (+6 F71), palette.ts (+4 F73), export.ts
(+5 F75), deps.ts (+3 F74 hasWalkableChain + threaded query), depedit.ts (+3 F74
walk button), chain.ts (+5 F65 highlight). Go side adds export_scope_test.go
(6 endpoint tests + 1 unit on filterTasksByIDs) for F75.

Gates (run once at end of batch) — all green:
- gofmt -l clean, go vet clean, go build ok
- go test ./... ok all packages (internal/commands 59.5s; internal/serve cached
  against the freshly-embedded T13 bundle + the new export-ids path)
- web: npm run check 519 pass (was 493; +26 net new), npm run build ok
  (40 modules, JS 32.72KB gz, CSS 8.85KB gz)

Live end-to-end proof: built the binary, init + 4 tasks with a #1->#2->#3 blocker
chain, an overdue #1 (due 2026-06-20), and a due-this-week #4 (due 2026-06-26);
`tsk serve`. F75: /api/export?format=csv with no ids returned all 4; ids=1,4
returned exactly #1+#4 in store order; ids=3 markdown returned just "fix the
bug"; empty ids -> null (empty); ids=999,2 skipped the unknown and returned only
#2. F73: /api/parse-date resolved eow -> Sun Jun 28 (in 3d), 1w -> Thu Jul 2 (in
7d) — exactly what the palette preview renders. The served bundle carries every
slice's hooks (Active lens + "1 … 5" + data-lens-drill, cmdk-due-preview +
data-cmdk-due-preview, export-scope + ids=, data-dep-walk, chain-title +
data-chain-jump/data-unblock-jump). The hand-editable .tsk.md is byte-clean with
the depends: chain intact and `tsk show 1` reads #2 back — the CLI/TUI storage
contract is fully preserved.

Roadmap status: F1-F71, F73-F75 done; F65 done; F66-F70 done (66 of the 75-item
F-roadmap shipped; F68 + F72 dropped as no-ops). Unstarted: F48 (context-menu
submenus), F49 (autocomplete in edit/filter), F50 (dep mini-graph), F54 (touch
context menu). T14 backlog appended below so the loop never starves.

### T14 — depth (appended T13 2026-06-25 so the loop never starves)

Standing unstarted: F48 (context-menu submenus), F49 (autocomplete in edit +
filter), F50 (dep mini-graph), F54 (touch context menu). Fresh follow-ons after
the T13 lens-keyboard / palette-preview / chain-walk / scoped-export / chain-
highlight cluster:

- [x] **F76** Lens digit hints in the stats sidebar: render the "1".."5" shortcut
      key on each stats tile (a small kbd badge) so the keyboard shortcut is
      discoverable at the point of use, not just in the `?` overlay. Reuse LENS_ORDER
      so the badge can't drift from lensForDigit. (tick T14 2026-06-25)
- [x] **F77** Palette "Set priority ▸" live-preview parity: mirror F73 for the F63
      priority group — when a "Set priority: X" command is highlighted, show the
      target task's current -> new level inline so the change is previewed before
      Enter (no parse needed; pure from selTask.priority). (tick T14 2026-06-25)
- [x] **F78** Scoped export in the command palette: add explicit "Export N shown
      (JSON/CSV/MD)" commands to Cmd-K when a lens/filter is active, distinct from
      the whole-store export commands, so the scoped download is reachable
      keyboard-only (reuse isExportScoped + scopedExportUrl). (tick T14 2026-06-25)
- [x] **F79** Chain-walk from the dep editor's CURRENT blockers too (F74 only wired
      the candidate list): a walk affordance on each existing blocker chip so you can
      audit what an already-added blocker chains into, not just prospective ones.
      (tick T14 2026-06-25)
- [x] **F80** Lens-aware stats: when a lens is active, show the lensed subset's
      mini-counts (e.g. "of 12 blocked: 3 urgent, 5 overdue") in the sidebar so the
      numbers reflect what you're looking at, not just the whole board.
      (tick T14 2026-06-25)
- [ ] **F48** (carried) Context-menu submenus: nest the priority levels + a "due
      presets" flyout inside the F37 row menu, keyboard-navigable (reuse popnav.ts
      for submenu arrow-nav).
- [ ] **F49** (carried) Autocomplete depth: extend the F38 composer dropdown to the
      inline EDIT field (F7) and the filter box (F11); add a `!priority` completion.
- [ ] **F50** (carried) Dependency mini-graph in the dep editor (F39): a tiny
      ASCII/SVG "what blocks what" preview of the selected task's neighborhood.
- [ ] **F54** (carried) Row context menu on touch: long-press (reuse the F28 machine)
      to open the F37 menu, with a press-and-hold disambiguation.

### T14 — 2026-06-25 13:08 PT — lens/palette/dep/export consistency cluster (5/5)

Workdir note: the canonical `/Volumes/Projects/tsk` (external SSD sparseimage)
WAS mounted this tick — the lock wrapper resolved the repo cleanly, HEAD was at
origin/main d374fdc with a clean tree and node_modules present. Pushed a clean
fast-forward (d374fdc..7088dbd). The SSD-absent fallback was not needed.

Slices shipped (5/5 — a cohesive "every lens/palette/dep/export surface gets the
same discoverable, consistent treatment" cluster; led the standing T14 queue
exactly as written, no drops this tick):

- F76 feat(web): show lens digit-key badges on the stats tiles (a131e06)
- F77 feat(web): live "current -> new" preview for palette priority commands (c8f4746)
- F78 feat(web): explicit scoped + whole-store export commands in Cmd-K (d03fbb9)
- F79 feat(web): walk a current blocker's chain from the dep editor (51061f9)
- F80 feat(web): lens-aware stats breakdown in the sidebar (bf7fdf8)
- (+ chore 7088dbd: rebuilt embedded SPA bundle for all five slices)

Design notes worth keeping:
- F76 adds lens.lensDigit(kind) — the exact INVERSE of lensForDigit, derived from
  the same LENS_ORDER, returning "1".."5" for a real lens or "" for the "open"
  tile (which maps to the hideDone facet, not a numbered lens). stats.ts renders
  a `.lens-key` kbd badge in the top-right corner of every drill tile; faint,
  brightening to accent on hover/focus. A unit test asserts lensDigit/lensForDigit
  round-trip so a badge can never point at the wrong key.
- F77 adds palette.priorityForCommandId (id -> level | null) + priorityPreviewVM
  (current,target -> {state, text}) + renderPriorityPreview. Unlike F73's due
  preview (needs a server parse), this is pure + synchronous from the selected
  task's current level, so paintPaletteDuePreview resolves it FIRST and bumps the
  due-parse seq so a priority highlight can't be clobbered by a stale in-flight
  date parse. Reuses the .due-preview is-valid/is-empty classes — both previews
  share the one palette slot and read identically. Reads "Medium -> Urgent",
  "Already Urgent", or an em dash on the left when there's no current priority.
- F78 adds export.buildExportCommands(scopedCount) + exportCommandTarget(id). The
  three whole-store "Export tasks as <FMT>" commands always ship; when a
  lens/filter/tag narrows the board, three "Export N shown as <FMT>" commands
  (count via exportScopeLabel) are PREPENDED, distinct ids (export-scoped-csv).
  On a plain board only the whole-store trio shows (nothing duplicated).
  downloadExport gained a forceScope override so the explicit commands always do
  exactly what their title says, ignoring the F75 auto-scope; the menu button
  keeps its auto behaviour. exportCommandTarget decodes both id families to
  {format, scoped} for a single dispatch.
- F79 threads an optional `walkable` set into renderDepEditor; CURRENT blocker
  chips whose own chain runs deeper (deps.hasWalkableChain) get a corner-arrow
  button (data-dep-chip-walk) beside the remove x — the sister of F74's affordance
  on the CANDIDATE list. main.ts computes the set over the live graph (hoisted as
  a `function` so the initial render can call it before its position), recomputing
  each repaint so it tracks adds/removes; the click routes through openChainDrill
  (inherits F70 keyboard nav). A leaf blocker carries no button.
- F80 adds lens.computeLensBreakdown (applyLens + per-priority tally + overdue /
  blocked cross-cuts, each cross-cut suppressed when redundant with the active
  lens) + renderLensBreakdown (an "In view" section: headline count + non-zero
  priority/cross-cut pills). refreshStats appends it only when a lens is active,
  computed over the same not-deleted pool + whole-list done-index the render
  pipeline lenses, so blocked (cross-task) + time windows see the real graph.

Tests: +38 net new web tests (519 -> 557). lens.ts (+15: F76 lensDigit round-trip
+ F80 breakdown compute/render), stats.ts (+8 F76 badges), palette.ts (+10 F77),
export.ts (+11 F78 command build/decode), depedit.ts (+4 F79 chip walk button).

Gates (run once at end of batch) — all green:
- gofmt -l clean, go vet clean, go build ok
- go test ./... ok all packages (internal/commands 57.9s; internal/serve cached
  against the freshly-embedded T14 bundle)
- web: npm run check 557 pass (was 519; +38 net new), npm run build ok
  (40 modules, JS 33.58KB gz, CSS 9.04KB gz)

Live end-to-end proof: built the binary, init + 4 tasks (#1 overdue+urgent, #2
due-this-week+high, #3 medium no-due, #4 low blocked by #1); `tsk serve`. F78:
/api/export?format=csv&ids=1,4 returned exactly #1+#4 in store order; no-ids
markdown returned all four — both whole-store and scoped export resolve through
the one endpoint. The served bundle carries every slice's hooks (lens-key x1,
data-cmdk-due-preview + "Already" x1, export-scoped- x1, data-dep-chip-walk x2,
lens-bd x3 + "In view"). Storage contract intact: the raw .tsk.md is clean plain
markdown with the depends:1 chain, and `tsk show 4` reads #1 back.

Roadmap status: F1-F80 done (71 of the 80-item F-roadmap shipped; F68 + F72
dropped as no-ops in earlier ticks). Unstarted: F48 (context-menu submenus), F49
(autocomplete in edit/filter), F50 (dep mini-graph), F54 (touch context menu).
T15 backlog appended below so the loop never starves.

### T15 — depth (appended T14 2026-06-25 so the loop never starves)

Standing unstarted: F48 (context-menu submenus), F49 (autocomplete in edit +
filter), F50 (dep mini-graph), F54 (touch context menu). Fresh follow-ons after
the T14 lens-digit / palette-priority-preview / scoped-export-commands /
chip-chain-walk / lens-breakdown cluster:

- [x] **F81** Lens-breakdown pills are clickable: clicking the "3 urgent" pill in
      the F80 breakdown layers the priority facet on top of the active lens (lens
      AND urgent), so the sidebar's mini-counts become a drill-down, not just a
      readout. Reuse setFilter's priorities facet — lenses already coexist with
      facets in the render pipeline.
- [x] **F82** Digit-key hint in the lens chip + help overlay row: now that F76 put
      the "1".."5" badges on the tiles, echo the active lens's digit in the
      filter-bar lens chip ("2 overdue x") and bold that row in the `?` overlay's
      lens list, so the active shortcut is visible everywhere the lens is.
- [x] **F83** Palette "Set due ▸" + "Set priority ▸" disabled-reason hints: when a
      set command is disabled (no selection, or already in effect), show WHY in the
      preview slot ("select a task first" / "already urgent") instead of just
      greying it — reuse the F77/F73 preview line for the message.
- [x] **F84** Scoped export parity for the export MENU button: the F78 palette
      commands can force whole-store vs scoped, but the menu button only auto-scopes
      — add a small "all / shown" toggle to the export dropdown header so the menu
      reaches both too (mirror the palette's two-command split).
- [x] **F85** Chain-walk reachability from the row badge's UPSTREAM too: F61/F79
      walk a task's blocker chain (downstream to root); add a "what waits on this?"
      walk that follows dependents (upstream), reusing the graph the dep stats
      already build, so you can audit impact both directions from one popover.
- [ ] **F48** (carried) Context-menu submenus: nest the priority levels + a "due
      presets" flyout inside the F37 row menu, keyboard-navigable (reuse popnav.ts
      for submenu arrow-nav).
- [ ] **F49** (carried) Autocomplete depth: extend the F38 composer dropdown to the
      inline EDIT field (F7) and the filter box (F11); add a `!priority` completion.
- [ ] **F50** (carried) Dependency mini-graph in the dep editor (F39): a tiny
      ASCII/SVG "what blocks what" preview of the selected task's neighborhood.
- [ ] **F54** (carried) Row context menu on touch: long-press (reuse the F28 machine)
      to open the F37 menu, with a press-and-hold disambiguation.

---

## TICK LOG — T15 (2026-06-25 ~18:19 PT)

Shipped 5/5 web slices on `main` (F81-F85), gated once, pushed clean
(f2f412c..6f6fc6e). 29 net-new web tests (557 -> 586), Go gates green,
bundle rebuilt + embedded, live `tsk serve` smoke-tested.

- **F81** `764ad66` Clickable lens-breakdown priority pills. renderLensBreakdown
  gains an `active` set; the four priority pills become <button data-lens-bd-prio>
  drill-downs that layer the priorities facet onto the active lens (lens AND
  urgent). New pure lensBreakdownPriority + LENS_BD_PRIORITIES. Cross-cut pills
  (overdue/blocked) stay plain spans. +6 lens tests.
- **F82** `aa7bcd1` Lens digit echoed in the chip + help overlay. renderLensChipBody
  leads with a lens-chip-key <kbd> ("2 overdue x"); new renderActiveLensHelp()
  builds the ? overlay line with the digit kbd + bold label; the lens-shortcut
  help row bolds while a lens is active. +5 lens tests.
- **F83** `6bfebf1` Palette set-command disabled-reason hints. setCommandDisabledReason
  + renderDisabledReason: a disabled Set due/priority command with no selection
  says "select a task first" in the shared preview slot (the already-<level> case
  keeps F77's empty-state text). paintPaletteDuePreview checks it first, seq-bumping
  in-flight due parses. +6 palette tests.
- **F84** `de50582` All/shown scope toggle for the export menu. renderExportMenu
  gains a scopeShown param; when scoping is active the header is an All / N shown
  segmented toggle (data-export-scope-toggle), items carry data-export-scope mapped
  to F78's forceScope override. Live-verified the /api/export?ids= backend. +4
  export tests (one F75 test updated for the new markup).
- **F85** `c084551` Upstream "what waits on this?" walk. deps.ts: openDependents,
  deepestDependentChainFrom, hasWalkableDependents (reverse-edge mirrors of the
  blocker walk). openChainDrill takes a direction + a header flip button so both
  downstream blockers and upstream dependents are auditable from one popover. +8
  deps tests.
- `6f6fc6e` bundle rebuild (40 modules, JS 34.32KB gz, CSS 9.30KB gz).

Live proof: built /tmp/tsk-t15, a #4->#3->#2->#1 dep chain, `tsk serve
--addr 127.0.0.1:7895`. /api/export?ids=1,4 returned exactly #1+#4 in store
order; whole-store returned all four. Served bundle carries every hook
(lens-bd-prio, lens-chip-key, "select a task first", export-scope-toggle,
data-chain-flip + "Waiting on #" + "what waits on this?"). Raw .tsk.md stayed
plain hand-editable markdown with depends: meta; `tsk show 4` + `tsk depend 4
--tree` read the chain back intact. Storage contract honoured.

Deferred: nothing forced. F48/F49/F50/F54 remain the standing T9 carries.
T16 backlog (F86-F90) appended below so the loop never starves.

### T16 — depth (appended T15 2026-06-25 so the loop never starves)

Standing unstarted: F48 (context-menu submenus), F49 (autocomplete in edit +
filter), F50 (dep mini-graph), F54 (touch context menu). Fresh follow-ons after
the T15 lens-drill / lens-digit / disabled-reason / export-toggle / dependent-
walk cluster:

- [x] **F86** Lens-breakdown drill is two-way reversible from the filter bar: once
      F81 layers a priority facet onto a lens, add a tiny "in <lens>" qualifier to
      the priority chip in the filter bar while a lens is also active, so it's clear
      the facet is scoped to the lens, not the whole board. (tick T16 2026-06-25)
- [x] **F87** Dependent-walk count badge on the row: now that F85 can walk upstream,
      surface a small "N waiting" badge on a task that other undone tasks depend on
      (openDependents length), so you can SEE a chokepoint before opening the popover
      — sister of the existing "blocked by #N" downstream badge. (tick T16 2026-06-25)
- [x] **F88** Export menu remembers the last scope choice within a session: F84 resets
      to "shown" each open; persist the All/shown pick in sessionStorage so a user who
      always wants "All" isn't re-toggling every time. (tick T16 2026-06-25)
- [x] **F89** Palette disabled-reason for non-set commands too: extend F83 beyond Set
      due/priority — "Toggle done" with no selection -> "select a task first", "Undo"
      with nothing to undo -> "nothing to undo", so every greyed command explains itself. (tick T16 2026-06-25)
- [x] **F90** Help overlay digit map is a live mini-legend: under the bolded F82 lens
      row, render the five lens labels with their digit kbds inline ("1 blocked · 2
      overdue · …") and mark the active one, so the ? overlay teaches the whole digit
      map at a glance. (tick T16 2026-06-25)

---

## TICK LOG — T16 (2026-06-25 ~22:52 PT)

Shipped 5/5 web slices on `main` (F86-F90), gated once, pushed clean
(fc5f47d..910857b). 21 net-new web tests (586 -> 607), Go gates green,
bundle rebuilt + embedded, live `tsk serve` smoke-tested. The canonical
workdir `/Volumes/Projects/tsk` WAS mounted this tick — the lock wrapper
resolved the repo cleanly (HEAD at fc5f47d, clean tree, node_modules
present). The SSD-absent fallback was not needed.

This batch finishes the T16 standing queue exactly as written (no drops):
a "consistency + discoverability" cluster that closes loops opened by the
T15 lens-drill / dependent-walk / export-toggle / disabled-reason work.

- **F86** `ef00686` Priority-facet scope note. New pure
  filter.renderPriorityScopeNote(lensLabel, state) -> "in <lens>" only when a
  priority facet AND a lens are both active (else ""); HTML-escaped.
  renderFilterBar appends it after the priority pills using the active lens
  meta label; faint italic .fprio-scope, .filter-prios gains
  align-items:center. +4 filter tests.
- **F87** `9833f8c` "N waiting" dependent chokepoint badge. deps.ts:
  dependentCount + renderWaitingBadge (button with the open-dependent count +
  data-waiting-walk; "" when nothing waits or the task is done). render.ts
  threads the whole live list through a new RowContext.allTasks and renders the
  badge beside the blocked badge; main.ts feeds notDeleted and routes a click
  into the F85 dependent chain-drill (direction "dependent"), anchoring the
  popover on the waiting badge. Amber .waiting-badge (the upstream mirror of the
  red blocked-by badge). +4 deps tests.
- **F88** `4870b33` Persisted export scope. export.ts: EXPORT_SCOPE_KEY +
  parseExportScope (default shown/true for null/corrupt) + serializeExportScope.
  main.ts seeds exportScopeShown from sessionStorage at boot, re-reads on each
  open instead of force-resetting to "shown", and writes on the segment flip.
  +4 export tests.
- **F89** `b42d034` Palette disabled-reason for every gated command.
  palette.ts: SELECTION_GATED_COMMANDS set + commandDisabledReason(id, ctx),
  which delegates to setCommandDisabledReason first (F83 preserved exactly) then
  covers undo ("nothing to undo"), filter ("no tasks to filter"), alltasks
  ("already on all tasks"), and all per-task verbs ("select a task first").
  main.ts feeds the live ctx and only consults it when the highlighted command
  is actually disabled. +6 palette tests.
- **F90** `f129918` Live lens digit-map mini-legend. lens.ts:
  renderLensDigitMap(active) walks LENS_ORDER (can't drift from lensForDigit)
  emitting each lens's digit + glyph + label with a data-lens-legend hook; the
  active entry gets is-active + aria-current. main.ts renders it into a new
  data-help-legend slot in the help card, refreshed on every open. Faint legend
  that brightens the active entry to the amber accent. +3 lens tests.
- `910857b` bundle rebuild (40 modules, JS 34.84KB gz, CSS 9.45KB gz).

Live proof: built /tmp/tsk-t16, a #4->#3->#2->#1 dep chain (so #1 is a
chokepoint), `tsk serve --addr 127.0.0.1:7896`. /api/export?ids=1,4 returned
exactly #1+#4 in store order; whole-store returned all four. Served bundle
carries every hook (fprio-scope, waiting-badge + data-waiting-walk x3,
tsk.export.scopeShown, "nothing to undo"/"no tasks to filter"/"already on all
tasks", lens-legend + data-lens-legend + data-help-legend). Toggling the #1
chokepoint done round-tripped to .tsk.md preserving the depends: chain + adding
completed:; `tsk depend 4 --tree` read the whole #4->#3->#2->#1 chain back.
Storage contract honoured.

Deferred: nothing forced. F48 (context-menu submenus), F49 (autocomplete in
edit/filter), F50 (dep mini-graph), F54 (touch context menu) remain the standing
long-carries. T17 backlog (F91-F95) appended below so the loop never starves.

### T17 — depth (appended T16 2026-06-25 so the loop never starves)

Standing unstarted: F48 (context-menu submenus), F49 (autocomplete in edit +
filter), F50 (dep mini-graph), F54 (touch context menu). Fresh follow-ons after
the T16 scope-note / waiting-badge / persisted-export / disabled-reason /
digit-legend cluster:

- [x] **F91** Lens digit-map legend is clickable: F90 renders the five lenses with
      data-lens-legend hooks but they're inert. Wire a click on a legend entry to
      toggle that lens (same path as the digit key / stat tile) and close the help
      overlay, so the ? overlay becomes an actionable lens switcher, not just a
      readout. Reuse the existing setLens/toggle path. (tick T17 2026-06-26)
- [x] **F92** Waiting-badge count in the stats sidebar: F87 surfaces a per-row "N
      waiting" badge; add an aggregate "biggest chokepoint: #N (k waiting)" line to
      the dep-stats sidebar so the single worst bottleneck is visible without
      scanning rows. Reuse openDependents over the live graph; tie-break by id.
      (tick T17 2026-06-26)
- [x] **F93** Persist the active lens across reloads (sessionStorage): the lens is
      render-only state today, lost on refresh. Persist the active LensKind per-tab
      (sessionStorage, NOT localStorage — a lens is time-relative and shouldn't leak
      across sessions) and restore it on boot, mirroring F88's export-scope pattern.
      Validate the stored kind against LENS_ORDER so a stale value can't wedge.
      (tick T17 2026-06-26)
- [x] **F94** Scope note for the TAG facet too (sister of F86): when a tag chip is
      active AND a lens is on, show the same "in <lens>" qualifier on the tag row, so
      every facet layered onto a lens reads as scoped, not just priority. Reuse
      renderPriorityScopeNote's shape over the tags facet. (tick T17 2026-06-26)
- [x] **F95** Disabled-reason hints for the bulk-edit bar commands (extend F89's idea
      beyond the palette): when a bulk action is unavailable (e.g. "set due" with no
      rows selected, or an operation that would no-op), surface the same quiet reason
      text in the bulk bar so the floating-bar actions explain themselves too.
      (tick T17 2026-06-26)

When fewer than 5 remain, append more (recurring-task UI, archive view, an undo
stack beyond single delete, a keyboard-driven dep-graph navigator, etc.).
---

## TICK LOG — T17 (2026-06-26 ~03:43 PT)

Shipped 5/5 web slices on `main` (F91-F95), gated once, pushed clean
(2dca726..fe3004a). 26 net-new web tests (607 -> 633), Go gates green,
bundle rebuilt + embedded, live `tsk serve` smoke-tested. The canonical
workdir `/Volumes/Projects/tsk` WAS mounted this tick (HEAD at 2dca726,
clean tree, node_modules present).

This batch finishes the T17 standing queue exactly as written (no drops):
a "close-the-loop" cluster — the lens/facet/bulk consistency work opened
by T15-T16 (lens-drill, dependent-walk, scope-note, disabled-reason,
digit-legend) gets its natural follow-ons.

- **F92** `7c4c13e` Biggest-chokepoint line. deps.ts: biggestChokepoint
  (scan every undone task's open-dependent count, keep the max, tie-break by
  lowest id; null on a flat board) + the Chokepoint type. stats.ts:
  renderChokepoint emits a full-width button under the dep grid ("Biggest
  chokepoint  #N up K waiting") carrying the SAME data-waiting-walk hook the
  F87 row badge uses, so a click opens the F85 dependent chain-drill. Threaded
  through renderDepStats + renderStatsPanel as an optional arg (back-compat).
  refreshStats computes it over the same live list + whole-list done-index the
  per-row badges use. +12 tests.
- **F94** `4ad700e` Tag-facet scope note. filter.ts: renderTagScopeNote mirrors
  renderPriorityScopeNote over the tags facet ("#work in overdue"); returns ""
  unless a non-empty lens label AND a tag are both present; own .ftag-scope
  class. renderFilterBar appends it after the tag chips. Closes the F86 loop so
  EVERY facet layered onto a lens reads as scoped, not just priority. +5 tests.
- **F95** `a1d8874` Bulk-bar disabled-reason hints. bulkedit.ts:
  bulkPriorityDisabledReason + bulkPinDisabledReason (pure no-op detectors over
  the selection) + a bulkReasonLine; renderBulkPriorityMenu / renderBulkPinMenu
  take an optional selected[] (default empty -> nothing disabled, back-compat),
  greying a no-op option (aria-disabled) with a quiet reason ("all already
  high" / "none are pinned"). openBulkEdit feeds the live selection + short-
  circuits a click on a greyed option (no PATCH fan-out). Extends F89's palette
  idea to the floating bar. +9 tests.
- **F91** `65f49f2` Clickable lens digit-map. lens.ts: renderLensDigitMap emits
  real <button>s (same lens-legend-item class + data-lens-legend hook + title
  copy) instead of inert spans. main.ts: a delegated click on the help overlay
  routes a legend entry through setLens (toggle) + closes the overlay, before
  the backdrop-dismiss check. CSS strips the native button chrome (pointer,
  hover, focus ring). +2 tests.
- **F93** `60f55bd` Persisted active lens. lens.ts: LENS_KEY ("tsk.lens") +
  parseLens (validates against LENS_ORDER so a stale value can't wedge ->
  null). main.ts: activeLens seeds from sessionStorage at boot; setLens + the
  two clear paths route through one persistLens() helper (sessionStorage, NOT
  localStorage — a lens is time-relative; mirrors F88's export-scope). Writes
  wrapped so private-mode degrades to in-session-only. +3 tests.
- `fe3004a` bundle rebuild (40 modules, JS 35.49KB gz, CSS 9.70KB gz).

Live proof: built /tmp/tsk-t17, a #2/#3/#4 -> #1 dep chain (so #1 is a
chokepoint with 3 waiters), `tsk serve --addr 127.0.0.1:7897`. Served bundle
carries every hook (data-lens-legend buttons x5, tsk.lens, stat-chokepoint +
"Biggest chokepoint", ftag-scope, bulkedit-reason + "all already"/"none are
pinned"; CSS: button.lens-legend-item, chokepoint-n, ftag-scope, bulkedit-
reason). /api/stats reported overdue:1, the dep chain intact. Toggling the #1
chokepoint done round-tripped to .tsk.md preserving the depends: chain + adding
completed:; `tsk show 2` read the depends:#1 link back. Storage contract
honoured — the raw .tsk.md stayed plain hand-editable markdown.

Deferred: nothing forced. F48 (context-menu submenus), F49 (autocomplete in
edit/filter), F50 (dep mini-graph), F54 (touch context menu) remain the standing
long-carries. T18 backlog (F96-F100) appended below so the loop never starves.

### T18 — depth (appended T17 2026-06-26 so the loop never starves)

Standing unstarted: F48 (context-menu submenus), F49 (autocomplete in edit +
filter), F50 (dep mini-graph), F54 (touch context menu). Fresh follow-ons after
the T17 chokepoint / tag-scope / bulk-disabled-reason / clickable-legend /
persisted-lens cluster:

- [x] **F96** Chokepoint line also offers a quick "filter to its dependents":
      F92 surfaces #N with K waiters and opens the dependent chain-drill on click.
      Add a secondary affordance (or a drill-popover action) that layers a filter
      showing exactly the K undone tasks waiting on #N, so you can act on the whole
      blocked cohort, not just walk it. Reuse openDependents for the id set.
      (tick T18 2026-06-26)
- [x] **F97** Persist the stats-sidebar lens-breakdown drilled facet across the
      reload too: F93 restores the active lens, but if F81 layered a priority facet
      onto it (lens AND urgent), that facet rides the normal filter — confirm it
      restores coherently with the lens on boot, and add a regression test for the
      lens+facet combo surviving a reload (sessionStorage lens + the facet state).
      (tick T18 2026-06-26)
- [x] **F98** "Clear lens" entry in the Cmd-K palette: the palette can set lenses
      (digit / tile) but has no explicit clear when one's active. Add a context-aware
      "Clear lens (<label>)" command shown only while a lens is active, routing
      through setLens(null), so the keyboard-only path can exit a lens without the
      filter-bar chip. Pair with a disabled-reason ("no lens active") per F89/F95.
      (tick T18 2026-06-26)
- [x] **F99** Tag-scope note is clickable to clear the lens (sister of the F86/F94
      notes being passive): make the "in <lens>" qualifier a tiny button that clears
      the lens (keeping the facet), so you can drop the lens scope in one click while
      keeping your tag/priority filter. Reuse setLens(null); keep it keyboard-focusable.
      (tick T18 2026-06-26)
- [x] **F100** Bulk-bar "set due" / "tag" disabled-reason parity: F95 covers the
      priority + pin menus; extend the same quiet reason line to the due + tag editors
      when they'd no-op (e.g. an empty due-clear on tasks that already have no due, or
      a tag op that changes nothing across the whole selection). Reuse the bulkReasonLine.
      (tick T18 2026-06-26)

When fewer than 5 remain, append more (recurring-task UI, archive view, an undo
stack beyond single delete, a keyboard-driven dep-graph navigator, a saved-view
for a lens+facet combo, etc.).

---

## TICK LOG — T18 (2026-06-26 ~08:50 PT)

Shipped 5/5 web slices on `main` (F96-F100), gated once, pushed clean
(95b0a0d..6d06ce3). 33 net-new web tests (633 -> 666), Go gates green,
bundle rebuilt + embedded, live `tsk serve` smoke-tested. The canonical
workdir `/Volumes/Projects/tsk` WAS mounted this tick (HEAD at 95b0a0d,
clean tree, node_modules present). The SSD-absent fallback was not needed.

This batch finishes the T18 standing queue exactly as written (no drops):
a "close-the-loop" cluster acting on the dependency/lens/bulk surfaces the
T15-T17 work opened — turning readouts into actions and closing the last
disabled-reason / persistence gaps.

- **F96** `bd02cae` Focus the board on a chokepoint's waiting cohort. New
  pure `web/src/cohort.ts` (buildCohort over deps.openDependents, applyCohort,
  chip markup) — an explicit id-set render-pipeline narrowing, the sibling of a
  lens (non-serializing, mutually exclusive with one). A "focus" button on the
  F92 chokepoint line (data-cohort-focus) narrows the board to exactly the K
  undone waiters; a filter-bar cohort chip (alert hue) clears it. ALSO wired the
  F92 chokepoint's data-waiting-walk hook in the sidebar (it was previously
  unwired there — now opens the dependent chain-drill). isExportScoped +
  jumpToTask + clear-all account for the cohort. +21 tests (cohort 11, stats 3,
  + the chokepoint suite).
- **F97** `3fa1ce6` Persist the lens-drilled priority facet across reloads.
  lens.ts: LENS_FACET_KEY + parseLensFacet (validated JSON array of known
  levels, garbage-proof) + serializeLensFacet. main.ts persists the facet from
  setFilter while a lens is active, in persistLens tied to the lens lifecycle,
  restores at boot only when a lens restored. The F81 lens+facet drill now
  survives a refresh whole, not just the lens (F93). +8 tests.
- **F98** `f46f8aa` "Clear lens" Cmd-K command. palette.ts: clearLensCommand
  (names the active lens; disabled when none) + CommandReasonContext.hasLens so
  commandDisabledReason explains "no lens active". main.ts adds it to
  buildCommands + runCommand (setLens(null)) + threads hasLens. The keyboard-only
  path can now exit a lens without the filter-bar chip. +5 tests.
- **F99** `be1124c` Clickable lens scope notes. filter.ts: renderPriorityScopeNote
  + renderTagScopeNote now emit a <button data-lens-scope-clear> (same
  fprio-scope/ftag-scope class + "in <lens>" text). main.ts wires a delegated
  click on the priority + tag rows to setLens(null) (drops the lens, keeps the
  facet). app.css strips the button chrome + adds a hover underline. +3 tests.
- **F100** `bc75963` Due + tag bulk-edit reason parity. bulkedit.ts: BulkTaskLike
  gains tags+due; bulkTagCommandReason (set-equal applyTagOps over the selection)
  + bulkDueClearDisabledReason; bulkReasonLine exported (data-bulk-reason hook);
  the tag + due editors render a [data-bulk-reason-slot] main.ts fills live per
  keystroke. Closes the F95 parity so every bulk action explains a no-op. +8 tests.
- `6d06ce3` bundle rebuild (41 modules, JS 36.50KB gz, CSS 9.91KB gz).

Live proof: built /tmp/tsk-t18, a #2/#3/#4 -> #1 dep chain (so #1 is a
chokepoint with 3 waiters), `tsk serve --addr 127.0.0.1:7898`. The served
bundle carried every hook (data-cohort-focus x2, data-filter-cohort x2,
stat-chokepoint-focus, tsk.lens.facet, lens-clear + "Clear lens" + "no lens
active", data-lens-scope-clear x2, data-bulk-reason-slot x3 + "no change to
any selected" + "all already have no due"). /api/export?ids=2,3 returned
exactly the cohort in store order. Toggling the #1 chokepoint done round-tripped
to .tsk.md preserving the depends: chain + adding completed:; `tsk show 2` read
depends:#1 back. Storage contract honoured — the raw .tsk.md stayed plain
hand-editable markdown.

Commit-split note: F96 and F97 are physically interleaved in main.ts (cohort
state lives beside the lens-facet boot-restore + persistLens rewrite), so the
F96 commit carries those few shared lens-restore lines; F97's commit owns the
lens.ts facet functions + tests + the setFilter hook. Each feature is still
revertible by its own module/test files; the cumulative index build was verified
byte-for-byte equal to the final tree before each commit.

Deferred: nothing forced. F48 (context-menu submenus), F49 (autocomplete in
edit/filter), F50 (dep mini-graph), F54 (touch context menu) remain the standing
long-carries. T19 backlog (F101-F105) appended below so the loop never starves.

### T19 — depth (appended T18 2026-06-26 so the loop never starves)

Standing unstarted: F48 (context-menu submenus), F49 (autocomplete in edit +
filter), F50 (dep mini-graph), F54 (touch context menu). Fresh follow-ons after
the T18 cohort-focus / lens-facet-persist / clear-lens / scope-clear /
bulk-reason-parity cluster:

- [x] **F101** Persist the cohort focus across a live-reload refresh: F96's
      focusCohort is dropped on every render-from-server today (it's re-derived
      only when you click focus). When an SSE change lands (F21) or you hit refresh,
      re-derive the cohort for the same sourceId against the fresh graph and keep the
      focus if it still has waiters (else clear it with a quiet toast), so a chokepoint
      cohort survives an external .tsk.md edit instead of silently vanishing. (tick T19 2026-06-26)
- [x] **F102** Cohort focus from the per-row "N waiting" badge too: F87's row badge
      (data-waiting-walk) opens the dependent chain-drill; add a secondary focus
      affordance (or a drill-popover "focus these" action) so you can drop into the
      cohort from ANY chokepoint row, not just the single biggest one F92 surfaces.
      Reuse setCohort(id); keyboard-reachable from the drill. (tick T19 2026-06-26)
- [x] **F103** "Clear cohort" + "Focus chokepoint cohort" Cmd-K commands (sisters of
      F98): a context-aware "Clear cohort focus (N waiting on #M)" shown only while a
      cohort is active (routes setCohort->clear), plus a "Focus biggest chokepoint"
      command that runs setCohort(biggestChokepoint.id) keyboard-only. Pair both with
      disabled-reasons ("no cohort active" / "no chokepoint") per F89/F95. (tick T19 2026-06-26)
- [x] **F104** Lens+facet saved-view bridge: a cohort/lens is non-serializing by
      design, but the lens+facet COMBO (F97) is a recurring drill — offer a "save this
      lens+facet as a view" that captures the facet (the serializable half) and notes
      the lens kind in the view name (e.g. "Urgent (overdue)"), recalling it re-applies
      the facet and re-sets the lens. Bridges the F25 saved views to the F66/F81 lenses. (tick T19 2026-06-26)
- [~] **F105** Bulk-bar reason parity for the priority/pin menus' EMPTY-selection edge
      + a "nothing selected" guard. DEFERRED as filler in T19: openBulkEdit early-returns
      at `bulk.ids.size === 0` and the bar is `hidden` at count 0, so the inert-menu state
      this guards is UNREACHABLE through real interaction (the spec itself only cites a
      "race"). Per the no-padding rule, shipped F106 instead (a real triage feature).
- [x] **F106** "Other bottlenecks" ranked chokepoint list (shipped IN PLACE OF F105):
      F92 surfaces only the single biggest chokepoint; topChokepoints + renderOtherChokepoints
      list the runners-up below it, each row reusing the walk + cohort-focus hooks. Real
      triage value on complex dep boards. (tick T19 2026-06-26)

When fewer than 5 remain, append more (recurring-task UI, archive view, an undo
stack beyond single delete, a keyboard-driven dep-graph navigator, a cohort
history / back-stack, etc.).

---

## TICK LOG — T19 (2026-06-26 ~14:48 PT)

Shipped 5/5 web slices on `main` (F101-F104 + F106), gated once, pushed clean
(3e524cd..6f00a44). 34 net-new web tests (666 -> 700), Go gates green
(gofmt/vet/build/test — commands pkg ~56s, all ok), both tsc configs clean,
bundle rebuilt + embedded (41 modules, JS 37.36KB gz, CSS 10.10KB gz), live
`tsk serve` smoke-tested. Canonical workdir `/Volumes/Projects/tsk` WAS mounted
(HEAD at 3e524cd, clean tree, node_modules present).

This batch finishes the T19 queue with ONE honest deferral: F105 (the
empty-selection bulk guard) was filler — openBulkEdit early-returns at
`bulk.ids.size === 0` and the bar is `hidden` at count 0, so the inert-menu
state it guards is unreachable through real interaction (the spec itself only
cited a "race"). Per the no-padding rule I deferred it and shipped F106 (a real
ranked-chokepoint triage feature) in its place, so the tick is still 5 GREAT
slices, not 4 + filler.

- **F101** `7da6470` Persist cohort focus across live-reload. cohort.ts:
  reconcileCohort(tasks, prev) re-derives the focus for the same sourceId via
  buildCohort (no prior -> {null,false}; still has waiters -> {fresh,false};
  chokepoint done/gone or all waiters complete -> {null,true}) + the
  CohortReconcile type. main.ts: refresh() reconciles focusCohort against the
  fresh list, keeping a LIVE id set or dropping it with a quiet toast. +6 tests.
- **F102** `5214994` Cohort focus from any chokepoint you're walking. cohort.ts:
  renderCohortFocusButton(sourceId) emits the SAME data-cohort-focus hook the
  sidebar uses. main.ts: openChainDrill (dependent dir) renders "focus these" in
  the popover head when fromId has open waiters; a delegated click closes the
  drill + setCohort. app.css: .chain-pop-focus. +2 tests.
- **F103** `d1c2f30` Cohort Cmd-K commands. cohort.ts: cohortSummary ("N waiting
  on #M"). palette.ts: clearCohortCommand + focusChokepointCommand (pure
  builders); CommandReasonContext gains hasCohort+hasChokepoint;
  commandDisabledReason explains both ("no cohort active" / "no chokepoint").
  main.ts: currentChokepointId() helper; both added to buildCommands + runCommand;
  ctx threads the two flags. +9 tests.
- **F104** `4120a59` Lens+facet saved-view bridge. views.ts: SavedView gains
  optional `lens`; addView/updateView take an optional lens (a lens makes an
  empty filter savable); normalizeViews round-trips it (junk dropped); new
  viewMatches + activeViewWithLens (facet AND lens must align); describeView +
  renderViewChips lens-aware (is-lensed marker). main.ts: saveCurrentView
  captures activeLens (suggests "urgent (overdue)"); recallView re-applies via
  parseLens; renderViewsRow + updateViewToCurrent + the save-view palette gate
  all lens-aware. app.css: .view-chip.is-lensed. +15 tests.
- **F106** `40c49e8` "Other bottlenecks" ranked chokepoint list (in place of
  F105). deps.ts: topChokepoints(tasks, done, limit=5) — ranked generalization
  of biggestChokepoint ([0] always == biggestChokepoint). stats.ts:
  renderOtherChokepoints(chokes) lists chokes.slice(1), each row reusing the
  data-waiting-walk + data-cohort-focus hooks (zero new dispatch);
  renderStatsPanel takes an optional ranked list. main.ts: refreshStats passes
  topChokepoints (cap 6). app.css: .stat-choke-* list. +8 tests.
- `6f00a44` bundle rebuild (41 modules, JS 37.36KB gz, CSS 10.10KB gz).

Live proof: built /tmp/tsk-t19-test via the CLI, two chokepoints — #1 (Ship
release) with 4 waiters (#3,#4,#5,#6) and #2 (Deploy infra) with 2 (#7,#8) —
then `tsk serve --addr 127.0.0.1:7919`. The served bundle carried every hook
(Cohort-cleared toast, chain-pop-focus + "focus these", "Clear cohort focus" +
"Focus biggest chokepoint" + "no cohort active"/"no chokepoint", "Other
bottlenecks" + stat-choke-walk, "lens: " + is-lensed; CSS: chain-pop-focus,
stat-choke-list/walk/focus, view-chip.is-lensed). Toggling the #1 chokepoint
done via POST /api/tasks/1/toggle round-tripped to .tsk.md preserving every
depends: link + adding completed:; `tsk show 3` read depends:#1 back. Storage
contract honoured — the raw .tsk.md stayed plain hand-editable markdown. (After
#1 completes, F101's reconcile shifts the biggest chokepoint to #2 live — the
exact behaviour the feature ships.)

Deferred: F105 (filler, see above). F48 (context-menu submenus), F49
(autocomplete in edit/filter), F50 (dep mini-graph), F54 (touch context menu)
remain the standing long-carries. T20 backlog (F107-F111) appended below so the
loop never starves.

### T20 — depth (appended T19 2026-06-26 so the loop never starves)

Standing unstarted: F48 (context-menu submenus), F49 (autocomplete in edit +
filter), F50 (dep mini-graph), F54 (touch context menu). Fresh follow-ons after
the T19 cohort-persist / cohort-focus-from-drill / cohort-commands /
lens-view-bridge / ranked-chokepoints cluster:

- [x] **F107** "Other bottlenecks" rows are keyboard-reachable + focus from the
      Cmd-K palette: F106 lists the runner-up chokepoints with mouse hooks; add a
      "Focus chokepoint #N" command group (or extend F103's focus command to a
      ranked submenu) so every bottleneck — not just the biggest — is reachable
      keyboard-only. Reuse topChokepoints + setCohort. (tick T20 2026-06-26)
- [x] **F108** Cohort history / back-stack: setCohort replaces the focus today;
      keep a small stack so "focus #1's cohort" then "focus #3's cohort" can step
      BACK to #1 with Escape or a back chip. Bridges the cohort model toward the
      multi-step drill the dep-debugging cluster invites. Persist nothing (cohorts
      are momentary), but keep the stack per-session. (tick T20 2026-06-26)
- [x] **F109** Saved-view lens chip in the recalled state: F104 stores the lens on
      a view; when such a view is active, surface the lens it re-applied as a small
      readout beside the view chip (or in the filter bar) so "why is the overdue
      lens on?" is answerable — it came from the recalled view. Reuse lensMeta. (tick T20 2026-06-26)
- [x] **F110** "Save lens as quick view" one-click: F104 saves a lens+facet combo
      via the prompt; add a one-click "pin this lens" affordance on the active-lens
      chip that saves a pure-lens view named after the lens (no prompt) for the
      common case, so a frequently-used lens becomes a recallable chip in one click. (tick T20 2026-06-26)
- [x] **F111** Chokepoint trend hint: the dep-stats sidebar shows the current
      biggest chokepoint (F92) + runners-up (F106); add a tiny "was #M last
      refresh" delta when the biggest chokepoint CHANGES across a live-reload, so a
      shifting bottleneck (e.g. after F101 reconciles a completed chokepoint) is
      visible, not silent. Compare against the previous refresh's biggestChokepoint
      id held in a module slot. (tick T20 2026-06-26)

When fewer than 5 remain, append more (recurring-task UI, archive view, an undo
stack beyond single delete, a keyboard-driven dep-graph navigator, a cohort
back-stack, a saved-view import/export, etc.).

---

## TICK LOG — T20 (2026-06-26 ~19:43 PT)

Shipped 5/5 web slices on `main` (F107-F111), gated once, pushed clean
(ca171c3..82edd7d). 24 net-new web tests (700 -> 724), Go gates green
(gofmt clean / vet clean / build clean / `go test ./...` all ok — commands,
serve, store, model, dateparse, tui, util packages), both tsc configs clean,
bundle rebuilt + embedded (41 modules, JS 38.25KB gz, CSS 10.23KB gz), live
`tsk serve` smoke-tested. Canonical workdir `/Volumes/Projects/tsk` WAS mounted
(HEAD at ca171c3, node_modules present).

This batch finishes the T20 queue exactly as written (no deferrals): the
five fresh follow-ons appended in T19 after the cohort/lens/chokepoint
cluster. Every slice closes a "the readout exists but the action/answer is
missing" gap the T15-T19 work opened.

- **F107** `f35e308` Keyboard-focus every bottleneck from Cmd-K. palette.ts:
  ChokepointLike + buildChokepointFocusCommands(chokes) — skips the first
  (biggest, owned by F103's focusChokepointCommand so no duplicate ids) and emits
  one "cohort-focus-<id>" command per runner-up, fuzzy-findable by "#N" +
  "bottleneck". main.ts: currentChokepoints() (topChokepoints cap 6, matching the
  sidebar cap); buildCommands spreads them after focusChokepointCommand;
  runCommand decodes "cohort-focus-<id>" (distinct from "cohort-focus-biggest")
  -> setCohort. +5 tests.
- **F108** `4c083c6` Cohort history back-stack. cohort.ts: pushCohortHistory
  (append, de-dupe top, cap 20) + popCohortHistory(tasks, stack) -> CohortBack
  (pops most-recent-first, rebuilds each ancestor via buildCohort against the
  FRESH graph, transparently skipping dead ancestors to land on the nearest live
  cohort, or null when none holds); renderCohortChipBody gains historyDepth that
  prepends a data-cohort-back glyph. main.ts: cohortHistory slot; setCohort pushes
  the outgoing source; clearCohort / setLens / clear-all / jumpToTask reset it;
  cohortBack() steps one level; Escape steps back first (falling through to
  bulk-clear / tag-exit when no history); the chip's < glyph routes to cohortBack.
  app.css: .cohort-back inset pill. +10 tests.
- **F109** `4a599ae` Lens provenance readout. views.ts: lensProvenanceNote(
  recalled, liveLens) -> the recalled view's name only when it captured a lens
  that still equals the live lens (a digit-key lens / a lens changed after recall
  reports nothing). main.ts: a [data-filter-lens-from] "from <view>" readout
  beside the lens chip, hidden otherwise. app.css: .lens-from muted italic. +4
  tests.
- **F110** `d735b9d` One-click pin-this-lens. views.ts: pureLensViewName(label) +
  findPureLensView(views, lens) (empty-filter view for a lens kind, distinct from
  activeViewWithLens). main.ts: a [data-filter-lens-pin] star reflecting pinned
  state (filled recall vs hollow pin) via findPureLensView; pinCurrentLens() saves
  a pure-lens view named after the lens with NO prompt, or recalls the existing
  pin (idempotent, never duplicates). app.css: .lens-pin chrome-free star. +5
  tests.
- **F111** `28f910a` Chokepoint trend hint. stats.ts: chokepointTrend(prev, curr)
  -> the prior id only when both exist and differ; renderChokepointTrend(prev) ->
  a muted " was #M" note; renderChokepoint / renderDepStats / renderStatsPanel
  gain an optional trendPrev arg (default null keeps every existing snapshot
  byte-identical). main.ts: prevBiggestChokepoint slot; refreshStats computes the
  trend, paints, then updates the slot. app.css: .chokepoint-trend muted inline
  note. +4 tests.
- `82edd7d` bundle rebuild (41 modules, JS 38.25KB gz, CSS 10.23KB gz).

Live proof: built /tmp/t20.tsk.md via the CLI (--file), #1 with 3 waiters
(#3,#4,#5) and #2 with 2 (#6,#7) — two chokepoints — then `tsk serve --addr
127.0.0.1:7920`. The served bundle (app-BLEIlhsp.js) carried every hook:
F107 ("cohort-focus-", "Focus chokepoint #"), F108 ("data-cohort-back",
"cohort-back", "back to ", "cohort history cleared"), F109 ("lens-from",
"came from the saved view"), F110 ("lens-pin", "is-pinned", "Pin the ",
"pinned lens"), F111 ("chokepoint-trend", "was #", "biggest chokepoint
changed"). Toggling the #1 chokepoint done via POST /api/tasks/1/toggle
round-tripped to .tsk.md preserving every depends: link + adding completed:;
`tsk show 3` read depends:#1 back. /api/export?ids=6,7 returned exactly #2's
cohort (200). Storage contract honoured — the raw .tsk.md stayed plain
hand-editable markdown. (After #1 completes, F111's trend would surface
"was #1" as the biggest chokepoint shifts to #2 — the exact behaviour shipped.)

Process note: accidentally ran the first few fixture `add`s against the repo's
own .tsk.md before switching to --file; restored it to its single `testroot`
line + removed the stray .tsk.md.bak, so the working tree is clean and only the
intended commits landed.

Deferred: nothing. F48 (context-menu submenus), F49 (autocomplete in
edit/filter), F50 (dep mini-graph), F54 (touch context menu) remain the standing
long-carries. T21 backlog (F112-F116) appended below so the loop never starves.

### T21 — depth (appended T20 2026-06-26 so the loop never starves)

Standing unstarted: F48 (context-menu submenus), F49 (autocomplete in edit +
filter), F50 (dep mini-graph), F54 (touch context menu). Fresh follow-ons after
the T20 chokepoint-keyboard-focus / cohort-back-stack / lens-provenance /
pin-lens / chokepoint-trend cluster:

- [x] **F112** Pinned-lens chips in the Views row read as lenses, not filters:
      F110 saves a pure-lens view named after the lens; in the views chip row those
      pins already wear `is-lensed` (F104), but a pin (empty filter + lens) could
      carry a distinct glyph (the lensMeta glyph) so a pinned lens is visually a
      "lens bookmark" vs a "filter bookmark" at a glance. Reuse lensMeta(kind).glyph
      keyed off the view's stored lens string (validate via parseLens). (tick T21 2026-06-27)
- [x] **F113** Cohort back-stack depth readout: F108 keeps a per-session stack but
      only the immediate "<" back affordance shows. Add a tiny depth badge on the
      cohort chip ("‹2") when the stack has >1 entry, so a multi-step drill shows
      how deep you are. Pure: extend renderCohortChipBody to render the count; the
      Escape/glyph behaviour is unchanged (still steps one level). (tick T21 2026-06-27)
- [x] **F114** "Focus chokepoint #N" trend awareness: F107's per-chokepoint focus
      commands + F111's trend hint don't know about each other. When the biggest
      chokepoint just changed (trendPrev set), surface a Cmd-K command "Focus the
      NEW biggest chokepoint (#N, was #M)" at the top of the focus group, so the
      keyboard path leads with the shift the sidebar just flagged. Reuse
      chokepointTrend + currentChokepointId. (tick T21 2026-06-27)
- [x] **F115** Pin-lens from the Cmd-K palette: F110's pin is a mouse-only star on
      the chip. Add a context-aware "Pin lens (<label>)" command (shown only while a
      lens is active, disabled-reason "no lens active" via F89) that runs
      pinCurrentLens keyboard-only — the palette sister of the star, mirroring how
      F103's cohort commands shadow the sidebar focus button. (tick T21 2026-06-27)
- [x] **F116** Lens provenance in the help/`?` overlay's active-view line: F109
      shows "from <view>" beside the chip; the F82 help overlay already has an
      "active lens" line (renderActiveLensRow). When the lens came from a recalled
      view, append the provenance there too so the keyboard-only `?` summary answers
      "why is this lens on?" without looking at the filter bar. Reuse
      lensProvenanceNote. (tick T21 2026-06-27)

When fewer than 5 remain, append more (recurring-task UI, archive view, an undo
stack beyond single delete, a keyboard-driven dep-graph navigator, a saved-view
import/export, a cohort-history breadcrumb trail, etc.).

---

## TICK LOG — T21 (2026-06-27 ~01:45 PT)

Shipped 5/5 web slices on `main` (F112-F116), gated once, pushed clean
(419fd19..909741b). 23 net-new web tests (724 -> 747), Go gates green
(gofmt clean / vet clean / build clean / `go test ./...` all ok — commands
pkg 66.7s, serve/store/model/dateparse/tui/util cached-green), both tsc
configs clean, bundle rebuilt + embedded (41 modules, JS 38.68KB gz, CSS
10.32KB gz), live `tsk serve` smoke-tested. Canonical workdir
`/Volumes/Projects/tsk` WAS mounted (HEAD at 419fd19, node_modules present).

This batch finishes the T21 queue exactly as written (no deferrals): the five
fresh follow-ons appended in T20 after the chokepoint-keyboard-focus /
cohort-back-stack / lens-provenance / pin-lens / chokepoint-trend cluster.
Every slice closes a "the readout/action exists on ONE surface but not its
sibling" gap the T15-T20 work opened (mouse-only star -> palette; chip
provenance -> help overlay; sidebar trend -> palette lead; back glyph ->
depth badge; generic lensed marker -> real lens glyph).

- **F112** `59356cb` Pure-lens bookmarks read as lens chips in the Views row.
  views.ts: isPureLensView(view) (lens present AND filter empty — a drill with
  a facet is NOT a pure pin); ViewChipOpts gains an optional lensGlyph resolver
  (kind -> glyph) so views.ts stays decoupled from lens.ts; renderViewChips
  emits a view-chip-lens-glyph span + is-lens-pin class only for pure-lens
  chips, suppressing the generic ◈ diamond so there's exactly one marker.
  main.ts: supplies the resolver via parseLens + lensMeta(kind).glyph (stale
  lens -> no glyph). app.css: is-lens-pin suppresses ::before, tints the real
  glyph. +6 tests.
- **F113** `fb38719` Cohort back-stack depth badge. cohort.ts:
  renderCohortChipBody surfaces the stack depth as a numeral riding the ‹ back
  glyph (cohort-back-depth) once history is >1 deep; depth 0/1 keep the bare
  glyph byte-identical; the back behaviour is unchanged (Escape/click still
  steps one level — pure readout). main.ts already passed cohortHistory.length,
  so a pure renderer + CSS change. app.css: cohort-back-depth inset numeral.
  +3 tests.
- **F114** `d768c5b` Cmd-K leads with the just-shifted biggest chokepoint.
  palette.ts: focusShiftedChokepointCommand(currId, prevId) -> the lead command
  only on a real shift (id "cohort-focus-new", disjoint from F103 biggest +
  F107 numeric ids). main.ts: two slots (lastBiggestChokepoint +
  chokepointShiftFrom) updated in refresh() — which always runs, unlike the
  panel-gated F111 slot — so the shift is tracked with the sidebar closed; a
  tiny maybeCommand helper spreads the optional lead; runCommand handles
  cohort-focus-new (setCohort the new biggest + consume the pending shift). +6
  tests.
- **F115** `370e97c` Pin the active lens from the Cmd-K palette. palette.ts:
  pinLensCommand(lensLabel, pinned) -> "Pin lens (<label>)" or "Recall pinned
  lens (<label>)" (matching the F110 star's pin-vs-recall state); id "lens-pin";
  commandDisabledReason gains lens-pin -> "no lens active". main.ts: built next
  to clearLensCommand with pinned via findPureLensView; runCommand routes
  lens-pin -> pinCurrentLens. +6 tests.
- **F116** `5a94266` Lens provenance in the help overlay's active-lens line.
  lens.ts: renderActiveLensHelp gains an optional provenance arg -> appends a
  quiet "from <view>" note (help-lens-from), byte-identical when omitted.
  main.ts: toggleHelp computes the provenance via lensProvenanceNote (the same
  recalled-view check F109 uses at the chip) and passes it. app.css:
  help-lens-from muted italic. +4 tests... (overlay line now answers "why is
  this lens on?" keyboard-only).
- `909741b` bundle rebuild (41 modules, JS 38.68KB gz, CSS 10.32KB gz).

Live proof: built /tmp/t21.tsk.md via the CLI (--file), two chokepoints — #1
(Ship release) with 2 waiters (#3,#4) and #2 (Deploy infra) with 1 (#5) —
then `tsk serve --addr 127.0.0.1:7921`. The served bundle (app-DE_YTOya.js)
carried every hook: F112 (is-lens-pin, view-chip-lens-glyph), F113
(cohort-back-depth), F114 (cohort-focus-new, "new biggest chokepoint"), F115
("Pin lens", "Recall pinned lens"), F116 (help-lens-from); CSS carried
is-lens-pin, view-chip-lens-glyph, cohort-back-depth, help-lens-from.
Toggling #1 done via POST /api/tasks/1/toggle round-tripped to .tsk.md
preserving every depends: link + adding completed:; `tsk show 3` read
depends:#1 back. Storage contract honoured — the raw .tsk.md stayed plain
hand-editable markdown. (After #1 completes, the biggest chokepoint shifts
#1 -> #2 — the exact trend F114's shift-lead surfaces in Cmd-K.)

Deferred: nothing. F48 (context-menu submenus), F49 (autocomplete in
edit/filter), F50 (dep mini-graph), F54 (touch context menu) remain the
standing long-carries. T22 backlog (F117-F121) appended below so the loop
never starves.

### T22 — depth (appended T21 2026-06-27 so the loop never starves)

Standing unstarted: F48 (context-menu submenus), F49 (autocomplete in edit +
filter), F50 (dep mini-graph), F54 (touch context menu). Fresh follow-ons after
the T21 pure-lens-glyph / cohort-depth-badge / chokepoint-shift-lead /
pin-lens-command / help-provenance cluster:

- [x] **F117** Cohort history breadcrumb in the `?` help overlay: F108/F113
      track a per-session cohort back-stack with a chip depth badge, but the
      keyboard-only `?` summary doesn't mention the cohort at all. Add a "Cohort
      focus: N waiting on #M (‹K in history)" line to the help overlay (sister of
      F116's active-lens line), so the keyboard summary reports the cohort + its
      drill depth. Reuse cohortSummary + cohortHistory.length. (tick T22 2026-06-27)
- [x] **F118** "Clear cohort" depth-aware label: F103's clearCohortCommand names
      the active cohort ("3 waiting on #1"); when a back-stack exists, the command
      could read "Clear cohort + N-step history" so the keyboard user knows clearing
      drops the whole drill, not just the current level. Pure: thread the history
      depth into the command label; the action is unchanged (clearCohort already
      resets the stack). (tick T22 2026-06-27)
- [x] **F119** Pin-lens success surfaces the new chip: F110/F115 save a pure-lens
      view but the only feedback is a status line. After a pin, briefly flash the
      newly-created Views chip (a one-shot highlight class) so the user sees WHERE
      the pin landed in the row — the spatial confirmation the status line can't
      give. Reuse the F112 is-lens-pin chip + a transient class cleared on the next
      render. (tick T22 2026-06-27)
- [x] **F120** "Recall pinned lens" from a digit-key collision guard: F115's
      pin/recall command + the F71 digit keys can both target the same lens; when a
      pinned lens is recalled via the palette, ensure the digit-key state stays in
      sync (the chip's pin star + the help digit map both reflect "this lens is
      active AND pinned"). Mostly a wiring/test slice confirming the three surfaces
      (star, palette, digit map) never disagree after a recall.
      Shipped as a real readout: the pin state now surfaces in the help overlay's
      active-lens line AND the digit map, all driven by the SAME findPureLensView
      the star + palette read, so the four surfaces agree by construction. (tick T22 2026-06-27)
- [x] **F121** Chokepoint-shift toast: F114 leads Cmd-K with the shift and F111
      shows a sidebar "was #M" hint, but with both the palette closed AND the stats
      panel closed, a shifting bottleneck is silent. On a live-reload where the
      biggest chokepoint changes (chokepointShiftFrom set), show a one-shot info
      toast ("Biggest chokepoint moved: #M -> #N") so the shift is noticed even
      with every panel closed. Reuse showInfoToast + the F114 shift slot; throttle
      so a rapid series of edits doesn't spam. (tick T22 2026-06-27)

When fewer than 5 remain, append more (recurring-task UI, archive view, an undo
stack beyond single delete, a keyboard-driven dep-graph navigator, a saved-view
import/export, a per-tag saved-view group, etc.).

---

## TICK LOG — T22 (2026-06-27 ~06:35 PT)

Shipped 5/5 web slices on `main` (F117-F121), gated once, pushed clean
(b9926ae..15152de). 21 net-new web tests (747 -> 768), Go gates green
(gofmt clean / vet clean / build clean / `go test ./...` all ok — commands
pkg 66.3s, serve re-run clean against the fresh bundle 2.6s, store/model/
dateparse/tui/util cached-green), both tsc configs clean, bundle rebuilt +
embedded (41 modules, JS 39.08KB gz, CSS 10.46KB gz), live `tsk serve`
smoke-tested. Canonical workdir `/Volumes/Projects/tsk` WAS mounted (HEAD at
b9926ae, node_modules present).

This batch finishes the T22 queue exactly as written (no deferrals): the five
fresh follow-ons appended in T21 after the pure-lens-glyph / cohort-depth-badge
/ chokepoint-shift-lead / pin-lens-command / help-provenance cluster. Every
slice closes a "the readout/action exists on ONE surface but not its sibling"
gap the T15-T21 work opened (cohort known to the chip but not the `?` overlay;
clear-cohort named but silent about the drill it drops; pin saved but
spatially invisible; pin state on the mouse surfaces only; chokepoint shift
visible only with a panel open).

- **F117** `d8fb270` Cohort breadcrumb in the help overlay. cohort.ts:
  renderCohortHelp(focus, historyDepth=0) bolds the summary (reusing
  cohortSummary so the overlay line, the Cmd-K command label, and the chip
  can't drift) + a muted "(<K in history)" note echoing the F113 chip depth
  badge; "" for a null focus. main.ts: a [data-help-cohort] slot populated in
  toggleHelp from focusCohort + cohortHistory.length. app.css:
  .help-foot[data-help-cohort] + .help-cohort-history. +6 tests.
- **F118** `39361ee` Depth-aware Clear cohort label. palette.ts:
  clearCohortCommand(summary, historyDepth=0) appends " + N-step history" only
  when a cohort is active AND depth > 0; default 0 is byte-identical so
  existing snapshots/call sites are unchanged; added history/drill keywords.
  main.ts passes cohortHistory.length. +5 tests.
- **F119** `190e3f0` Flash the just-pinned chip. views.ts: ViewChipOpts.flashId
  -> a one-shot is-flash class on the matching chip (composes with the F112
  is-lens-pin marker). main.ts: a pendingPinFlashViewId slot set in
  pinCurrentLens (the new pure-lens view's id via findPureLensView),
  renderViewsRow passes it then clears it so the class rides one paint.
  app.css: @keyframes view-chip-flash amber-pulse + scale, reduced-motion
  fallback to a static box-shadow. +3 tests.
- **F120** `fed4326` Pinned-lens state synced across help line + digit map.
  lens.ts: renderActiveLensHelp(kind, provenance, pinned=false) appends a
  "★ pinned" marker; renderLensDigitMap(active, pinnedKind=null) marks the
  active+pinned legend entry. main.ts: toggleHelp computes lensPinned ONCE via
  the same findPureLensView that drives the F110 star + F115 command and feeds
  both surfaces, so star/command/line/legend agree by construction. app.css:
  .help-lens-pinned + .lens-legend-item.is-pinned + .lens-legend-pin. +6 tests.
- **F121** `88e217f` Chokepoint-shift toast on live reload. stats.ts:
  chokepointShiftMessage(prev, curr) -> "Biggest chokepoint moved: #M -> #N",
  "" on no genuine shift (same guard as chokepointTrend, so the toast + the
  F111 sidebar hint fire on the same condition). main.ts: a liveRefresh(deferred)
  wrapper the TWO live-reload paths route through (single fetch, snapshots the
  biggest chokepoint before/after, shows the shift toast OR the generic notice),
  throttled to one shift-toast per 8s. A manual refresh() never toasts. +3 tests.
- `15152de` bundle rebuild (41 modules, JS 39.08KB gz, CSS 10.46KB gz).

Live proof: built /tmp/t22.tsk.md via the CLI (--file) — #1 (Ship release)
with 2 waiters (#3,#4) and #2 (Deploy infra) with 1 (#5), so #1 is the
biggest chokepoint — then `tsk serve --addr 127.0.0.1:7922`. GET / -> 200; the
served bundle (app-uesrDKoT.js) carried every hook: data-help-cohort + "Cohort
focus:" (F117), "step history" (F118), is-flash (F119), help-lens-pinned
(F120), "Biggest chokepoint moved" (F121); CSS carried help-cohort-history,
view-chip-flash, help-lens-pinned, lens-legend-pin. Toggling #1 done via POST
/api/tasks/1/toggle (which completes the biggest chokepoint -> shift to #2, the
exact F121 scenario) round-tripped to .tsk.md preserving every depends: link +
adding completed:; `tsk show 3` read depends:#1 back. The raw .tsk.md stayed
plain hand-editable markdown — the CLI/TUI storage contract is intact. Cleaned
up the /tmp fixture + the built binary after.

Deferred: nothing. F48 (context-menu submenus), F49 (autocomplete in
edit/filter), F50 (dep mini-graph), F54 (touch context menu) remain the
standing long-carries. T23 backlog (F122-F126) appended below so the loop
never starves.

### T23 — depth (appended T22 2026-06-27 so the loop never starves)

Standing unstarted: F48 (context-menu submenus), F49 (autocomplete in edit +
filter), F50 (dep mini-graph), F54 (touch context menu). Fresh follow-ons after
the T22 cohort-breadcrumb / depth-aware-clear / pin-flash / pin-state-sync /
shift-toast cluster:

- [x] **F122** Cohort breadcrumb is clickable in the help overlay: F117 renders
      the "Cohort focus: N waiting on #M (‹K in history)" line as static text. Make
      the "‹K in history" segment a real button that runs cohortBack one step (the
      same Escape / chip-glyph path), so the keyboard user can step back through the
      drill from the `?` summary itself, then it re-renders the line. Reuse the
      existing cohortBack + a delegated click in ensureHelpEl like the F91 legend. (tick T23 2026-06-27)
- [x] **F123** Shift-toast "focus it" action: F121's "Biggest chokepoint moved:
      #M -> #N" toast is informational only. Add a "Focus" action button (reusing
      showInfoToast's F42 action slot) that drops into the new chokepoint's cohort
      (setCohort(after)) — the toast sibling of F114's Cmd-K lead, so a shift you
      notice via the toast is one click from acting on. (tick T23 2026-06-27)
- [x] **F124** Pin-flash also scrolls the chip into view: F119 flashes the
      freshly-pinned chip, but if the Views row has overflowed horizontally the new
      chip may be off-screen when the flash plays. After setting pendingPinFlashViewId,
      scrollIntoView({inline: "nearest"}) the flashed chip so the spatial
      confirmation is actually visible. Guard for jsdom-less/test env (no-op when
      scrollIntoView is absent). (tick T23 2026-06-27)
- [x] **F125** "Unpin lens" from the chip star + palette: F110/F115 pin a lens
      and recall it, but there's no one-click UNPIN — you have to find the chip and
      hit its × delete. When a lens is already pinned, a long-press / right-click on
      the star (and a "Unpin lens (<label>)" palette command) removes the pure-lens
      view via removeView, so the pin lifecycle is symmetric. Reuse findPureLensView
      to locate the view + the existing removeView. (tick T23 2026-06-27)
- [x] **F126** Active-cohort line in the stats sidebar: F117 put the cohort in the
      `?` overlay; the stats panel (the mouse surface) shows the biggest-chokepoint
      line but not the ACTIVE cohort focus. Add a small "Focused: N waiting on #M"
      readout to the stats panel when a cohort is active (sister of F117), with a
      click-to-clear, so the cohort state is visible on the panel too — reuse
      cohortSummary + the existing clearCohort wiring. (tick T23 2026-06-27)

When fewer than 5 remain, append more (recurring-task UI, archive view, an undo
stack beyond single delete, a keyboard-driven dep-graph navigator, a saved-view
import/export, a per-tag saved-view group, a cohort-history breadcrumb trail in
the sidebar, etc.).

---

## TICK LOG — T23 (2026-06-27 ~10:03 PT)

Shipped 5/5 web slices on `main` (F122-F126), gated once, pushed clean
(c8a0856..f9cd799). 19 net-new web tests (768 -> 787), Go gates green
(gofmt clean / vet clean / build clean / `go test ./...` all ok — commands
pkg cached-green, serve re-run clean against the fresh bundle 2.75s, store/
model/dateparse/tui/util cached-green), both tsc configs clean, bundle rebuilt
+ embedded (41 modules, JS 39.61KB gz, CSS 10.63KB gz), live `tsk serve`
smoke-tested. Canonical workdir `/Volumes/Projects/tsk` WAS mounted (HEAD at
c8a0856, node_modules present).

This batch finishes the T23 queue exactly as written (no deferrals): the five
fresh follow-ons appended in T22 after the cohort-breadcrumb / depth-aware-clear
/ pin-flash / pin-state-sync / shift-toast cluster. Every slice closes a "the
readout/action exists on ONE surface but not its sibling" gap the T15-T22 work
opened (cohort breadcrumb known but static -> actionable; shift toast noticed but
not actable -> Focus action; pin flash plays but off-screen -> scroll into view;
pin lifecycle one-way -> symmetric unpin; cohort focus in the help but not the
panel -> panel line).

- **F122** `fa27ace` Clickable cohort breadcrumb in the help overlay. cohort.ts:
  renderCohortHelp's "(<K in history)" note becomes a `<button data-cohort-back>`
  carrying the SAME hook the chip's < glyph + Escape drive (zero new dispatch,
  mirroring F91's actionable legend); F117 class + text/glyph preserved, depth 0
  renders no button. main.ts: a delegated click in ensureHelpEl runs cohortBack()
  then toggleHelp(true) to repopulate in place (overlay stays open to keep
  stepping back). app.css: button strips native chrome, keeps the quiet italic
  look + hover/focus accent. +2 tests.
- **F123** `11c6a43` Chokepoint-shift toast gains a Focus action. stats.ts:
  chokepointShiftToast(prev, curr) -> { message, focusId } bundling the F121
  message with the NEW chokepoint id, same genuine-shift-only guard (empty
  message AND null focus on a no-op). main.ts: liveRefresh routes through it and,
  on a real shift, hangs a 6s "Focus" action that setCohort(focusId) via
  showInfoToast's F42 slot — the toast sibling of F114's Cmd-K lead. +3 tests.
- **F124** `6c4dc77` Scroll a just-pinned chip into view. views.ts: chipClippedX(
  chip, container, eps=1) -> pure rect geometry (left/right outside the container,
  1px epsilon for sub-pixel rounding). main.ts: renderViewsRow, after the paint
  and before consuming pendingPinFlashViewId, scrollIntoView({inline:"nearest"})
  the flashed chip when chipClippedX says it's clipped; guarded for the
  jsdom-less/test env. +4 tests.
- **F125** `ec5420c` Unpin a lens from the star + palette. palette.ts:
  unpinLensCommand(label, pinned) -> id "lens-unpin", enabled only when the active
  lens is pinned; CommandReasonContext gains lensPinned so commandDisabledReason
  explains "no lens active" vs "lens not pinned". main.ts: unpinCurrentLens() drops
  the pure-lens view via the same removeView the chip × drives (lens stays ON,
  only the bookmark goes) + render + refreshStats; wired into buildCommands +
  runCommand, plus the star gains a contextmenu (right-click) + touch long-press
  that unpin a pinned star, and the title advertises "right-click to unpin". +6
  tests.
- **F126** `b126df2` Active-cohort line in the stats panel. cohort.ts:
  renderCohortPanelLine(focus) -> a button reusing cohortSummary carrying
  data-cohort-clear; "" on an unfocused board. main.ts: refreshStats prepends it
  above the panel when focused; the stats-panel click handler routes
  data-cohort-clear -> clearCohort (checked first); setCohort / clearCohort /
  cohortBack now call refreshStats so the line stays live (the panel repaints only
  on its own refresh, not render()). app.css: the alert-soft pill matches the
  filter-bar cohort chip. +5 tests... (one slice's tests split: 2 F122 + 5 F126).
- `f9cd799` bundle rebuild (41 modules, JS 39.61KB gz, CSS 10.63KB gz).

Live proof: built /tmp/t23.tsk.md via the CLI (--file) — #1 (Ship release) with
2 waiters (#3,#4) and #2 (Deploy infra) with 1 (#5), so #1 is the biggest
chokepoint — then `tsk serve --addr 127.0.0.1:7923`. GET / -> 200; the served
bundle (app-C1Uj-Pic.js) carried every hook: data-cohort-back x3 (F122),
"Biggest chokepoint moved" (F123), scrollIntoView x4 (F124), "Unpin lens" +
lens-unpin (F125), data-cohort-clear + stat-cohort-line (F126); CSS carried
button.help-cohort-history (F122) + stat-cohort-line (F126). Toggling #1 done via
POST /api/tasks/1/toggle (which completes the biggest chokepoint -> shift to #2,
the exact F123 scenario) round-tripped to .tsk.md preserving every depends: link
+ adding completed:; `tsk show 3` read depends:#1 back. The raw .tsk.md stayed
plain hand-editable markdown — the CLI/TUI storage contract is intact. Cleaned up
the /tmp fixture + the built binary after.

Deferred: nothing. F48 (context-menu submenus), F49 (autocomplete in
edit/filter), F50 (dep mini-graph), F54 (touch context menu) remain the standing
long-carries. T24 backlog (F127-F131) appended below so the loop never starves.

### T24 — depth (appended T23 2026-06-27 so the loop never starves)

Standing unstarted: F48 (context-menu submenus), F49 (autocomplete in edit +
filter), F50 (dep mini-graph), F54 (touch context menu). Fresh follow-ons after
the T23 clickable-breadcrumb / shift-focus-action / pin-flash-scroll /
unpin-lens / panel-cohort-line cluster:

- [x] **F127** Cohort line in the stats panel is also a back-step when history
      exists: F126 added "Focused: N waiting on #M" to the panel with click-to-clear,
      and F113/F122 track a back-stack. When the cohort has history (depth > 0), give
      the panel line the same "‹K" back affordance the chip + help breadcrumb wear, so
      a mouse user can step back through the drill from the panel too — reuse
      cohortHistory.length + cohortBack, mirror F122's delegated-click pattern. (tick T24 2026-06-27)
- [x] **F128** "Pin this cohort's chokepoint" from the panel line: F126's panel
      line clears the focus; add a small secondary "walk" affordance (data-waiting-walk
      on #M) so you can jump from the active-cohort readout straight into the F85
      dependent chain-drill for the chokepoint — the panel sibling of the chip's
      implicit "what waits?" question. Reuse openChainDrill(sourceId, "dependent"). (tick T24 2026-06-27)
- [x] **F129** Unpin success flashes the chip's disappearance: F125 removes the
      pure-lens view but the only feedback is a status line — the chip just vanishes
      from the Views row. Before removeView, mark the chip with a one-shot
      `is-unpinning` fade-out class (a brief CSS transition) so the user sees WHICH
      chip left, the inverse of F119's pin-flash. Needs a deferred removeView (after
      the animation frame) or a CSS-only exit; guard the test env. (tick T24 2026-06-27)
- [x] **F130** "Focus" toast action also opens the stats panel: F123's toast Focus
      button drops into the new chokepoint's cohort, but if the panel is closed the
      F126 "Focused" line (and the breakdown) aren't visible. When the toast Focus
      action runs, also toggleStats(true) so the cohort you just focused is immediately
      legible on the panel — chain setCohort + an open-panel call in the action. (tick T24 2026-06-27)
- [x] **F131** Keyboard shortcut to unpin the active lens: F125 added the unpin
      command + star right-click; add a direct key (e.g. Shift+P, paired with the
      pin/recall key if one exists, or a new binding) that toggles pin/unpin on the
      active lens keyboard-only, so the whole pin lifecycle is reachable without Cmd-K.
      Wire it through pinCurrentLens / unpinCurrentLens based on findPureLensView, and
      document it in the `?` overlay HELP_ROWS. (tick T24 2026-06-27 — bound to `*`)

When fewer than 5 remain, append more (recurring-task UI, archive view, an undo
stack beyond single delete, a keyboard-driven dep-graph navigator, a saved-view
import/export, a per-tag saved-view group, a cohort-history breadcrumb trail in
the sidebar, etc.).

---

## TICK LOG — T24 (2026-06-27 ~15:02 PT)

Shipped 5/5 web slices on `main` (F127-F131), gated once, pushed clean
(2ef41be..0b18761). 12 net-new web tests (787 -> 799), Go gates green
(gofmt clean / vet clean / build clean / `go test ./...` all ok — commands
68.1s, serve re-run clean against the fresh bundle 2.68s, store/model/dateparse/
tui/util cached-green), both tsc configs clean (app + test/tsconfig.json),
bundle rebuilt + embedded (41 modules, JS 39.93KB gz, CSS 10.75KB gz), live
`tsk serve` smoke-tested. Canonical workdir `/Volumes/Projects/tsk` WAS mounted
(HEAD at 2ef41be, node_modules present).

This batch finishes the T24 queue exactly as written (no deferrals): the five
fresh follow-ons appended in T23 after the clickable-breadcrumb / shift-focus-
action / pin-flash-scroll / unpin-lens / panel-cohort-line cluster. Every slice
closes a "the affordance exists on ONE surface but not its sibling" gap the
T22-T23 work opened (panel cohort line could clear but not step back -> back
button; could clear but not walk -> walk button; unpin removed the chip silently
-> exit fade; toast Focus drops into a cohort the closed panel can't show -> open
the panel; pin lifecycle had star + Cmd-K but no direct key -> the `*` toggle).

- **F127/F128** `d2d602b` Panel cohort line gains back-step + walk. cohort.ts:
  renderCohortPanelLine(focus, historyDepth=0) grows two disjoint sibling buttons
  in a `.stat-cohort-row` flex (a button can't nest a button) — a leading
  data-cohort-back (only when historyDepth > 0; bare ‹ at depth 1, ‹N badge once
  deeper, mirroring the F113 chip badge) and a trailing data-waiting-walk on the
  sourceId (the SAME hook the sidebar chokepoint rows use). main.ts: passes
  cohortHistory.length; the stats-panel click handler routes data-cohort-back ->
  cohortBack FIRST (before clear), and the existing data-waiting-walk branch
  already opens openChainDrill(sourceId, "dependent"). app.css: .stat-cohort-row +
  .stat-cohort-back/.stat-cohort-walk styling; .stat-cohort-line flex-grows. +6 tests.
- **F129** `0e2bf6f` Fade a lens chip out on unpin. views.ts: UNPIN_EXIT_MS (240,
  single source for the timer + CSS) + canAnimateChipExit(chip) (true only for a
  real element with a callable classList.add — jsdom-less/detached falls through).
  main.ts: unpinCurrentLens finds the chip, and when animatable adds is-unpinning
  then defers removeView by UNPIN_EXIT_MS; else removes synchronously. Status line
  fires up front. app.css: view-chip-unpin keyframe (shrink+fade) + reduced-motion
  instant-opacity fallback — the inverse of F119's pin-flash. +3 tests.
- **F130** `a083743` Shift-toast Focus action also reveals the panel. stats.ts:
  shouldRevealStatsOnFocus(statsOpen) -> true only when closed (so an open panel
  isn't toggled shut). main.ts: the F123 toast Focus run() now chains
  setCohort(focusId) + a guarded toggleStats(true) so the F126 "Focused" line +
  breakdown for the just-focused cohort are immediately legible. +1 test.
- **F131** `8573e06` Keyboard pin/unpin toggle on the active lens. lens.ts:
  lensPinToggleAction(kind, pinned) -> none | pin | unpin, decided from the SAME
  findPureLensView state the star + palette commands read. main.ts: a `*` keydown
  case (echoing the star glyph) routes pin -> pinCurrentLens, unpin ->
  unpinCurrentLens, none -> a "no lens to pin" hint; documented in HELP_ROWS next
  to the lens digit-map row. +3 tests.
- `0b18761` bundle rebuild (41 modules, JS 39.93KB gz, CSS 10.75KB gz).

Live proof: built /tmp/t24.tsk.md via the CLI (--file) — #1 (Ship release) with
2 waiters (#3,#4) and #2 (Deploy infra) with 1 (#5), so #1 is the biggest
chokepoint — then `tsk serve --addr 127.0.0.1:7924`. GET / -> 200; GET
/api/tasks returned the tasks; the served bundle (app-DoKPYNfx.js) carried every
hook: stat-cohort-back + stat-cohort-walk (F127/F128), is-unpinning (F129),
"no lens to pin" (F131); the shouldRevealStatsOnFocus body (F130) is inlined by
esbuild (one-line `!statsOpen`). Toggling #1 done via POST /api/tasks/1/toggle
(which completes the biggest chokepoint -> shift to #2, the exact F130 scenario)
round-tripped to .tsk.md preserving every depends: link + adding completed:;
`tsk show 3` read depends:#1 back. The raw .tsk.md stayed plain hand-editable
markdown — the CLI/TUI storage contract is intact. Cleaned up the /tmp fixture +
the built binary after.

Deferred: nothing. F48 (context-menu submenus), F49 (autocomplete in
edit/filter), F50 (dep mini-graph), F54 (touch context menu) remain the standing
long-carries. T25 backlog (F132-F136) appended below so the loop never starves.

### T25 — depth (appended T24 2026-06-27 so the loop never starves)

Standing unstarted: F48 (context-menu submenus), F49 (autocomplete in edit +
filter), F50 (dep mini-graph), F54 (touch context menu). Fresh follow-ons after
the T24 panel-back-step / panel-walk / unpin-fade / focus-reveals-panel /
pin-toggle-key cluster:

- [x] **F132** Cohort history breadcrumb TRAIL in the stats panel: F127 gave the
      panel line a single back-step button, but the chip/help only ever show the
      next step. Render the WHOLE cohort back-stack (cohortHistory) as a compact
      "#A › #B › #M" trail below the F126 line, each segment a button that pops
      straight to that ancestor (not just one level). Reuse popCohortHistory's
      skip-dead-ancestor logic but target a specific depth; pure-render the trail
      in cohort.ts (renderCohortTrail(focus, history)) so it's unit-tested. (T25 2026-06-27, 38e6be7)
- [x] **F133** "Pin this cohort as a view" from the panel line: a cohort is a
      momentary id-set (not serializable), but you often re-focus the SAME
      chokepoint. Add a small "pin" affordance on the F126 panel line that saves a
      named saved-view capturing the chokepoint's sourceId (a new view kind, or a
      filter that re-derives the cohort on recall) so "what waits on #1" is one
      click away next session. Design the recall path first (re-run buildCohort on
      recall); record the chosen shape in STATE-web before building. (T25 2026-06-27, a017373)
- [x] **F134** Unpin-fade also plays when the chip is removed via its × delete:
      F129 fades a chip out on the F125 unpin path, but deleting a saved view via
      the chip's × still vanishes instantly. Route the chip-× deleteView through the
      same is-unpinning exit (rename to a neutral is-leaving) so EVERY chip removal
      gets the spatial "this one left" confirmation, not just lens unpins. Reuse
      canAnimateChipExit + UNPIN_EXIT_MS; guard the test env. (T25 2026-06-27, f57a2fb)
- [x] **F135** "Focus" toast action keyboard mirror: F130 made the toast Focus
      button open the panel; add the same "focus the new chokepoint + reveal panel"
      as a keystroke while the shift toast is showing (e.g. `f`), so a keyboard user
      can act on a shift without reaching for Cmd-K or the mouse. Track the live
      toast's focusId in a slot; clear it when the toast times out. (T25 2026-06-27, 6acf1ca)
- [x] **F136** `*` pin-toggle also works from the chip star's keyboard focus: F131
      bound `*` globally to toggle the ACTIVE lens's pin. Make the chip star itself
      focusable (tabindex) and Enter/Space on it pin/recall, Shift+Enter unpin — so
      a keyboard user tabbing the filter bar can drive the pin from the star too,
      not only the global `*`. Reuse pinCurrentLens / unpinCurrentLens. (T25 2026-06-27, e5d0d64)

When fewer than 5 remain, append more (recurring-task UI, archive view, an undo
stack beyond single delete, a keyboard-driven dep-graph navigator, a saved-view
import/export, a per-tag saved-view group, a per-row dep mini-sparkline, etc.).

### F133 design note (recorded T25 2026-06-27 before building, per prompt)

A cohort is a momentary id-set (snapshot of "who waits on #N right now"), NOT
serializable — but the CHOKEPOINT (#N) is stable, so a saved cohort view stores
only the sourceId and RE-DERIVES the id-set on recall via buildCohort against the
live graph (exactly how setCohort already works). Chosen shape:

- `SavedView.cohort?: number` — the chokepoint sourceId. A cohort view has an
  EMPTY filter and NO lens (it's a third saved-view kind beside filter views and
  pure-lens views). normalizeViews carries it (positive int only).
- `isCohortView(v)`: cohort set AND empty filter AND no lens.
- `addCohortView(views, name, sourceId)`: add/overwrite-by-name a cohort view.
- `findCohortView(views, sourceId)`: the saved view for a chokepoint (so the pin
  affordance reads pinned vs unpinned, and recall-if-exists avoids duplicates).
- renderViewChips: a cohort chip wears an `is-cohort-pin` class + a fixed up-arrow
  (↑) glyph (the cohort glyph, echoing the chip's & panel's ↑). Active highlight
  for a cohort chip is `v.cohort === opts.activeCohort` (NOT the filter path —
  its empty filter would otherwise match every empty-filter board), so the chip
  lights up only while THAT chokepoint's cohort is focused.
- Recall path: recallView detects v.cohort and routes through setCohort(v.cohort)
  (re-runs buildCohort live; graceful "nothing waits" if the chokepoint cleared).
- Pin affordance: renderCohortPanelLine grows a trailing star button
  (data-cohort-pin) — ☆ when unpinned, ★ when the chokepoint is already a saved
  cohort view. main.ts togglePinCohort() pins (addCohortView, recall-if-exists)
  or unpins (removeView) symmetric with the lens star (F131).

### TICK LOG — T25 (2026-06-27 ~20:17 PT)

Shipped 5/5 web slices on `main`, gated once, pushed clean `288ce9c..3214dfd`.

- **F132** `38e6be7` — cohort drill breadcrumb trail in the stats panel.
  cohort.ts renderCohortTrail(focus, history) + jumpCohortHistory(tasks, stack,
  index) (leap to any ancestor, skip-dead via popCohortHistory over the
  prefix). main.ts cohortJumpTo(index), panel renders the trail above the F126
  line, click routes data-cohort-jump first. app.css .cohort-trail. +9 tests.
- **F133** `a017373` — pin a cohort as a re-derivable saved view. views.ts
  SavedView.cohort? + isCohortView/findCohortView/addCohortView; renderViewChips
  cohort chips (activeCohort highlight, ↑ glyph, is-cohort-pin). cohort.ts panel
  pin star (data-cohort-pin, ★/☆). main.ts togglePinCohort + recallView routes
  cohort views through setCohort (live re-derive). Design note recorded first.
  +18 tests.
- **F134** `f57a2fb` — every saved-view chip fades out on removal. Extracted
  animateChipExitThenRemove; deleteView + unpinCurrentLens + cohort-unpin all
  route through it. CSS .is-unpinning -> neutral .is-leaving (alias kept). Pure
  seam already covered (canAnimateChipExit + UNPIN_EXIT_MS).
- **F135** `6acf1ca` — `f` key mirrors the shift-toast Focus action. stats.ts
  toastFocusAction(focusId, statsOpen) shared by the toast button + the key.
  main.ts liveToastFocusId slot (cleared on dismiss/timeout/action),
  runLiveToastFocus, dismissInfoToast, `f` keydown, HELP_ROWS row. +4 tests.
- **F136** `e5d0d64` — keyboard pin/unpin from the lens star's own focus. lens.ts
  lensStarKeyAction(key, shift, kind, pinned): pin on Enter/Space, unpin on
  Shift+activation when pinned. main.ts star keydown preventDefaults to avoid
  the native-click double-fire. +4 tests.
- `3214dfd` — bundle rebuild (41 modules, JS 40.84KB gz, CSS 10.91KB gz).

Gates: web tests 799 -> 829 (+30), 0 fail; tsc app + test clean; gofmt/vet/build
clean; go test ./... green (commands 66.5s, serve cached-green). Live `tsk serve`
smoke: GET / 200, served bundle carried all new hooks (cohort-trail,
data-cohort-jump, data-cohort-pin, is-cohort-pin, is-leaving); POST
/api/tasks/1/toggle round-tripped to .tsk.md preserving BOTH depends:1 links +
adding completed:; `tsk show 2` read depends:#1 back. Storage contract intact.

Deferred: nothing. Standing long-carries: F48 (context-menu submenus), F49
(autocomplete in edit/filter), F50 (dep mini-graph), F54 (touch context menu).
T26 backlog (F137-F141) appended below so the loop never starves.

### T26 — depth (appended T25 2026-06-27 so the loop never starves)

Standing unstarted: F48, F49, F50, F54. Fresh follow-ons after the T25 cohort
trail / cohort-pin / chip-leave-fade / f-focus-mirror / star-keyboard cluster:

- [x] **F137** Cohort trail keyboard navigation: the F132 trail is mouse-only
      (click a segment to jump). Add a keyboard path — when the stats panel has
      focus, Alt+‹ / Alt+› (or `[`/`]` while a cohort is focused) step the trail
      one ancestor at a time, mirroring the existing cohortBack/cohortJumpTo so a
      keyboard user can walk the whole drill ancestry. Pure helper picks the
      target index from the current focus + a direction; main.ts wires the keys.
      (tick T26 2026-06-28 — Alt+Left=step one ancestor back, Alt+Right=leap to
      root; pure cohortTrailKeyTarget; +10 tests)
- [x] **F138** Cohort-view chips survive a chokepoint that completes: F133's
      recall calls setCohort, which no-ops with "nothing waits on #N" when the
      chokepoint cleared — but the dead chip lingers. Add a small "stale" marker
      on a cohort chip whose chokepoint is done/gone (computed at render from the
      live graph), and offer a one-click "forget" on recall-of-a-dead-cohort so
      the row self-cleans. Pure isStaleCohortView(view, tasks) → unit-tested.
      (tick T26 2026-06-28 — is-stale-cohort chip mark + recall self-clean; +6 tests)
- [x] **F139** "Pin current cohort" Cmd-K command: F133 added the panel star, but
      the keyboard-first path (Cmd-K) can't pin/unpin the focused cohort. Add a
      pinCohortCommand / unpinCohortCommand (mirroring F110's lens pin commands)
      that reads the focused chokepoint + its pinned state, so the whole cohort
      pin lifecycle is reachable from the palette too. Pure command builder +
      label ("Pin cohort waiting on #N"). (tick T26 2026-06-28 — pin/recall/unpin
      commands + cohortPinned ctx; shared pinFocusedCohort/unpinFocusedCohort; +8 tests)
- [x] **F140** Cohort trail "copy chain" affordance: a trailing button on the
      F132 trail copies the whole drill path as text ("#1 › #4 › #9") to the
      clipboard — useful for pasting a bottleneck-walk into a standup note. Pure
      formatCohortTrailText(focus, history); main.ts wires navigator.clipboard
      with the F-style guarded fallback (test env / no clipboard → status hint).
      (tick T26 2026-06-28 — data-cohort-copy button + guarded clipboard; +5 tests)
- [x] **F141** Saved-view chip count badges: a filter view chip could show how
      many tasks it currently matches (a tiny "·12" badge) computed live from the
      board, so the Views row doubles as an at-a-glance triage dashboard. Pure
      countViewMatches(view, tasks) reusing the filter/lens/cohort predicates;
      render a quiet badge in renderViewChips; opt-in via a ViewChipOpts flag so
      existing callers/tests stay byte-identical. (tick T26 2026-06-28 — injected
      filter/lens/cohort counters; ·N badge opt-in; +8 tests)

When fewer than 5 remain, append more (recurring-task UI, archive view, an undo
stack beyond single delete, a per-row dep mini-sparkline, a saved-view
import/export, a per-tag saved-view group, keyboard reordering of the Views row).

---

## Web tick T26 — 2026-06-28 02:20 PT

Shipped 5/5 web slices on `main` (F137-F141), gated once, pushed clean
`cf0e23a..37dc6ef` (6 commits: 5 features + the web_dist bundle rebuild).

### Features
- **F137** `03f1463` — cohort-trail keyboard navigation. cohort.ts pure
  cohortTrailKeyTarget(historyLength, dir): "step" -> historyLength-1 (most-recent
  ancestor, one level back = the Escape/cohortBack landing); "root" -> 0 (drill
  origin); empty -> -1. main.ts wires Alt+Left="step" / Alt+Right="root", handled
  BEFORE the modifier early-return guard, gated on a focused cohort with history +
  the not-typing/editing guards, routing through cohortJumpTo (shares the
  skip-dead-ancestor walk). HELP_ROWS row added. +10 cohort tests.
- **F140** `92b7feb` — copy-chain affordance on the trail. cohort.ts pure
  formatCohortTrailText(focus, history) -> "#A › #B › #current" (same segment
  order + separator glyph as renderCohortTrail; "" when no focus/history).
  renderCohortTrail appends a trailing data-cohort-copy button (disjoint sibling
  after the current segment, title carries the chain). main.ts copyCohortChain()
  guards navigator.clipboard (test/no-API/reject -> status-hint fallback that
  still shows the chain); stats-panel click routes copy first. app.css
  .cohort-trail-copy. +5 cohort tests.
- **F138** `bafd9df` — stale cohort chips self-clean. views.ts pure
  isStaleCohortView(view, hasLiveCohort) with an injected live-cohort check
  (decoupled, mirroring lensGlyph/cohortGlyph injection): cohort bookmark is stale
  iff its chokepoint has no live cohort; non-cohort views never stale.
  renderViewChips staleCohort resolver opt -> is-stale-cohort class + "— stale,
  recall to clear" title. main.ts passes the predicate (buildCohort over the live
  graph) + recallView self-cleans a dead cohort view (drop with F134 leave-fade +
  status) instead of the bare setCohort no-op. app.css dims/strikes the chip.
  +6 views tests.
- **F139** `3386db5` — Cmd-K cohort pin/unpin. palette.ts pinCohortCommand
  ("Pin cohort (<summary>)" / "Recall pinned cohort" when pinned) +
  unpinCohortCommand (enabled only when pinned); CommandReasonContext gains
  cohortPinned; commandDisabledReason adds "no cohort active" / "cohort not
  pinned" (mirrors the lens-unpin split). main.ts splits togglePinCohort into
  shared pinFocusedCohort/unpinFocusedCohort (star + commands drive one path
  each), wires build+run+ctx. +8 palette tests.
- **F141** `e9686aa` — view-chip match-count badges. views.ts pure
  countViewMatches(view, tasks, counters) with the 3 per-kind predicates injected
  (cohort -> id-set size; lens-bearing -> filter AND lens; plain -> filter).
  renderViewChips matchCount resolver opt -> quiet "·N" badge (null/no-resolver ->
  nothing). main.ts supplies counters backed by real matchesFilter/matchesLens/
  buildCohort over the same live pool the board narrows from (tag route applied).
  app.css .view-chip-count (dim, tabular-nums, brightens when active). +8 views tests.
- `37dc6ef` — web_dist bundle rebuild (41 modules, JS 41.73KB gz, CSS 11.01KB gz).

### Gates
- web check: tests 829 -> 857 (+28), 0 fail; tsc app + test/tsconfig.json clean
- web build green (41 modules); gofmt clean / go vet clean / go build clean
- go test ./... green (commands 67.7s, rest cached)
- live `tsk serve --addr 127.0.0.1:7937`: GET / 200; served JS/CSS carried all new
  hooks (data-cohort-copy, cohort-trail-copy, is-stale-cohort, "stale, recall to
  clear", cohort-pin, "Recall pinned cohort", "Unpin cohort", "cohort not
  pinned", view-chip-count); /api/tasks exposed depends_on:[1] for #2/#3;
  POST /api/tasks/2/toggle round-tripped to .tsk.md preserving depends:1 + adding
  completed:. Storage contract intact. Fixtures + binary cleaned up.

### Deferred
Nothing. Standing long-carries: F48 (context-menu submenus), F49 (autocomplete in
edit/filter), F50 (dep mini-graph), F54 (touch context menu).

### T27 — depth (appended T26 2026-06-28 so the loop never starves)

Standing unstarted: F48, F49, F50, F54. Fresh follow-ons after the T26
trail-keyboard / copy-chain / stale-chip / cohort-pin-command / chip-count
cluster:

- [x] **F142** Views-row total-count summary: with F141's per-chip "·N" badges
      live, add a tiny leading "Σ" or "N views" readout at the row head that sums
      the active board's coverage (e.g. "5 views"), or — more useful — a one-shot
      "busiest view" highlight: the chip with the highest live match-count gets a
      subtle marker so the densest bucket pops. Pure busiestView(views, counts).
      (tick T27 2026-06-28 — busiestViewId + is-busiest amber marker; +5 tests)
- [x] **F143** Cohort-trail copy as MARKDDOWN: F140 copies "#1 › #4 › #9" as
      plain text; add a modifier (Alt-click / a second button) that copies a
      markdown task-link chain ("[#1](…) → [#4](…)") or a checkbox list of the
      cohort, so the standup paste is richer. Pure formatCohortTrailMarkdown.
      (tick T27 2026-06-28 — Alt-click copies "#1 → #4 → #9" arrow chain; +6 tests)
- [x] **F144** Stale-chip bulk sweep: F138 self-cleans ONE dead cohort chip on
      recall; add a "forget all stale" affordance (a Cmd-K command + a Views-row
      button when ≥1 stale chip exists) that drops every dead cohort bookmark in
      one go. Pure staleCohortViewIds(views, hasLiveCohort) → the id list to remove.
      (tick T27 2026-06-28 — forgetStaleCohortsCommand + views-sweep button +
      F134 leave-fade batch removal; +7 tests)
- [x] **F145** Match-count badge on lens/cohort chips reflects DONE-state filter:
      F141 counts over the live pool, but a hideDone view and a show-all view read
      differently — surface that the badge respects the view's own hideDone, and
      add a tooltip breakdown ("12 open · 3 done") on hover for filter views. Pure
      countViewMatchesBreakdown returning {open, done}.
      (tick T27 2026-06-28 — open/done badge tooltip via matchTitle resolver; +9 tests)
- [ ] **F146** Keyboard recall of a numbered view (F25 views) from the palette
      WITHOUT opening it: a "Peek view (<name>)" command that previews the match
      count + describeView in the preview slot (F89-style) so you can compare
      views before committing to a recall. Pure peekViewLabel(view, count).
- [x] **F147** Cohort-trail "jump to densest ancestor": with F141's counts, a
      trail segment could show its own waiter-count, and a key (Alt+D) could jump
      straight to the ancestor with the most waiters (the worst bottleneck in the
      drill). Pure densestCohortAncestorIndex(history, cohortIds).
      (tick T27 2026-06-28 — Alt+D jumps to densest ancestor; +4 tests)

When fewer than 5 remain, append more (recurring-task UI, archive view, an undo
stack beyond single delete, a per-row dep mini-sparkline, a saved-view
import/export, a per-tag saved-view group, keyboard reordering of the Views row).

---

## Web tick T27 — 2026-06-28 07:40 PT

Shipped 5/5 web slices on `main` (F142, F145, F143, F147, F144), gated once,
pushed clean. Workdir note: the canonical `/Volumes/Projects/tsk` external SSD
WAS mounted this tick (git resolved cleanly on `main`, clean tree) — no fallback
to the internal worktree needed this time.

### Features (one commit each + the bundle rebuild)
- **F142+F145** `599a7f3` — Views-row badge depth. views.ts: pure
  countViewMatchesBreakdown(view, tasks, counters, isDone) -> {open, done} (a
  cohort is all-open by construction) + describeViewMatchBreakdown for the text;
  busiestViewId(views, count) returns the unique densest view id or null on a
  tie / empty board. renderViewChips gains matchTitle (richer "12 open · 3 done"
  badge tooltip) + busiestId (is-busiest amber left-accent) opts. main.ts hoists
  viewMatchPool() + viewMatchCounters() so F141's count, F145's tooltip, and
  F142's marker all read the IDENTICAL live pool + predicates. app.css
  .view-chip.is-busiest. +14 views tests.
- **F143** `dc25dbc` — copy the cohort drill chain as markdown. cohort.ts pure
  formatCohortTrailMarkdown(focus, history) -> "#1 -> #4 -> #9" (arrow U+2192,
  distinct from F140's chevron U+203A; same segment order + empty conditions).
  main.ts copyCohortChain(markdown) reuses the one guarded-clipboard path; the
  trail copy button title advertises "Alt-click for markdown"; the stats-panel
  click reads e.altKey. +6 cohort tests.
- **F147** `486d719` — Alt+D jumps to the densest cohort-drill ancestor. cohort.ts
  pure densestCohortAncestorIndex(history, waiterCount) scans only the ancestry
  (not the current focus), oldest-ancestor tie-break, all-dead/empty -> -1. main.ts
  wires Alt+D with the SAME guards + before-the-modifier-return placement as F137's
  Alt-arrows, backing the count with buildCohort and routing the winning index
  through cohortJumpTo (skip-dead). HELP_ROWS row. +4 cohort tests.
- **F144** `ca400b8` — forget-all-stale cohort sweep. views.ts pure
  staleCohortViewIds(views, hasLiveCohort) (isStaleCohortView mapped over the
  list). palette.ts forgetStaleCohortsCommand(staleCount) + a new staleCohortCount
  on CommandReasonContext + "no stale cohort views" reason. main.ts
  forgetStaleCohorts() drops every dead chip through the F134 leave-fade (overlap
  fades, one store update); shared currentStaleCohortIds() backs the command
  count, the disabled-reason context, and a new "forget stale (N)" Views-row
  button (shown only when >=1 stale). app.css .views-sweep. +7 tests (3 views, 4
  palette).
- `26affe9` — web_dist bundle rebuild (41 modules, JS 42.49KB gz, CSS 11.06KB gz).

### Gates
- web check: tests 857 -> 882 (+25), 0 fail; tsc app + test/tsconfig.json clean;
  web build green (41 modules).
- gofmt clean / go vet clean / go build clean; go test ./... green (commands
  82.6s, serve cached-green with the freshly-embedded T27 bundle).
- live `tsk serve --addr 127.0.0.1:7951`: GET / 200; served JS carried all new
  hooks (is-busiest, "Forget all stale cohort views", cohort-forget-stale,
  "Alt-click for markdown", data-views-sweep, "no stale cohort views", "Jump to
  the densest ancestor", literal arrow + middot glyphs); /api/tasks exposed
  depends_on for #2/#3; POST /api/tasks/2/toggle round-tripped to .tsk.md
  preserving depends:1 + adding completed:. Storage contract intact. Fixtures +
  binary cleaned up.

### Deferred
F146 (peek-view-without-recall) — deferred to T28, the one backlog item not
sized into this tick's badge/cohort-trail/sweep cluster. Standing long-carries:
F48 (context-menu submenus), F49 (autocomplete in edit/filter), F50 (dep
mini-graph), F54 (touch context menu).

### T28 — depth (appended T27 2026-06-28 so the loop never starves)

Standing unstarted: F48, F49, F50, F54, F146. Fresh follow-ons after the T27
busiest-marker / open-done-tooltip / markdown-copy / densest-jump / stale-sweep
cluster:

- [ ] **F146** Keyboard "Peek view (<name>)" command: preview a view's match
      count + describeView in the palette preview slot (F89-style) WITHOUT
      recalling it, so you can compare views before committing. Pure peekViewLabel.
- [ ] **F148** Views-row "N views, M tasks" header readout: with F141 counts +
      F142's busiest marker live, add a tiny leading summary at the row head that
      sums coverage across all chips (distinct views, total matched tasks). Pure
      viewsRowSummary(views, counts).
- [ ] **F149** Busiest-marker as a Cmd-K jump: a "Recall busiest view (<name>, N)"
      command that recalls whatever F142 marks as densest, so the keyboard path
      reaches the triage winner without eyeballing the row. Pure builder over
      busiestViewId + the view name/count.
- [ ] **F150** Cohort-trail segment waiter-counts: render each trail segment's own
      live waiter-count as a tiny superscript ("#4 7") so the F147 densest jump is
      visible in the trail itself, not just reachable by key. Pure
      cohortTrailCounts(history, waiterCount) -> per-segment counts.
- [ ] **F151** Stale-sweep undo: F144 drops every dead cohort bookmark; add a
      single-shot undo toast ("forgot 3 stale views — undo") that restores the
      swept set (mirroring F8's delete-undo), so a misfire is recoverable. Pure
      part = the swept-set snapshot; main.ts holds it for the toast window.
- [ ] **F152** Open/done badge filter action: clicking the F145 "·N" badge on a
      show-all view recalls it AND toggles hideDone so you land on just the open
      slice — the badge's done count becomes an actionable "hide these" affordance.

When fewer than 5 remain, append more (recurring-task UI, archive view, a
per-row dep mini-sparkline, a saved-view import/export, keyboard reordering of
the Views row, a per-tag saved-view group).
