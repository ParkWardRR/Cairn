// Command split-reduce removes from the front door everything the path map routes to a
// component repository, once those repositories are published and production runs from
// them. History is untouched: the removal is one ordinary commit's worth of deletions.
//
//	go run ./cmd/split-reduce --root .. --keep tests/check-runners.sh,tests/check-runners-selftest.sh
//
// It stages the deletions (git rm); committing is left to the caller.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ParkWardRR/cairn-driving-log-selfhosted/split/pathmap"
)

func main() {
	root := flag.String("root", "..", "the repository checkout")
	tsv := flag.String("paths", "paths.tsv", "the path map")
	keep := flag.String("keep", "", "comma-separated paths to keep although they are routed elsewhere")
	dry := flag.Bool("dry-run", false, "list what would be removed")
	flag.Parse()

	m, err := pathmap.Load(*tsv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "split-reduce:", err)
		os.Exit(2)
	}
	keepSet := map[string]bool{}
	for _, k := range strings.Split(*keep, ",") {
		if k = strings.TrimSpace(k); k != "" {
			keepSet[k] = true
		}
	}

	out, err := exec.Command("git", "-C", *root, "ls-files", "-z").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "git ls-files:", err)
		os.Exit(2)
	}
	var remove []string
	for _, f := range bytes.Split(out, []byte{0}) {
		p := string(f)
		if p == "" || keepSet[p] {
			continue
		}
		r, ok := m.Match(p)
		if !ok {
			fmt.Fprintf(os.Stderr, "split-reduce: %s is routed nowhere; refusing to guess\n", p)
			os.Exit(1)
		}
		switch r.Repo {
		case pathmap.Server, pathmap.Web, pathmap.Firmware:
			remove = append(remove, p)
		}
	}
	for k := range keepSet {
		if r, ok := m.Match(k); ok && r.Repo == pathmap.Door {
			fmt.Fprintf(os.Stderr, "note: --keep %s is already the front door's\n", k)
		}
	}
	fmt.Printf("%d path(s) routed to the component repositories\n", len(remove))
	if *dry {
		for _, p := range remove {
			fmt.Println(" ", p)
		}
		return
	}
	for i := 0; i < len(remove); i += 200 {
		end := min(i+200, len(remove))
		args := append([]string{"-C", *root, "rm", "-q", "--"}, remove[i:end]...)
		if o, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "git rm: %v\n%s", err, o)
			os.Exit(1)
		}
	}
	fmt.Println("removed (staged)")
}
