// Package links rewrites and checks the relative links in a repository's markdown.
//
// Moving a document breaks every link to it AND every relative link inside it. Fixing
// that by hand across dozens of files is how links rot, so the move is described once,
// as a set of path moves, and every link is recomputed from it. The same code checks
// that every relative link resolves, which is the docs CI's job afterwards.
package links

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Moves maps old repo-relative paths to new ones. A key may be a file or a directory
// (directories apply to everything beneath them).
type Moves map[string]string

// Apply returns the new location of an old path (itself if it did not move).
func (m Moves) Apply(p string) string {
	if n, ok := m[p]; ok {
		return n
	}
	// the longest directory prefix wins
	best := ""
	for old := range m {
		if strings.HasPrefix(p, old+"/") && len(old) > len(best) {
			best = old
		}
	}
	if best != "" {
		return m[best] + strings.TrimPrefix(p, best)
	}
	return p
}

// Old returns where a new path was before the moves (itself if it did not move).
func (m Moves) Old(p string) string {
	if o, ok := m.inverse()[p]; ok {
		return o
	}
	inv := m.inverse()
	best := ""
	for n := range inv {
		if strings.HasPrefix(p, n+"/") && len(n) > len(best) {
			best = n
		}
	}
	if best != "" {
		return inv[best] + strings.TrimPrefix(p, best)
	}
	return p
}

func (m Moves) inverse() map[string]string {
	inv := make(map[string]string, len(m))
	for o, n := range m {
		inv[n] = o
	}
	return inv
}

var (
	mdLink   = regexp.MustCompile(`\]\(([^)\s]+)(\s+"[^"]*")?\)`)
	htmlAttr = regexp.MustCompile(`\b(src|href)="([^"]+)"`)
)

func isRelative(t string) bool {
	if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "/") || strings.HasPrefix(t, "mailto:") {
		return false
	}
	return !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`).MatchString(t)
}

// splitFrag separates "path#frag" or "path?q".
func splitFrag(t string) (string, string) {
	if i := strings.IndexAny(t, "#?"); i >= 0 {
		return t[:i], t[i:]
	}
	return t, ""
}

// retarget recomputes one relative link target for a file that is now at newFile and
// was at oldFile. It returns the new target text.
func retarget(target, oldFile, newFile string, m Moves) string {
	p, frag := splitFrag(target)
	if p == "" {
		return target
	}
	// where the link pointed, in the OLD tree
	oldTarget := path.Clean(path.Join(path.Dir(oldFile), p))
	newTarget := m.Apply(oldTarget)
	rel, err := relPath(path.Dir(newFile), newTarget)
	if err != nil {
		return target
	}
	// keep a trailing slash if the original had one
	if strings.HasSuffix(p, "/") && !strings.HasSuffix(rel, "/") {
		rel += "/"
	}
	// keep the original text when nothing about it changed (preserves style: ./x vs x)
	if path.Clean(path.Join(path.Dir(newFile), p)) == newTarget {
		return target
	}
	return rel + frag
}

func relPath(fromDir, to string) (string, error) {
	if fromDir == "" {
		fromDir = "."
	}
	r, err := filepath.Rel(filepath.FromSlash(fromDir), filepath.FromSlash(to))
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(r), nil
}

// Rewrite fixes the relative links of one markdown document. It skips fenced code.
func Rewrite(text, oldFile, newFile string, m Moves) string {
	var out strings.Builder
	inFence := false
	for _, line := range strings.SplitAfter(text, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			inFence = !inFence
			out.WriteString(line)
			continue
		}
		if inFence {
			out.WriteString(line)
			continue
		}
		line = mdLink.ReplaceAllStringFunc(line, func(s string) string {
			sub := mdLink.FindStringSubmatch(s)
			if !isRelative(sub[1]) {
				return s
			}
			return "](" + retarget(sub[1], oldFile, newFile, m) + sub[2] + ")"
		})
		line = htmlAttr.ReplaceAllStringFunc(line, func(s string) string {
			sub := htmlAttr.FindStringSubmatch(s)
			if !isRelative(sub[2]) {
				return s
			}
			return sub[1] + `="` + retarget(sub[2], oldFile, newFile, m) + `"`
		})
		out.WriteString(line)
	}
	return out.String()
}

// ReplacePaths rewrites mentions of moved repo-root-relative paths in any text (code
// comments, build files, prose). A mention must start at a word edge, so "x/fixtures/a"
// or "../fixtures/a" is left for a human: those are relative to somewhere, and guessing
// would be wrong.
func ReplacePaths(text string, m Moves) string {
	olds := make([]string, 0, len(m))
	for o := range m {
		olds = append(olds, o)
	}
	sort.Slice(olds, func(i, j int) bool { return len(olds[i]) > len(olds[j]) })
	for _, o := range olds {
		re := regexp.MustCompile(`(^|[^A-Za-z0-9_./-])` + regexp.QuoteMeta(o) + `($|[^A-Za-z0-9_-])`)
		text = re.ReplaceAllStringFunc(text, func(s string) string {
			sub := re.FindStringSubmatch(s)
			return sub[1] + m[o] + sub[2]
		})
	}
	return text
}

// Broken is a relative link that resolves to nothing.
type Broken struct{ File, Target string }

func (b Broken) String() string { return fmt.Sprintf("%s: %s", b.File, b.Target) }

// Check reports every relative link in the given markdown files (paths relative to root)
// that does not resolve to an existing file or directory.
func Check(root string, files []string) ([]Broken, error) {
	var bad []Broken
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			return nil, err
		}
		inFence := false
		for _, line := range strings.Split(string(b), "\n") {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			var targets []string
			for _, s := range mdLink.FindAllStringSubmatch(line, -1) {
				targets = append(targets, s[1])
			}
			for _, s := range htmlAttr.FindAllStringSubmatch(line, -1) {
				targets = append(targets, s[2])
			}
			for _, t := range targets {
				if !isRelative(t) {
					continue
				}
				p, _ := splitFrag(t)
				if p == "" {
					continue
				}
				abs := filepath.Join(root, filepath.FromSlash(path.Join(path.Dir(f), p)))
				if _, err := os.Stat(abs); err != nil {
					bad = append(bad, Broken{File: f, Target: t})
				}
			}
		}
	}
	return bad, nil
}
