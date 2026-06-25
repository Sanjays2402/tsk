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
- [ ] **F59** Stats: a "due this week" + "no-due" count tile, and make the
      blocked tile click-to-filter (show only blocked tasks) reusing the F11
      filter plumbing.
- [ ] **F60** Bulk blocked-guard: when a bulk multi-toggle would complete one or
      more BLOCKED tasks, list them in a single confirm ("3 of 5 are blocked —
      complete all anyway?") instead of silently completing — the bulk sibling
      of F45.

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
- [ ] **F62** Unblock-toast depth: when several tasks unblock at once (F42 plural
      case), make the toast action a tiny picker ("3 unblocked — jump to which?")
      instead of always jumping to the first.
- [ ] **F63** Priority keyboard parity in the palette: a "Set priority ▸" command
      group in Cmd-K (urgent/high/medium/low) acting on the selected task, reusing
      setPriority — keyboard-only priority without leaving the palette.
- [ ] **F64** Stats "blocked" tile click-to-filter (the F59 idea, split out):
      clicking the F46 Blocked count filters the list to only blocked tasks via
      the F11 filter plumbing; a clear chip resets it.
- [ ] **F65** Notes-snippet highlight in the chain drill + dep badge titles, so a
      search that matched on a blocker's text is visible when you walk the chain.
