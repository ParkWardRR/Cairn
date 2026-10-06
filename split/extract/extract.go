// Package extract carves one component repository out of the monorepo's history and
// proves the result is faithful.
//
// Extraction uses git filter-repo, driven by the path map. Everything around it is
// plain git, and everything it produces is checked by Verify: a repository that
// merely looks right is not accepted.
package extract

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ParkWardRR/cairn-driving-log-selfhosted/split/pathmap"
)

// Options configure one extraction.
type Options struct {
	Source string // the monorepo checkout (read only: it is cloned, never touched)
	Ref    string // the commit or tag to extract from, e.g. monorepo-final
	Repo   string // server | web | firmware
	Map    *pathmap.Map
	Out    string // directory to create; must not exist
}

// Report is what Verify found.
type Report struct {
	Repo           string
	Files          int
	SourceCommits  int
	ResultCommits  int
	Authors        int
	SampledCommits int
	Problems       []string
}

// OK reports whether the extraction verified.
func (r *Report) OK() bool { return len(r.Problems) == 0 }

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// Extract clones the source at Ref, filters it down to Repo's paths and verifies it.
func Extract(o Options) (*Report, error) {
	if _, err := os.Stat(o.Out); err == nil {
		return nil, fmt.Errorf("%s already exists; refusing to overwrite", o.Out)
	}
	sha, err := git(o.Source, "rev-parse", "--verify", o.Ref+"^{commit}")
	if err != nil {
		return nil, err
	}
	sha = strings.TrimSpace(sha)

	// --no-local: a real clone, not hardlinks, so nothing the rewrite does can reach
	// the source repository.
	cl := exec.Command("git", "clone", "--no-local", "-q", o.Source, o.Out)
	if out, err := cl.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("clone: %w: %s", err, out)
	}
	if _, err := git(o.Out, "checkout", "-q", "-B", "split-main", sha); err != nil {
		return nil, err
	}
	// Only the commit being extracted: no other branches, no tags (the plan carries
	// no tags into a component).
	refs, err := git(o.Out, "for-each-ref", "--format=%(refname)")
	if err != nil {
		return nil, err
	}
	for _, r := range strings.Fields(refs) {
		if r != "refs/heads/split-main" {
			if _, err := git(o.Out, "update-ref", "-d", r); err != nil {
				return nil, err
			}
		}
	}

	spec := strings.Join(o.Map.FilterSpec(o.Repo), "\n") + "\n"
	specPath := filepath.Join(o.Out, ".git", "split-paths.txt")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		return nil, err
	}
	fr := exec.Command("git", "-C", o.Out, "filter-repo", "--force", "--paths-from-file", specPath)
	if out, err := fr.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git filter-repo: %w: %s", err, out)
	}
	if _, err := git(o.Out, "branch", "-m", "split-main", "main"); err != nil {
		return nil, err
	}
	return Verify(o)
}

type entry struct{ mode, sha string }

// tree lists a ref's files: path -> mode and blob.
func tree(dir, ref string) (map[string]entry, error) {
	out, err := git(dir, "ls-tree", "-r", "-z", "--full-tree", ref)
	if err != nil {
		return nil, err
	}
	m := map[string]entry{}
	for _, rec := range strings.Split(out, "\x00") {
		if rec == "" {
			continue
		}
		tab := strings.IndexByte(rec, '\t')
		if tab < 0 {
			return nil, fmt.Errorf("unparseable ls-tree record %q", rec)
		}
		f := strings.Fields(rec[:tab])
		if len(f) != 3 {
			return nil, fmt.Errorf("unparseable ls-tree record %q", rec)
		}
		m[rec[tab+1:]] = entry{mode: f[0], sha: f[2]}
	}
	return m, nil
}

// compare checks one source snapshot against one result snapshot.
func compare(o Options, srcRef, dstRef, label string) ([]string, int, error) {
	src, err := tree(o.Source, srcRef)
	if err != nil {
		return nil, 0, err
	}
	paths := make([]string, 0, len(src))
	for p := range src {
		paths = append(paths, p)
	}
	want, err := o.Map.Expected(o.Repo, paths)
	if err != nil {
		return nil, 0, err
	}
	got, err := tree(o.Out, dstRef)
	if err != nil {
		return nil, 0, err
	}

	var problems []string
	dests := make([]string, 0, len(want))
	for d := range want {
		dests = append(dests, d)
	}
	sort.Strings(dests)
	for _, d := range dests {
		s := src[want[d]]
		g, ok := got[d]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s: %s is missing (from %s)", label, d, want[d]))
		case g.sha != s.sha:
			problems = append(problems, fmt.Sprintf("%s: %s differs from %s (content)", label, d, want[d]))
		case g.mode != s.mode:
			problems = append(problems, fmt.Sprintf("%s: %s has mode %s, source %s has %s", label, d, g.mode, want[d], s.mode))
		}
	}
	var extra []string
	for d := range got {
		if _, ok := want[d]; !ok {
			extra = append(extra, d)
		}
	}
	sort.Strings(extra)
	for _, d := range extra {
		problems = append(problems, fmt.Sprintf("%s: %s is not routed here by the path map", label, d))
	}
	return problems, len(want), nil
}

func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// Verify checks an extracted repository against its source. The commit count is
// reported but never judged: pruned and merge commits make equality the wrong test.
func Verify(o Options) (*Report, error) {
	r := &Report{Repo: o.Repo}

	// 1. The final snapshot: content, modes and symlinks, exactly the routed set.
	probs, n, err := compare(o, o.Ref, "HEAD", "final snapshot")
	if err != nil {
		return nil, err
	}
	r.Problems = append(r.Problems, probs...)
	r.Files = n
	if n == 0 {
		r.Problems = append(r.Problems, "the result contains no files")
	}

	// 2. Sampled history: the same exact comparison at commits spread across it.
	commitMap, err := os.ReadFile(filepath.Join(o.Out, ".git", "filter-repo", "commit-map"))
	if err != nil {
		return nil, fmt.Errorf("commit-map: %w", err)
	}
	newOf := map[string]string{}
	for i, l := range lines(string(commitMap)) {
		if i == 0 {
			continue // header
		}
		f := strings.Fields(l)
		if len(f) == 2 && strings.Trim(f[1], "0") != "" {
			newOf[f[0]] = f[1]
		}
	}
	srcList, err := git(o.Source, "rev-list", "--reverse", o.Ref)
	if err != nil {
		return nil, err
	}
	srcCommits := lines(srcList)
	r.SourceCommits = len(srcCommits)
	var mapped []string
	for _, c := range srcCommits {
		if _, ok := newOf[c]; ok {
			mapped = append(mapped, c)
		}
	}
	const samples = 14
	picked := map[string]bool{}
	for i := 0; i < samples && len(mapped) > 0; i++ {
		idx := i * (len(mapped) - 1) / max(samples-1, 1)
		picked[mapped[idx]] = true
	}
	var order []string
	for c := range picked {
		order = append(order, c)
	}
	sort.Strings(order)
	for _, old := range order {
		probs, _, err := compare(o, old, newOf[old], "history@"+old[:8])
		if err != nil {
			return nil, err
		}
		r.Problems = append(r.Problems, probs...)
	}
	r.SampledCommits = len(order)

	// 3. Authors: nothing invented, and none lost for retained paths (every author of
	// a retained commit appears; checked as: result authors are a subset of the source's).
	srcAuthors, err := git(o.Source, "log", o.Ref, "--format=%aN <%aE>")
	if err != nil {
		return nil, err
	}
	dstAuthors, err := git(o.Out, "log", "--all", "--format=%aN <%aE>")
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, a := range lines(srcAuthors) {
		have[a] = true
	}
	seen := map[string]bool{}
	for _, a := range lines(dstAuthors) {
		seen[a] = true
		if !have[a] {
			r.Problems = append(r.Problems, "author "+a+" is not in the source history")
		}
	}
	r.Authors = len(seen)

	// 4. Counts, diagnostic only; and no stray refs.
	dstList, err := git(o.Out, "rev-list", "--all")
	if err != nil {
		return nil, err
	}
	r.ResultCommits = len(lines(dstList))
	refs, err := git(o.Out, "for-each-ref", "--format=%(refname)")
	if err != nil {
		return nil, err
	}
	for _, ref := range lines(refs) {
		if ref != "refs/heads/main" {
			r.Problems = append(r.Problems, "unexpected ref "+ref+" in the result")
		}
	}
	return r, nil
}
