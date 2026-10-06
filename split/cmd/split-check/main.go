// Command split-check verifies that every tracked path at a git ref is routed by
// paths.tsv, and prints how the paths divide between repositories.
//
//	go run ./cmd/split-check --ref HEAD
//
// It exits non-zero if any path matches no rule, so a file added anywhere in the
// monorepo cannot be left out of the split unnoticed.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/ParkWardRR/Cairn/split/pathmap"
)

func main() {
	ref := flag.String("ref", "HEAD", "git ref to check")
	repo := flag.String("repo", ".", "path to the monorepo checkout")
	tsv := flag.String("paths", "paths.tsv", "the path map")
	verbose := flag.Bool("v", false, "list every path under its repository")
	flag.Parse()

	m, err := pathmap.Load(*tsv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "split-check:", err)
		os.Exit(2)
	}

	cmd := exec.Command("git", "-C", *repo, "ls-tree", "-r", "-z", "--name-only", *ref)
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "split-check: git ls-tree:", err)
		os.Exit(2)
	}
	var paths []string
	for _, p := range bytes.Split(out, []byte{0}) {
		if len(p) > 0 {
			paths = append(paths, string(p))
		}
	}

	byRepo, uncovered := m.Coverage(paths)
	names := make([]string, 0, len(byRepo))
	for r := range byRepo {
		names = append(names, r)
	}
	sort.Strings(names)
	fmt.Printf("%d tracked paths at %s\n", len(paths), *ref)
	for _, r := range names {
		fmt.Printf("  %-9s %4d\n", r, len(byRepo[r]))
		if *verbose {
			for _, p := range byRepo[r] {
				rule, _ := m.Match(p)
				fmt.Printf("      %s -> %s\n", p, m.DestPath(rule, p))
			}
		}
	}

	// A destination collision would silently merge two files.
	for _, r := range pathmap.Extracted {
		if _, err := m.Expected(r, paths); err != nil {
			fmt.Fprintf(os.Stderr, "split-check: %v\n", err)
			os.Exit(1)
		}
	}

	// A rule that matches nothing is usually a typo or a stale row.
	var stale []string
	for _, r := range m.Rules {
		hit := false
		for _, p := range paths {
			if got, ok := m.Match(p); ok && got.Source == r.Source {
				hit = true
				break
			}
		}
		if !hit {
			stale = append(stale, fmt.Sprintf("line %d: %s", r.Line, r.Source))
		}
	}
	if len(stale) > 0 {
		fmt.Printf("\nrules that route no tracked path (stale?):\n  %s\n", strings.Join(stale, "\n  "))
	}

	if len(uncovered) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d path(s) match no rule in %s:\n  %s\n", len(uncovered), *tsv, strings.Join(uncovered, "\n  "))
		os.Exit(1)
	}
	fmt.Println("\nevery tracked path is routed")
}
