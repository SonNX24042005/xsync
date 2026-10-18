package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Check every parent component so a renamed file cannot escape the configured target via a symlink.
func checkedParents(root, rel string, create bool) string {
	var script strings.Builder
	current := root
	fmt.Fprintf(&script, "[ -d %s ] && [ ! -L %s ] || exit 71; ", ShellQuote(current), ShellQuote(current))
	parent := path.Dir(rel)
	if parent == "." {
		return script.String()
	}
	for _, part := range strings.Split(parent, "/") {
		current = path.Join(current, part)
		quoted := ShellQuote(current)
		fmt.Fprintf(&script, "[ ! -L %s ] || exit 71; ", quoted)
		if create {
			fmt.Fprintf(&script, "if [ ! -e %s ]; then mkdir -- %s || exit; fi; ", quoted, quoted)
		}
		fmt.Fprintf(&script, "[ -d %s ] || exit 71; ", quoted)
	}
	return script.String()
}

func (p *gitPlan) pendingPath() string {
	dir, _ := Dir(p.original.ID)
	return filepath.Join(dir, "changes-pending.json")
}

func (p *gitPlan) guardSelectionMoves(ctx context.Context, t *Transport, dry bool) error {
	if p == nil || len(p.cleanup) == 0 {
		return nil
	}
	pending := struct {
		Identity   string
		Selections []Selection
	}{selectionIdentity(p.original), p.effective.Selections}
	// Validate existing parent components even when resuming a partially applied move.
	var parents strings.Builder
	for i, s := range p.original.Selections {
		next := p.effective.Selections[i]
		if s.Path == next.Path {
			continue
		}
		current := p.original.Target()
		for _, part := range strings.Split(path.Dir(next.Path), "/") {
			if part == "." {
				continue
			}
			current = path.Join(current, part)
			quoted := ShellQuote(current)
			fmt.Fprintf(&parents, "[ ! -L %s ] && { [ ! -e %s ] || [ -d %s ]; } || exit 71; ", quoted, quoted, quoted)
		}
		quoted := ShellQuote(path.Join(p.original.Target(), next.Path))
		fmt.Fprintf(&parents, "[ ! -L %s ] || exit 71; ", quoted)
	}
	if _, err := t.Run(ctx, "sh -c "+ShellQuote(parents.String())); err != nil {
		return fmt.Errorf("renamed selection has unsafe remote parent: %w", err)
	}
	data, err := os.ReadFile(p.pendingPath())
	if err == nil {
		// A failed transfer may have already created the new path. Only this exact pending plan may resume it.
		want, err := json.MarshalIndent(pending, "", "  ")
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(data)) == strings.TrimSpace(string(want)) {
			return nil
		}
		return fmt.Errorf("a different folder move is pending; finish it before changing the selection")
	} else if !os.IsNotExist(err) {
		return err
	}
	var checks strings.Builder
	for i, s := range p.original.Selections {
		next := p.effective.Selections[i]
		if s.Path == next.Path {
			continue
		}
		dest := ShellQuote(path.Join(p.original.Target(), next.Path))
		// Before the first transfer, never claim an existing out-of-scope destination.
		fmt.Fprintf(&checks, "[ ! -e %s ] && [ ! -L %s ] || exit 72; ", dest, dest)
	}
	out, err := t.Run(ctx, "sh -c "+ShellQuote(checks.String()))
	if err != nil {
		return fmt.Errorf("cannot follow renamed selection: its remote destination already exists or is unavailable: %w %s", err, out)
	}
	if !dry {
		return writeJSON(p.pendingPath(), pending)
	}
	return nil
}

// removableParents skips a path whose remote parent is already gone and refuses
// to follow a symlinked parent out of the configured target.
func removableParents(root, rel string) string {
	var script strings.Builder
	current := root
	fmt.Fprintf(&script, "[ -d %s ] && [ ! -L %s ] || exit 71; ", ShellQuote(current), ShellQuote(current))
	if parent := path.Dir(rel); parent != "." {
		for _, part := range strings.Split(parent, "/") {
			current = path.Join(current, part)
			quoted := ShellQuote(current)
			fmt.Fprintf(&script, "[ ! -L %s ] || exit 71; [ -d %s ] || exit 0; ", quoted, quoted)
		}
	}
	return script.String()
}

// removeRemote deletes managed entries that disappeared locally. Directories are
// removed only once empty, so remote data xsync does not manage is preserved.
func (s *changeSet) removeRemote(ctx context.Context, t *Transport, root string) error {
	items := make([]string, 0, len(s.RemoveFiles)+len(s.RemoveDirs))
	for _, rel := range s.RemoveFiles {
		target := ShellQuote(path.Join(root, rel))
		items = append(items, "("+removableParents(root, rel)+"if [ -L "+target+" ] || { [ -e "+target+" ] && [ ! -d "+target+" ]; }; then rm -f -- "+target+" || exit; fi) || exit")
	}
	for _, rel := range s.RemoveDirs {
		target := ShellQuote(path.Join(root, rel))
		items = append(items, "("+removableParents(root, rel)+"if [ -d "+target+" ] && [ ! -L "+target+" ]; then rmdir -- "+target+" 2>/dev/null; fi; exit 0) || exit")
	}
	const batch = 100
	for start := 0; start < len(items); start += batch {
		end := min(start+batch, len(items))
		reportProgress(ctx, Progress{Stage: "deleting", Checked: int64(start), Total: int64(len(items))})
		if _, err := t.runWithTimeout(ctx, "sh -c "+ShellQuote(strings.Join(items[start:end], "; ")), 5*time.Minute); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("remote deletion failed: %w", err)
		}
	}
	return nil
}

// Remote Git is unnecessary. Hash verification prevents moving a remotely modified file.
func (p *gitPlan) moveRemote(ctx context.Context, t *Transport, dry bool) (string, error) {
	if p == nil {
		return "", nil
	}
	if p.warning != "" {
		return p.warning, nil
	}
	var output strings.Builder
	for i, s := range p.original.Selections {
		if next := p.effective.Selections[i]; next.Path != s.Path {
			fmt.Fprintf(&output, "Selected path: %s -> %s\n", s.Path, next.Path)
		}
	}
	if err := p.guardSelectionMoves(ctx, t, dry); err != nil {
		return output.String(), err
	}
	for i, move := range p.moves {
		reportProgress(ctx, Progress{Stage: "renaming", Checked: int64(i), Total: int64(len(p.moves))})
		if dry {
			fmt.Fprintf(&output, "Git rename candidate: %s -> %s (remote hash checked when syncing)\n", move.From, move.To)
			continue
		}
		src, dst := ShellQuote(path.Join(p.original.Target(), move.From)), ShellQuote(path.Join(p.original.Target(), move.To))
		script := fmt.Sprintf("if [ -f %s ] && [ ! -L %s ] && [ ! -e %s ] && [ ! -L %s ]; then ", src, src, dst, dst) +
			checkedParents(p.original.Target(), move.From, false) +
			"if command -v sha256sum >/dev/null 2>&1; then got=$(sha256sum < " + src + ") || exit; got=${got%% *}; " +
			"elif command -v shasum >/dev/null 2>&1; then got=$(shasum -a 256 < " + src + ") || exit; got=${got%% *}; " +
			"elif command -v openssl >/dev/null 2>&1; then got=$(openssl dgst -sha256 < " + src + ") || exit; got=${got##* }; else exit 0; fi; " +
			"if [ \"$got\" = " + ShellQuote(move.Hash) + " ]; then " + checkedParents(p.original.Target(), move.To, true) +
			"mv -n -- " + src + " " + dst + " || exit; if [ ! -e " + src + " ] && [ ! -L " + src + " ]; then printf 'moved'; fi; fi; fi"
		result, err := t.runWithTimeout(ctx, "sh -c "+ShellQuote(script), 5*time.Minute)
		if err != nil {
			if ctx.Err() != nil {
				return output.String(), ctx.Err()
			}
			// Do not race rsync against an SSH command that may have timed out after moving data.
			return output.String(), fmt.Errorf("remote rename verification interrupted; retry before transferring: %w", err)
		} else if string(result) == "moved" {
			fmt.Fprintf(&output, "Git rename reused remote data: %s -> %s\n", move.From, move.To)
		}
	}
	return output.String(), nil
}
