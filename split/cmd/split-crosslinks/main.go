// Command split-crosslinks rewrites relative markdown links that no longer resolve (because
// their targets moved to another repository) into links to the repository that owns them.
// It is the same logic split-rewrite applies to an extracted repository, here applied to the
// front door after its implementation code has been removed.
//
//	go run ./cmd/split-crosslinks --repo door --dir .. --source .. --ref monorepo-final
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ParkWardRR/cairn-driving-log-selfhosted/split/pathmap"
	"github.com/ParkWardRR/cairn-driving-log-selfhosted/split/rewrite"
)

func main() {
	repo := flag.String("repo", "door", "the repository whose links are fixed")
	dir := flag.String("dir", "..", "its working tree")
	source := flag.String("source", "..", "the monorepo checkout (for the layout the links were written against)")
	ref := flag.String("ref", "monorepo-final", "the monorepo ref that layout is read from")
	tsv := flag.String("paths", "paths.tsv", "the path map")
	flag.Parse()

	m, err := pathmap.Load(*tsv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "split-crosslinks:", err)
		os.Exit(2)
	}
	abs, _ := filepath.Abs(*dir)
	err = rewrite.CrossLinks(rewrite.Options{
		Repo: *repo, Dir: abs, Source: *source, Ref: *ref, Map: m,
		Log: func(f string, a ...any) { fmt.Printf(f+"\n", a...) },
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "split-crosslinks:", err)
		os.Exit(1)
	}
	fmt.Println("links rewritten")
}
