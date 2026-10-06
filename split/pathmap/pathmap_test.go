package pathmap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "paths.tsv")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const sample = "# a comment\n" +
	"srv/\tserver\t.\n" +
	"srv/sub/\tweb\ttools/sub/\tcarved out of srv\n" +
	"ui/\tweb\t.\n" +
	"docs/a.md\tserver\tdocs/a.md\n" +
	"docs/b.md\tdoor\tdocs/b.md\n"

func load(t *testing.T) *Map {
	t.Helper()
	m, err := Load(write(t, sample))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestLongestMatchWinsSoAChildCanBeCarvedOut(t *testing.T) {
	m := load(t)
	cases := []struct{ path, repo, dest string }{
		{"srv/main.go", Server, "main.go"},
		{"srv/cmd/x/y.go", Server, "cmd/x/y.go"},
		{"srv/sub/z.go", Web, "tools/sub/z.go"},
		{"srv/subtle.go", Server, "subtle.go"}, // "srv/sub/" must not capture "srv/subtle.go"
		{"ui/app/p.vue", Web, "app/p.vue"},
		{"docs/a.md", Server, "docs/a.md"},
		{"docs/b.md", Door, "docs/b.md"},
	}
	for _, c := range cases {
		r, ok := m.Match(c.path)
		if !ok || r.Repo != c.repo || m.DestPath(r, c.path) != c.dest {
			t.Errorf("%s -> %v %q (ok=%v), want %s %q", c.path, r.Repo, m.DestPath(r, c.path), ok, c.repo, c.dest)
		}
	}
}

func TestAnUnroutedPathIsReportedNeverDropped(t *testing.T) {
	m := load(t)
	by, un := m.Coverage([]string{"srv/a.go", "new/file.txt", "docs/a.md", "zzz"})
	if len(un) != 2 || un[0] != "new/file.txt" || un[1] != "zzz" {
		t.Fatalf("uncovered = %v", un)
	}
	if len(by[Server]) != 2 {
		t.Fatalf("server paths = %v", by[Server])
	}
}

func TestADestinationCollisionIsAnError(t *testing.T) {
	m, err := Load(write(t, "a/\tserver\t.\nb/\tserver\t.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Expected(Server, []string{"a/x", "b/x"}); err == nil || !strings.Contains(err.Error(), "both map to") {
		t.Fatalf("err = %v, want a collision", err)
	}
}

func TestBadRowsAreRefused(t *testing.T) {
	for name, body := range map[string]string{
		"unknown repo":      "a/\tnowhere\t.\n",
		"duplicate source":  "a/\tserver\t.\na/\tweb\t.\n",
		"parent traversal":  "../a\tserver\t.\n",
		"absolute":          "/a\tserver\t.\n",
		"too few columns":   "a/\tserver\n",
		"dir needs a slash": "a/\tserver\tx\n",
	} {
		if _, err := Load(write(t, body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestFilterSpecCarvesChildrenOutOfTheirParent(t *testing.T) {
	m := load(t)
	srv := strings.Join(m.FilterSpec(Server), "\n")
	if !strings.Contains(srv, `(?!(?:sub/))`) {
		t.Errorf("the server filter does not exclude the carved child:\n%s", srv)
	}
	if !strings.Contains(srv, "literal:docs/a.md\n") && !strings.HasSuffix(srv, "literal:docs/a.md") {
		t.Errorf("file include missing:\n%s", srv)
	}
	if !strings.Contains(srv, `regex:^srv/(?!(?:sub/)).+$`) {
		t.Errorf("directory include missing: a rename line alone would select nothing:\n%s", srv)
	}
	web := strings.Join(m.FilterSpec(Web), "\n")
	if !strings.Contains(web, `regex:^srv/sub/.+$`) || !strings.Contains(web, `regex:^srv/sub/(.+)$==>tools/sub/\1`) || !strings.Contains(web, `regex:^ui/(.+)$==>\1`) {
		t.Errorf("web filter wrong:\n%s", web)
	}
	if strings.Contains(web, "docs/b.md") {
		t.Error("a front-door file leaked into the web filter")
	}
}

// The real map must always be loadable and self-consistent.
func TestTheRealMapLoads(t *testing.T) {
	m, err := Load("../paths.tsv")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Rules) < 30 {
		t.Fatalf("only %d rules", len(m.Rules))
	}
	for _, r := range Extracted {
		if len(m.FilterSpec(r)) == 0 {
			t.Errorf("no rules route anything to %s", r)
		}
	}
}
