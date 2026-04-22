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
- 🟠 Amber / gold palette with adaptive dark/light terminal support.

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
brew install Sanjays2402/tap/tsk   # coming soon
```

## Usage

```
tsk init                                   # create .tsk.md in cwd
tsk add "Buy milk" -p high -d tomorrow -t errand -t home
tsk ls                                     # undone by default
tsk ls --all --tag errand
tsk done 1
tsk next                                   # highest-priority undone
tsk stats
tsk export --json > tasks.json
tsk                                        # launch the TUI
```

### Dates

`-d/--due` (and the TUI `D` key) accept natural language as well as `YYYY-MM-DD`:

- `today`, `tomorrow`, `tmrw`, `yesterday`
- Weekdays: `mon`..`sun` / `monday`..`sunday` — next occurrence
- Relative: `3d`, `2w`, `1m`, `in 3d`, `in 2 weeks`
- Months: `jul 4`, `july 4 2027`, `4 jul`, `dec`
- Aliases: `next week`, `next month`, `next mon`, `eow`, `eom`

All dates resolve in `America/Los_Angeles`. Unknown inputs exit with code 2 and a hint.

### TUI keys

| Key       | Action                      |
|-----------|-----------------------------|
| `j` / `k` | Move selection              |
| `␣` / `⏎` | Toggle done                 |
| `a`       | Add task (inline form)      |
| `e`       | Edit title                  |
| `t`       | Edit tags                   |
| `D`       | Set due date (natural lang) |
| `p`       | Cycle priority              |
| `d`       | Delete (`y` to confirm)     |
| `/`       | Fuzzy filter                |
| `s`       | Sort: priority / due / created / id |
| `tab`     | Collapse current section    |
| `?`       | Help overlay                |
| `q`       | Quit                        |

### Storage format

`.tsk.md` is a GitHub-flavored task list with an HTML comment carrying metadata:

```
- [ ] Buy milk <!-- id:1 prio:medium due:2026-04-25 tags:errand,home created:2026-04-21T19:20:00-07:00 -->
      Notes text here, indented 6 spaces if present.
- [x] Pay rent <!-- id:2 prio:high completed:2026-04-20T09:00:00-07:00 -->
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
