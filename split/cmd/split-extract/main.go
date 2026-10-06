// Command split-extract carves one component repository out of the monorepo and
// verifies it. It never modifies the monorepo: it works on a throwaway clone.
//
//	go run ./cmd/split-extract --source .. --ref monorepo-final --repo server --out /tmp/cairn-vehicle-server
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ParkWardRR/Cairn/split/extract"
	"github.com/ParkWardRR/Cairn/split/pathmap"
)

func main() {
	source := flag.String("source", "..", "the monorepo checkout")
	ref := flag.String("ref", "HEAD", "commit or tag to extract (monorepo-final for the real run)")
	repo := flag.String("repo", "", "server | web | firmware")
	out := flag.String("out", "", "directory to create")
	tsv := flag.String("paths", "paths.tsv", "the path map")
	flag.Parse()

	if *repo == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: split-extract --repo server|web|firmware --out DIR [--ref REF] [--source DIR]")
		os.Exit(2)
	}
	m, err := pathmap.Load(*tsv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "split-extract:", err)
		os.Exit(2)
	}
	abs, _ := filepath.Abs(*out)
	rep, err := extract.Extract(extract.Options{Source: *source, Ref: *ref, Repo: *repo, Map: m, Out: abs})
	if err != nil {
		fmt.Fprintln(os.Stderr, "split-extract:", err)
		os.Exit(1)
	}
	fmt.Printf("%s: %d files; %d source commits -> %d (diagnostic); %d authors; %d historical snapshots verified\n",
		rep.Repo, rep.Files, rep.SourceCommits, rep.ResultCommits, rep.Authors, rep.SampledCommits)
	if !rep.OK() {
		for _, p := range rep.Problems {
			fmt.Fprintln(os.Stderr, "  PROBLEM:", p)
		}
		os.Exit(1)
	}
	fmt.Println("verified")
}
