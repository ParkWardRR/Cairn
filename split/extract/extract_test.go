package extract_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ParkWardRR/cairn-driving-log-selfhosted/split/extract"
	"github.com/ParkWardRR/cairn-driving-log-selfhosted/split/pathmap"
)

// A small monorepo with everything extraction has to survive: a rename, an
// executable file, a symlink, a merge commit, several authors, a directory that
// exists only in history, and a child directory carved out of its parent.

func run(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func write(t *testing.T, dir, rel, body string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
}

func commit(t *testing.T, dir, msg, who string) {
	t.Helper()
	env := []string{
		"GIT_AUTHOR_NAME=" + who, "GIT_AUTHOR_EMAIL=" + strings.ToLower(who) + "@example.com",
		"GIT_COMMITTER_NAME=" + who, "GIT_COMMITTER_EMAIL=" + strings.ToLower(who) + "@example.com",
	}
	run(t, dir, nil, "add", "-A")
	run(t, dir, env, "commit", "-q", "-m", msg)
}

const fixtureMap = "srv/\tserver\t.\n" +
	"srv/sub/\tweb\ttools/sub/\n" +
	"ui/\tweb\t.\n" +
	"docs/a.md\tserver\tdocs/a.md\n" +
	"docs/b.md\tdoor\tdocs/b.md\n" +
	"README.md\tdoor\tREADME.md\n"

func monorepo(t *testing.T) (string, *pathmap.Map) {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, nil, "init", "-q", "-b", "main")
	run(t, dir, nil, "config", "commit.gpgsign", "false")

	write(t, dir, "README.md", "monorepo\n", 0o644)
	write(t, dir, "srv/main.go", "package main\n", 0o644)
	write(t, dir, "srv/legacy.txt", "gone later\n", 0o644)
	write(t, dir, "legacy/old.txt", "a directory with no rule\n", 0o644)
	write(t, dir, "ui/old.vue", "<template/>\n", 0o644)
	commit(t, dir, "initial", "Alice")

	write(t, dir, "srv/run.sh", "#!/bin/sh\necho hi\n", 0o755)
	if err := os.Symlink("run.sh", filepath.Join(dir, "srv", "link")); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "srv/sub/thing.go", "package sub\n", 0o644)
	write(t, dir, "docs/a.md", "server doc\n", 0o644)
	write(t, dir, "docs/b.md", "door doc\n", 0o644)
	commit(t, dir, "add script, link, sub and docs", "Bob")

	run(t, dir, nil, "mv", "ui/old.vue", "ui/page.vue")
	run(t, dir, nil, "rm", "-q", "srv/legacy.txt")
	commit(t, dir, "rename a page, drop a file", "Alice")

	run(t, dir, nil, "checkout", "-q", "-b", "feature")
	write(t, dir, "ui/feature.vue", "<feature/>\n", 0o644)
	commit(t, dir, "feature", "Carol")
	run(t, dir, nil, "checkout", "-q", "main")
	write(t, dir, "srv/main.go", "package main // changed\n", 0o644)
	commit(t, dir, "change main", "Bob")
	run(t, dir, []string{"GIT_AUTHOR_NAME=Alice", "GIT_AUTHOR_EMAIL=alice@example.com", "GIT_COMMITTER_NAME=Alice", "GIT_COMMITTER_EMAIL=alice@example.com"},
		"merge", "-q", "--no-ff", "-m", "merge feature", "feature")
	run(t, dir, nil, "tag", "monorepo-final")

	p := filepath.Join(t.TempDir(), "paths.tsv")
	if err := os.WriteFile(p, []byte(fixtureMap), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := pathmap.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return dir, m
}

func needFilterRepo(t *testing.T) {
	t.Helper()
	if err := exec.Command("git", "filter-repo", "--version").Run(); err != nil {
		t.Skip("git filter-repo is not installed")
	}
}

func extractTo(t *testing.T, src string, m *pathmap.Map, repo string) (extract.Options, *extract.Report) {
	t.Helper()
	o := extract.Options{Source: src, Ref: "monorepo-final", Repo: repo, Map: m, Out: filepath.Join(t.TempDir(), repo)}
	rep, err := extract.Extract(o)
	if err != nil {
		t.Fatal(err)
	}
	return o, rep
}

func TestExtractionIsFaithfulAndKeepsHistory(t *testing.T) {
	needFilterRepo(t)
	src, m := monorepo(t)

	so, srv := extractTo(t, src, m, pathmap.Server)
	if !srv.OK() {
		t.Fatalf("server did not verify: %v", srv.Problems)
	}
	wo, web := extractTo(t, src, m, pathmap.Web)
	if !web.OK() {
		t.Fatalf("web did not verify: %v", web.Problems)
	}

	// The server holds exactly its files, with the exec bit and the symlink intact.
	ls := run(t, so.Out, nil, "ls-tree", "-r", "HEAD")
	for _, want := range []string{"100755 blob", "120000 blob", "\tmain.go", "\trun.sh", "\tlink", "\tdocs/a.md"} {
		if !strings.Contains(ls, want) {
			t.Errorf("server tree lacks %q:\n%s", want, ls)
		}
	}
	for _, not := range []string{"thing.go", "page.vue", "b.md", "README"} {
		if strings.Contains(ls, not) {
			t.Errorf("server tree contains %q, which belongs elsewhere:\n%s", not, ls)
		}
	}

	// The carved child went to the web repo, renamed.
	wls := run(t, wo.Out, nil, "ls-tree", "-r", "--name-only", "HEAD")
	for _, want := range []string{"tools/sub/thing.go", "page.vue", "feature.vue"} {
		if !strings.Contains(wls, want) {
			t.Errorf("web tree lacks %q:\n%s", want, wls)
		}
	}

	// History: a file deleted long ago is still in the server's history; a directory
	// no rule routes appears nowhere; the rename is followed.
	if h := run(t, so.Out, nil, "log", "--all", "--oneline", "--", "legacy.txt"); strings.TrimSpace(h) == "" {
		t.Error("a historical file was lost from the server's history")
	}
	for _, repo := range []string{so.Out, wo.Out} {
		if all := run(t, repo, nil, "log", "--all", "--name-only", "--format="); strings.Contains(all, "legacy/old.txt") {
			t.Errorf("%s: an unrouted path survived in history", repo)
		}
	}
	if f := run(t, wo.Out, nil, "log", "--follow", "--name-only", "--format=", "--", "page.vue"); !strings.Contains(f, "old.vue") {
		t.Errorf("the rename was not followed in the web history:\n%s", f)
	}

	// Authors are a subset of the source's; the merge survived where it still has content.
	if a := run(t, wo.Out, nil, "log", "--format=%aN"); !strings.Contains(a, "Carol") {
		t.Error("the feature branch's author was lost")
	}
}

func TestExtractionRefusesToOverwriteAndNeverTouchesTheSource(t *testing.T) {
	needFilterRepo(t)
	src, m := monorepo(t)
	before := run(t, src, nil, "rev-parse", "HEAD", "monorepo-final")
	status := run(t, src, nil, "status", "--porcelain")

	o, _ := extractTo(t, src, m, pathmap.Server)
	if _, err := extract.Extract(o); err == nil {
		t.Fatal("extraction overwrote an existing directory")
	}
	if after := run(t, src, nil, "rev-parse", "HEAD", "monorepo-final"); after != before {
		t.Fatalf("the source's refs changed:\n%s\n%s", before, after)
	}
	if s := run(t, src, nil, "status", "--porcelain"); s != status {
		t.Fatalf("the source's working tree changed:\n%s", s)
	}
}

// Verification is only worth anything if it can fail. Each row damages a correct
// extraction in one way and requires Verify to notice.
func TestVerifyCatchesEveryKindOfDamage(t *testing.T) {
	needFilterRepo(t)
	src, m := monorepo(t)

	damage := map[string]func(t *testing.T, out string){
		"a file's content changes": func(t *testing.T, out string) { write(t, out, "main.go", "package main // tampered\n", 0o644) },
		"a file goes missing":      func(t *testing.T, out string) { run(t, out, nil, "rm", "-q", "docs/a.md") },
		"a stray file appears":     func(t *testing.T, out string) { write(t, out, "intruder.txt", "x\n", 0o644) },
		"an exec bit is lost":      func(t *testing.T, out string) { write(t, out, "run.sh", "#!/bin/sh\necho hi\n", 0o644) },
		"a symlink becomes a file": func(t *testing.T, out string) {
			if err := os.Remove(filepath.Join(out, "link")); err != nil {
				t.Fatal(err)
			}
			write(t, out, "link", "run.sh", 0o644)
		},
	}
	for name, hurt := range damage {
		t.Run(name, func(t *testing.T) {
			o, rep := extractTo(t, src, m, pathmap.Server)
			if !rep.OK() {
				t.Fatalf("baseline did not verify: %v", rep.Problems)
			}
			hurt(t, o.Out)
			commit(t, o.Out, "damage", "Mallory")
			again, err := extract.Verify(o)
			if err != nil {
				t.Fatal(err)
			}
			if again.OK() {
				t.Fatal("Verify accepted a damaged extraction")
			}
		})
	}
}
