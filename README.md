# tsk

Fast, keyboard-first markdown todo manager. One TUI, one CLI, one plain `.tsk.md` file you can still edit by hand.

Inspired by [HxX2/todocli](https://github.com/HxX2/todocli).

## Features

- TUI with sectioned sections (Overdue / Today / Upcoming / No Due / Done), progress bar, and fuzzy search.
- CLI for scripting: `add`, `ls`, `done`, `undo`, `rm`, `edit`, `next`, `stats`, `export`.
- Markdown storage. Readable in any editor, diffable in git, salvageable if a process crashes mid-write.
- Walks up from `$PWD` to find the nearest `.tsk.md`, like a `.git` directory. Falls back to `~/.tsk/global.md`.
- Respects `NO_COLOR`.
- Shell completions for bash, zsh, fish, and powershell.
- Amber / gold palette with adaptive dark/light terminal support.

## Install

### Go

```
go install github.com/Sanjays2402/tsk/cmd/tsk@latest
```

### Prebuilt binary

Grab the archive for your OS/arch from [Releases](https://github.com/Sanjays2402/tsk/releases).

### Homebrew

A tap will live at `Sanjays2402/homebrew-tap` — not yet published.

```
brew install Sanjays2402/tap/tsk # coming soon
```

## Usage

```
tsk init # create .tsk.md in cwd
tsk add "Buy milk" -p high -d tomorrow -t errand -t home
tsk ls # undone by default
tsk ls --all --tag errand
tsk done 1
tsk next # highest-priority undone
tsk stats
tsk export --json > tasks.json
tsk export --format markdown # shareable markdown
tsk archive --older-than 30d # tidy old completed tasks
tsk # launch the TUI
```

### Archiving and purging

`tsk archive` moves completed tasks out of the active `.tsk.md` and into a
sibling `.tsk.archive.md` in the same directory. The archive file is plain
markdown, portable, and git-safe — commit it, share it, or grep it like
any other tsk file.

```
tsk archive                       # default: done & completed >30 days ago
tsk archive --older-than 7d       # tighter cutoff
tsk archive --all                 # archive every Done task
tsk archive --all --dry-run       # preview without writing
```

Archived tasks get fresh sequential IDs in the archive file (continuing
from the archive's existing max), so the archive stays self-consistent.
Active task IDs do **not** renumber after an archive run.

`tsk purge` hard-deletes tasks. Unlike `archive`, it never moves anything
elsewhere. It refuses to run without an explicit selection — you must pass
`--done`, `--id`, or both — to keep "oops, I meant to archive" off the
table.

```
tsk purge --done                  # all completed tasks
tsk purge --done --older-than 90d # only old completed tasks
tsk purge --id 17 --id 23         # specific tasks regardless of state
tsk purge --done --dry-run        # preview
```

Duration strings accepted by `--older-than`: `Nd` days, `Nw` weeks, `Nm`
months (~30d), `Ny` years (~365d). For sub-day windows pass a Go duration
like `24h`.

Writes are atomic (tempfile + fsync + rename). Both commands accept
`--dry-run` to preview changes; `purge` additionally requires an explicit
selection flag, refusing to delete-everything-by-accident.

### Dates

`-d/--due` (and the TUI `D` key) accept natural language as well as `YYYY-MM-DD`:

- `today`, `tomorrow`, `tmrw`, `yesterday`
- Weekdays: `mon`..`sun` / `monday`..`sunday` — next occurrence
- Relative: `3d`, `2w`, `1m`, `in 3d`, `in 2 weeks`
- Months: `jul 4`, `july 4 2027`, `4 jul`, `dec`
- Aliases: `next week`, `next month`, `next mon`, `eow`, `eom`

All dates resolve in the time zone returned by `$TSK_TZ`, then `$TZ`, then the
system local zone, with `America/Los_Angeles` as a last-resort fallback on
stripped containers. Unknown inputs exit with code 2 and a hint.

```
TSK_TZ=America/New_York tsk add "Call mom" -d tomorrow
```

### Recurring tasks

A task can carry a recurrence rule. When you mark a recurring task done, tsk
completes the current instance and spawns a new open task with a fresh due
date computed from the rule.

Syntax (case-insensitive):

- `daily`, `weekly`, `monthly`, `yearly`
- `weekdays` (Mon-Fri only — Friday + weekdays = next Monday)
- `every:Nd`, `every:Nw`, `every:Nm`, `every:Ny` (e.g. `every:3d`, `every:2w`)

Monthly rules clamp overflow: Jan 31 + 1 month is Feb 28 (or 29 in leap years).
Yearly rules clamp Feb 29 to Feb 28 in non-leap years.

```
tsk add "Pay rent" -d 2026-06-01 -r monthly
tsk add "Standup" -r weekdays
tsk set-recur 4 every:2w   # add or change a rule on an existing task
tsk set-recur 4 none       # clear the rule
tsk done 4                 # marks #4 done AND creates the next instance
```

The new instance is anchored on the just-completed task's `due` date (or
today if it had none) and always advances at least one full step.

### TUI keys

| Key | Action |
|-----------|-----------------------------|
| `j` / `k` | Move selection |
| `␣` / `⏎` | Toggle done |
| `a` | Add task (inline form) |
| `e` | Edit title |
| `t` | Edit tags |
| `D` | Set due date (natural lang) |
| `p` | Cycle priority |
| `d` | Delete (`y` to confirm) |
| `/` | Fuzzy filter |
| `s` | Sort: priority / due / created / id |
| `tab` | Collapse current section |
| `?` | Help overlay |
| `q` | Quit |

### Storage format

`.tsk.md` is a GitHub-flavored task list with an HTML comment carrying metadata:

```
- [ ] Buy milk <!-- id:1 prio:medium due:2026-04-25 tags:errand,home created:2026-04-21T19:20:00-07:00 -->
 Notes text here, indented 6 spaces if present.
- [x] Pay rent <!-- id:2 prio:high recur:monthly completed:2026-04-20T09:00:00-07:00 -->
```

Rules:

- Unknown metadata keys are preserved on read, ignored on semantics.
- Priority values: `low`, `medium`, `high`, `urgent`.
- Due dates are `YYYY-MM-DD`.
- Created / completed timestamps are RFC 3339.
- IDs are auto-assigned; edit them by hand at your own risk.
- Writes are atomic: tempfile + fsync + rename.

## Screenshots

_TODO: add screenshots and a short GIF in `assets/`._

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

MIT. See [LICENSE](./LICENSE).
