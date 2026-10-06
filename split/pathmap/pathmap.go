// Package pathmap routes every tracked path of the Cairn monorepo to the repository it
// belongs in, and turns that routing into the path filters git filter-repo needs.
//
// The rules live in paths.tsv. The properties that matter, and that the tests pin:
//
//   - the LONGEST matching source wins, so a directory can be routed wholesale and a
//     child of it sent elsewhere;
//   - a path that matches no rule is an error, reported by Coverage, never silently
//     dropped: dropping is what the split exists to do deliberately, not by accident;
//   - the filter for one repository never includes a path that a more specific rule
//     sends to another.
package pathmap

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Repos a rule may name.
const (
	Server   = "server"
	Web      = "web"
	Firmware = "firmware"
	Door     = "door"
	Archive  = "archive"
)

// Extracted lists the repositories created by extraction. The front door already exists.
var Extracted = []string{Server, Web, Firmware}

// Rule routes one source file or directory.
type Rule struct {
	Source string // a file, or a directory when it ends with "/"
	Repo   string
	Dest   string // in Repo; "." is the root
	Note   string
	Line   int
}

// IsDir reports whether the rule covers a directory.
func (r Rule) IsDir() bool { return strings.HasSuffix(r.Source, "/") }

// Map is a set of rules.
type Map struct{ Rules []Rule }

// Load reads paths.tsv.
func Load(path string) (*Map, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m := &Map{}
	seen := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 3 {
			return nil, fmt.Errorf("%s:%d: want source, repo, destination (tab-separated), got %q", path, n, line)
		}
		r := Rule{Source: cols[0], Repo: cols[1], Dest: cols[2], Line: n}
		if len(cols) > 3 {
			r.Note = cols[3]
		}
		switch r.Repo {
		case Server, Web, Firmware, Door, Archive:
		default:
			return nil, fmt.Errorf("%s:%d: unknown repo %q", path, n, r.Repo)
		}
		if r.Source == "" || strings.HasPrefix(r.Source, "/") || strings.Contains(r.Source, "..") || strings.Contains(r.Source, "//") {
			return nil, fmt.Errorf("%s:%d: bad source %q", path, n, r.Source)
		}
		if prev, dup := seen[r.Source]; dup {
			return nil, fmt.Errorf("%s:%d: %q is already routed on line %d", path, n, r.Source, prev)
		}
		seen[r.Source] = n
		if r.IsDir() && r.Repo != Door && r.Repo != Archive && !strings.HasSuffix(r.Dest, "/") && r.Dest != "." {
			return nil, fmt.Errorf("%s:%d: directory %q needs a destination ending in / (or .)", path, n, r.Source)
		}
		m.Rules = append(m.Rules, r)
	}
	return m, sc.Err()
}

// Match returns the rule that routes p: the longest matching source.
func (m *Map) Match(p string) (Rule, bool) {
	var best Rule
	found := false
	for _, r := range m.Rules {
		ok := false
		if r.IsDir() {
			ok = strings.HasPrefix(p, r.Source)
		} else {
			ok = p == r.Source
		}
		if ok && (!found || len(r.Source) > len(best.Source)) {
			best, found = r, true
		}
	}
	return best, found
}

// DestPath is where p lands in its repository.
func (m *Map) DestPath(r Rule, p string) string {
	if !r.IsDir() {
		if r.Dest == "." {
			return p
		}
		return r.Dest
	}
	rest := strings.TrimPrefix(p, r.Source)
	if r.Dest == "." {
		return rest
	}
	return r.Dest + rest
}

// Coverage routes every path. Uncovered paths are returned sorted.
func (m *Map) Coverage(paths []string) (byRepo map[string][]string, uncovered []string) {
	byRepo = map[string][]string{}
	for _, p := range paths {
		r, ok := m.Match(p)
		if !ok {
			uncovered = append(uncovered, p)
			continue
		}
		byRepo[r.Repo] = append(byRepo[r.Repo], p)
	}
	sort.Strings(uncovered)
	return byRepo, uncovered
}

// Expected is the exact set of (destination path -> source path) a repository must
// contain, from a list of source paths.
func (m *Map) Expected(repo string, paths []string) (map[string]string, error) {
	out := map[string]string{}
	for _, p := range paths {
		r, ok := m.Match(p)
		if !ok || r.Repo != repo {
			continue
		}
		d := m.DestPath(r, p)
		if prev, clash := out[d]; clash {
			return nil, fmt.Errorf("%s and %s both map to %s in %s", prev, p, d, repo)
		}
		out[d] = p
	}
	return out, nil
}

// FilterSpec returns the lines of a git filter-repo --paths-from-file for repo.
//
// filter-repo treats a "old==>new" line as a RENAME only; it does not select the
// path. So every rule emits two lines: one that includes the paths, and one that
// renames them. (Emitting only the rename keeps every path in the repository, which
// the extraction tests caught.) A directory rule excludes the sources of any more
// specific rule beneath it with a negative lookahead, so a child routed elsewhere is
// not also swept up by its parent.
func (m *Map) FilterSpec(repo string) []string {
	var lines []string
	for _, r := range m.Rules {
		if r.Repo != repo {
			continue
		}
		if !r.IsDir() {
			lines = append(lines, "literal:"+r.Source)
			dest := r.Dest
			if dest == "." {
				dest = r.Source
			}
			if dest != r.Source {
				lines = append(lines, fmt.Sprintf("literal:%s==>%s", r.Source, dest))
			}
			continue
		}

		var carve []string
		for _, o := range m.Rules {
			if o.Source != r.Source && strings.HasPrefix(o.Source, r.Source) {
				carve = append(carve, regexp.QuoteMeta(strings.TrimPrefix(o.Source, r.Source)))
			}
		}
		sort.Strings(carve)
		look := ""
		if len(carve) > 0 {
			look = "(?!(?:" + strings.Join(carve, "|") + "))"
		}
		pattern := "^" + regexp.QuoteMeta(r.Source) + look
		lines = append(lines, "regex:"+pattern+".+$")

		dest := r.Dest
		if dest == "." {
			dest = ""
		}
		if dest != r.Source {
			lines = append(lines, fmt.Sprintf("regex:%s(.+)$==>%s\\1", pattern, dest))
		}
	}
	return lines
}
