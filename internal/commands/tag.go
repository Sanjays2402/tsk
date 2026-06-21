package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// newTagCmd implements `tsk tag <id> +foo -bar`: a single-task tag mutator
// that adds and/or removes tags in one shot.
//
// Each positional arg after the id is a tag op:
//
//	+name   -> add this tag (idempotent)
//	-name   -> remove this tag if present
//	name    -> shorthand for +name
//
// Args are processed in order, so `tsk tag 3 +work -personal +urgent`
// adds work, removes personal, and adds urgent in a single Save.
//
// Tags are normalized (lowercased, trimmed, deduped) consistent with the
// rest of tsk so `+Work` and `+WORK` are equivalent.
func newTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "tag <id> <+name|-name> [...]",
		Short:              "Add or remove tags on a single task",
		DisableFlagParsing: true,
		Long: `Add or remove tags on a single task in one shot.

Each arg after the id is a tag op:
  +name    add this tag (idempotent — already-present is a no-op)
  -name    remove this tag if present
  name     shorthand for +name

Tags are case-insensitive (normalized to lowercase) and deduped.

Examples:
  tsk tag 3 +work
  tsk tag 3 +work -personal +urgent
  tsk tag 12 doc          # shorthand: same as +doc
  tsk tag 7 -old +legacy  # swap a tag
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// DisableFlagParsing is on so leading -foo tokens land as args
			// instead of unknown flags. Re-extract the persistent --file
			// flag from the raw args ourselves before treating the rest as
			// tag ops.
			cleanArgs, fileVal, err := extractFileFlag(args)
			if err != nil {
				return err
			}
			if fileVal != "" {
				if err := cmd.Flags().Set("file", fileVal); err != nil {
					return err
				}
			}
			if len(cleanArgs) == 0 {
				return usageErrorf("tag requires <id> and at least one tag op")
			}
			if len(cleanArgs) < 2 {
				return usageErrorf("tag requires at least one tag op (e.g. +work)")
			}
			id, err := parseSingleID(cleanArgs[0])
			if err != nil {
				return err
			}
			adds, removes, err := parseTagOps(cleanArgs[1:])
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			t := s.ByID(id)
			if t == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
			before := append([]string(nil), t.Tags...)
			if len(adds) > 0 {
				t.Tags = addUniqueTags(t.Tags, adds)
			}
			if len(removes) > 0 {
				t.Tags = removeTagsFrom(t.Tags, removes)
			}
			t.NormalizeTags()
			if tagSetEqual(before, t.Tags) {
				pf(cmd.OutOrStdout(), "#%d tags unchanged\n", id)
				return nil
			}
			if err := s.Save(); err != nil {
				return err
			}
			pf(cmd.OutOrStdout(), "#%d tags %s -> %s\n", id,
				renderTagList(before), renderTagList(t.Tags))
			return nil
		},
	}
	return cmd
}

// extractFileFlag pulls a `--file <path>` (or `--file=<path>`) pair out of a
// raw arg slice and returns the remainder. Used by commands with
// DisableFlagParsing=true to keep the persistent --file flag working even
// when their positional args clash with cobra's flag parser.
func extractFileFlag(args []string) (rest []string, file string, err error) {
	rest = make([]string, 0, len(args))
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--file":
			if i+1 >= len(args) {
				return nil, "", usageErrorf("--file requires a value")
			}
			file = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--file="):
			file = strings.TrimPrefix(a, "--file=")
			i++
		default:
			rest = append(rest, a)
			i++
		}
	}
	return rest, file, nil
}

// parseTagOps splits the trailing args into add/remove slices.
// A leading "+" or "-" picks the op; a bare name is treated as +name.
// Returns a usage-coded error on empty names or duplicate +/- on the same tag.
func parseTagOps(ops []string) (adds, removes []string, err error) {
	addSet := make(map[string]bool)
	rmSet := make(map[string]bool)
	for _, op := range ops {
		op = strings.TrimSpace(op)
		if op == "" {
			return nil, nil, usageErrorf("empty tag op")
		}
		var sign byte = '+'
		name := op
		switch op[0] {
		case '+', '-':
			sign = op[0]
			name = strings.TrimSpace(op[1:])
		}
		if name == "" {
			return nil, nil, usageErrorf("tag op %q is missing a name", op)
		}
		key := strings.ToLower(name)
		if sign == '+' {
			if rmSet[key] {
				return nil, nil, usageErrorf("conflicting +%s and -%s", name, name)
			}
			if !addSet[key] {
				addSet[key] = true
				adds = append(adds, key)
			}
		} else {
			if addSet[key] {
				return nil, nil, usageErrorf("conflicting +%s and -%s", name, name)
			}
			if !rmSet[key] {
				rmSet[key] = true
				removes = append(removes, key)
			}
		}
	}
	return adds, removes, nil
}

// tagSetEqual checks if two tag slices contain the same set (order-insensitive).
func tagSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as := append([]string(nil), a...)
	bs := append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if !strings.EqualFold(as[i], bs[i]) {
			return false
		}
	}
	return true
}

// renderTagList formats a tag slice as "#a #b" or "-" when empty.
func renderTagList(tags []string) string {
	if len(tags) == 0 {
		return "-"
	}
	return "#" + strings.Join(tags, " #")
}
