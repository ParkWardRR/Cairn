// Command split-rewrite applies the post-extraction rewrites to an extracted repository:
// one commit per change, on top of the extracted history.
//
//	go run ./cmd/split-extract --source .. --ref monorepo-final --repo server --out /tmp/server
//	go run ./cmd/split-rewrite --source .. --ref monorepo-final --repo server --dir /tmp/server
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ParkWardRR/Cairn/split/pathmap"
	"github.com/ParkWardRR/Cairn/split/rewrite"
)

func gitConfig(dir, key string) string {
	out, _ := exec.Command("git", "-C", dir, "config", key).Output()
	return strings.TrimSpace(string(out))
}

func main() {
	source := flag.String("source", "..", "the monorepo checkout")
	ref := flag.String("ref", "monorepo-final", "the tag the extraction was made from")
	repo := flag.String("repo", "", "server | web | firmware")
	dir := flag.String("dir", "", "the extracted repository")
	tsv := flag.String("paths", "paths.tsv", "the path map")
	ctag := flag.String("contracts-tag", "", "tag of the contracts release to pin (default: --ref)")
	author := flag.String("author", "", "commit author name (default: the source repository's user.name)")
	target := flag.String("deploy-target", os.Getenv("CAIRN_DEPLOY_TARGET"), "web only: the user@host the old deploy scripts hard-coded (or CAIRN_DEPLOY_TARGET)")
	site := flag.String("site-host", os.Getenv("CAIRN_SITE_HOST"), "web only: the public site name the old scripts hard-coded (or CAIRN_SITE_HOST)")
	email := flag.String("email", "", "commit author email (default: the source repository's user.email)")
	flag.Parse()

	if *repo == "" || *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: split-rewrite --repo server|web|firmware --dir DIR [--ref TAG] [--source DIR]")
		os.Exit(2)
	}
	m, err := pathmap.Load(*tsv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "split-rewrite:", err)
		os.Exit(2)
	}
	if *author == "" {
		*author = gitConfig(*source, "user.name")
	}
	if *email == "" {
		*email = gitConfig(*source, "user.email")
	}
	if *author == "" || *email == "" {
		fmt.Fprintln(os.Stderr, "split-rewrite: no commit identity: set user.name and user.email in the source repository or pass --author and --email")
		os.Exit(2)
	}
	if *ctag == "" {
		*ctag = *ref
	}
	abs, _ := filepath.Abs(*dir)
	err = rewrite.Run(rewrite.Options{
		Repo: *repo, Dir: abs, Source: *source, Ref: *ref, Map: m,
		ContractsTag: *ctag, Author: *author, Email: *email, DeployTarget: *target, SiteHost: *site,
		Log: func(f string, a ...any) { fmt.Printf(f+"\n", a...) },
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "split-rewrite:", err)
		os.Exit(1)
	}
	fmt.Println("rewritten")
}
