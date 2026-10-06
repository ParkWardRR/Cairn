package rewrite

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ParkWardRR/cairn-driving-log-selfhosted/split/pathmap"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func put(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeMap(t *testing.T, rules string) *pathmap.Map {
	t.Helper()
	p := filepath.Join(t.TempDir(), "paths.tsv")
	if err := os.WriteFile(p, []byte(rules), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := pathmap.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// A relative link to a file that now lives in another repository must become an absolute
// link to that repository, and one that still resolves must be left exactly as written.
func TestCrossLinksAbsolutizesOnlyWhatBroke(t *testing.T) {
	src := t.TempDir()
	git(t, src, "init", "-q", "-b", "main")
	put(t, src, "server/main.go", "package main\n")
	put(t, src, "docs/a.md", "see [b](b.md), [spec](../contracts/x/spec.md#s1) and [web](../ui/README.md)\n")
	put(t, src, "docs/b.md", "b\n")
	put(t, src, "contracts/x/spec.md", "spec\n")
	put(t, src, "ui/README.md", "web\n")
	git(t, src, "add", "-A")
	git(t, src, "commit", "-q", "-m", "init")

	m := writeMap(t, strings.Join([]string{
		"server/\tserver\t.\t",
		"docs/a.md\tserver\tdocs/a.md\t",
		"docs/b.md\tserver\tdocs/b.md\t",
		"contracts/\tdoor\tcontracts/\t",
		"ui/\tweb\t.\t",
	}, "\n")+"\n")

	dir := t.TempDir() // the extracted repository
	put(t, dir, "main.go", "package main\n")
	put(t, dir, "docs/a.md", "see [b](b.md), [spec](../contracts/x/spec.md#s1) and [web](../ui/README.md)\n")
	put(t, dir, "docs/b.md", "b\n")

	r := &runner{Options: Options{Repo: pathmap.Server, Dir: dir, Source: src, Ref: "HEAD", Map: m, Log: func(string, ...any) {}}}
	if err := r.crossLinks(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "docs/a.md"))
	want := "see [b](b.md), [spec](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/blob/main/contracts/x/spec.md#s1) and [web](https://github.com/ParkWardRR/cairn-vehicle-web-dashboard/blob/main/README.md)\n"
	if string(got) != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

// A link to a path no rule routes is an error, never a guess.
func TestCrossLinksRefusesUnroutedTargets(t *testing.T) {
	src := t.TempDir()
	git(t, src, "init", "-q", "-b", "main")
	put(t, src, "docs/a.md", "[x](../nowhere/x.md)\n")
	git(t, src, "add", "-A")
	git(t, src, "commit", "-q", "-m", "init")
	m := writeMap(t, "docs/\tserver\tdocs/\t\n")
	dir := t.TempDir()
	put(t, dir, "docs/a.md", "[x](../nowhere/x.md)\n")
	r := &runner{Options: Options{Repo: pathmap.Server, Dir: dir, Source: src, Ref: "HEAD", Map: m, Log: func(string, ...any) {}}}
	if err := r.crossLinks(); err == nil || !strings.Contains(err.Error(), "nowhere") {
		t.Fatalf("want an error naming the link, got %v", err)
	}
}

// An exact-text edit that does not find its text is an error, and changes nothing.
func TestEditFailsLoudlyOnDrift(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "f.sh", "HOST=a\nHOST=a\n")
	r := &runner{Options: Options{Dir: dir}}
	if err := r.edit("f.sh", "HOST=a", "HOST=b", 1); err == nil {
		t.Fatal("two occurrences where one was expected must fail")
	}
	if err := r.edit("f.sh", "HOST=zzz", "HOST=b", 1); err == nil {
		t.Fatal("zero occurrences must fail")
	}
	got, _ := os.ReadFile(filepath.Join(dir, "f.sh"))
	if string(got) != "HOST=a\nHOST=a\n" {
		t.Fatalf("a failed edit changed the file: %q", got)
	}
	if err := r.edit("f.sh", "HOST=a", "HOST=b", 2); err != nil {
		t.Fatal(err)
	}
}

// A step that changes nothing must not commit an empty change.
func TestCommitRefusesAnEmptyStep(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	put(t, dir, "a", "a\n")
	r := &runner{Options: Options{Dir: dir, Author: "t", Email: "t@example.com"}}
	if err := r.commit("first"); err != nil {
		t.Fatal(err)
	}
	if err := r.commit("second"); err == nil {
		t.Fatal("an empty step must be an error")
	}
}
