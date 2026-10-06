// Package rewrite applies, to an extracted repository, the changes that make it stand on
// its own: provenance, its own Go module path, links that crossed a boundary, deploy
// scripts that no longer assume the monorepo layout, and the files every repository
// needs (README, licence, ignore rules, the contracts pin, CI).
//
// Each step is ONE commit on top of the extracted history, so `git log` shows exactly
// what was rewritten and nothing else was touched. A step that finds nothing to do, or
// whose expected text is not where it expected, is an error: a silent no-op would leave
// a repository half-rewritten while looking finished.
package rewrite

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/ParkWardRR/Cairn/split/links"
	"github.com/ParkWardRR/Cairn/split/pathmap"
)

//go:embed templates/*
var templates embed.FS

// Owner is the GitHub account the repositories live under.
const Owner = "ParkWardRR"

// Names are the GitHub names of the repositories. The front door is still called Cairn
// until the last phase renames it; a link to the old name redirects afterwards.
var Names = map[string]string{
	pathmap.Server:   "cairn-vehicle-server",
	pathmap.Web:      "cairn-vehicle-web-dashboard",
	pathmap.Firmware: "cairn-esp32-device-firmware",
	pathmap.Door:     "Cairn",
}

// Protocols each repository is pinned to, as the body of the "protocols" JSON object.
var protocols = map[string]string{
	pathmap.Server:   `"format": "v3", "enrolment": "v1", "sync": "v1", "ble": "v1"`,
	pathmap.Firmware: `"format": "v3", "enrolment": "v1", "ble": "v1"`,
	pathmap.Web:      `"store": "v1"`,
}

// Options describes one rewrite.
type Options struct {
	Repo   string // server | web | firmware
	Dir    string // the extracted repository
	Source string // the monorepo checkout
	Ref    string // the tag the extraction was made from
	Map    *pathmap.Map

	// ContractsTag is the tag of the contracts release to pin. Until the first
	// contracts release exists this is the monorepo tag, which holds contracts/.
	ContractsTag string

	Author, Email string
	Log           func(format string, args ...any)
}

// errNothingToDo is returned by a step that found, correctly, nothing to change in this
// repository (as opposed to a step that failed to find what it expected).
var errNothingToDo = fmt.Errorf("nothing to do")

type step struct {
	subject string
	// summary is the line MIGRATION.md gives this step.
	summary string
	only    []string // repos this applies to; empty means all
	run     func(*runner) error
}

type runner struct {
	Options
	sha string // the source commit
}

// Steps returns the subjects of the rewrite commits, in order, for a repository.
func Steps(repo string) []string {
	var out []string
	for _, s := range steps() {
		if s.applies(repo) {
			out = append(out, s.subject)
		}
	}
	return out
}

func (s step) applies(repo string) bool {
	if len(s.only) == 0 {
		return true
	}
	for _, r := range s.only {
		if r == repo {
			return true
		}
	}
	return false
}

func steps() []step {
	return []step{
		{subject: "docs: record where this repository came from", summary: "this file", run: (*runner).migration},
		{subject: "build: rename the Go module to this repository's path",
			summary: "the Go module path and every import of it", only: []string{pathmap.Server, pathmap.Web}, run: (*runner).goModules},
		{subject: "docs: point links that crossed a repository boundary at the repository that owns them",
			summary: "relative links to files that now live in another repository became absolute links", run: (*runner).crossLinks},
		{subject: "deploy: take the target host from the environment, not the repository",
			summary: "no real host name is stored; the deploy scripts read one from the environment or a gitignored `deploy.env`, and deploy this repository's own `HEAD`",
			only:    []string{pathmap.Web}, run: (*runner).webDeploy},
		{subject: "deploy: sync from this repository's root; add --build-only",
			summary: "`deploy/deploy-v3.sh` syncs the repository root (the server used to be a subdirectory) and can stop after the build",
			only:    []string{pathmap.Server}, run: (*runner).serverDeploy},
		{subject: "docs: add a README and the licence", summary: "README and LICENSE", run: (*runner).readme},
		{subject: "chore: ignore rules for this repository", summary: "`.gitignore`", run: (*runner).gitignore},
		{subject: "build: pin the contracts (contracts.lock and its fetch script)",
			summary: "`contracts.lock` and `scripts/fetch-contracts.sh`: the protocol specs and vectors are fetched at a pinned tag and commit, not copied", run: (*runner).contractsPin},
		{subject: "ci: run on the self-hosted runner under the trusted-code policy",
			summary: "`.github/workflows/ci.yml`, `tests/check-runners.sh` and its self-test", run: (*runner).ci},
	}
}

// Run applies every step that applies to o.Repo.
func Run(o Options) error {
	if o.Log == nil {
		o.Log = func(string, ...any) {}
	}
	sha, err := gitOut(o.Source, "rev-parse", o.Ref+"^{commit}")
	if err != nil {
		return err
	}
	r := &runner{Options: o, sha: strings.TrimSpace(sha)}
	for _, s := range steps() {
		if !s.applies(o.Repo) {
			continue
		}
		if err := s.run(r); err == errNothingToDo {
			o.Log("  (skipped: %s; nothing to do)", s.subject)
			continue
		} else if err != nil {
			return fmt.Errorf("%s: %w", s.subject, err)
		}
		if err := r.commit(s.subject); err != nil {
			return fmt.Errorf("%s: %w", s.subject, err)
		}
		o.Log("  %s", s.subject)
	}
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

func (r *runner) commit(subject string) error {
	if _, err := gitOut(r.Dir, "add", "-A"); err != nil {
		return err
	}
	if out, _ := gitOut(r.Dir, "diff", "--cached", "--name-only"); strings.TrimSpace(out) == "" {
		return fmt.Errorf("the step changed nothing")
	}
	_, err := gitOut(r.Dir, "-c", "user.name="+r.Author, "-c", "user.email="+r.Email,
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", subject)
	return err
}

func (r *runner) path(p string) string { return filepath.Join(r.Dir, filepath.FromSlash(p)) }

func (r *runner) read(p string) (string, error) {
	b, err := os.ReadFile(r.path(p))
	return string(b), err
}

func (r *runner) write(p, content string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(r.path(p)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(r.path(p), []byte(content), mode)
}

// edit replaces old with new in a file, which must contain it exactly want times.
func (r *runner) edit(p, old, new string, want int) error {
	s, err := r.read(p)
	if err != nil {
		return err
	}
	if n := strings.Count(s, old); n != want {
		return fmt.Errorf("%s: expected %d occurrence(s) of %q, found %d (the source drifted from this rewrite)", p, want, firstLine(old), n)
	}
	return r.write(p, strings.ReplaceAll(s, old, new), fileMode(r.path(p)))
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + "…"
	}
	return s
}

func fileMode(p string) os.FileMode {
	if st, err := os.Stat(p); err == nil {
		return st.Mode().Perm()
	}
	return 0o644
}

func (r *runner) tmpl(name string, data any) (string, error) {
	b, err := templates.ReadFile("templates/" + name)
	if err != nil {
		return "", err
	}
	t, err := template.New(name).Delims("[[", "]]").Parse(string(b))
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (r *runner) static(name string) (string, error) {
	b, err := templates.ReadFile("templates/" + name)
	return string(b), err
}

// walk calls fn for every regular file of the repository, skipping tooling directories.
func (r *runner) walk(fn func(rel string) error) error {
	skip := map[string]bool{".git": true, "node_modules": true, ".pio": true, ".contracts": true, "target": true, ".nuxt": true, ".output": true}
	return filepath.WalkDir(r.Dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(r.Dir, p)
		return fn(filepath.ToSlash(rel))
	})
}

func isText(b []byte) bool { return !bytes.Contains(b[:min(len(b), 8000)], []byte{0}) }

// ─── steps ───────────────────────────────────────────────────────────────────

func (r *runner) migration() error {
	fr, _ := exec.Command("git", "filter-repo", "--version").Output()
	var rules strings.Builder
	rules.WriteString("| Source path (in the monorepo) | Path here |\n|---|---|\n")
	for _, rule := range r.Map.Rules {
		if rule.Repo != r.Repo {
			continue
		}
		dest := "`" + rule.Dest + "`"
		if rule.Dest == "." {
			dest = "the same path"
			if rule.IsDir() {
				dest = "the directory's contents become the repository root"
			}
		}
		fmt.Fprintf(&rules, "| `%s` | %s |\n", rule.Source, dest)
	}

	var sum strings.Builder
	for _, s := range steps() {
		if s.applies(r.Repo) {
			fmt.Fprintf(&sum, "- **%s**: %s\n", s.subject, s.summary)
		}
	}
	content, err := r.tmpl("MIGRATION.md.tmpl", map[string]string{
		"SourceURL":  fmt.Sprintf("https://github.com/%s/%s", Owner, Names[pathmap.Door]),
		"Ref":        r.Ref,
		"SourceSHA":  r.sha,
		"FilterRepo": strings.TrimSpace(string(fr)),
		"Rules":      rules.String(),
		"Rewrites":   sum.String(),
	})
	if err != nil {
		return err
	}
	return r.write("MIGRATION.md", content, 0o644)
}

var modLine = regexp.MustCompile(`(?m)^module\s+(\S+)\s*$`)

func (r *runner) goModules() error {
	type rename struct{ old, new string }
	var renames []rename
	if err := r.walk(func(rel string) error {
		if path.Base(rel) != "go.mod" {
			return nil
		}
		s, err := r.read(rel)
		if err != nil {
			return err
		}
		m := modLine.FindStringSubmatch(s)
		if m == nil {
			return fmt.Errorf("%s has no module line", rel)
		}
		dir := path.Dir(rel)
		nw := fmt.Sprintf("github.com/%s/%s", Owner, Names[r.Repo])
		if dir != "." {
			nw += "/" + dir
		}
		if m[1] != nw {
			renames = append(renames, rename{m[1], nw})
		}
		return nil
	}); err != nil {
		return err
	}
	if len(renames) == 0 {
		return fmt.Errorf("no Go module needed renaming")
	}
	// the longest old path first, so one module's path is never a prefix of another's
	sort.Slice(renames, func(i, j int) bool { return len(renames[i].old) > len(renames[j].old) })

	changed := 0
	err := r.walk(func(rel string) error {
		b, err := os.ReadFile(r.path(rel))
		if err != nil || !isText(b) || strings.HasSuffix(rel, "go.sum") || strings.HasSuffix(rel, "package-lock.json") {
			return err
		}
		s := string(b)
		o := s
		for _, rn := range renames {
			s = strings.ReplaceAll(s, rn.old, rn.new)
		}
		if s != o {
			changed++
			return r.write(rel, s, fileMode(r.path(rel)))
		}
		return nil
	})
	if err != nil {
		return err
	}
	// what still names the front door is a URL, and is correct as it is
	var left []string
	_ = r.walk(func(rel string) error {
		b, _ := os.ReadFile(r.path(rel))
		if isText(b) && bytes.Contains(b, []byte(Owner+"/Cairn")) {
			left = append(left, rel)
		}
		return nil
	})
	r.Log("    module: %d files rewritten; %d still name the front door (URLs)", changed, len(left))
	return nil
}

func (r *runner) crossLinks() error {
	out, err := gitOut(r.Source, "ls-tree", "-r", "--name-only", r.Ref)
	if err != nil {
		return err
	}
	exp, err := r.Map.Expected(r.Repo, strings.Split(strings.TrimSpace(out), "\n"))
	if err != nil {
		return err
	}
	srcOf := exp // dest -> source

	var unresolved []string
	rewritten := 0
	err = r.walk(func(rel string) error {
		if !strings.HasSuffix(rel, ".md") {
			return nil
		}
		srcFile, ok := srcOf[rel]
		if !ok {
			return nil // added by an earlier step; its links are written for this repository
		}
		text, err := r.read(rel)
		if err != nil {
			return err
		}
		next := links.Map(text, func(t string) string {
			p, suffix := links.SplitTarget(t)
			if p == "" {
				return t
			}
			if _, err := os.Stat(r.path(path.Join(path.Dir(rel), p))); err == nil {
				return t
			}
			srcTarget := path.Clean(path.Join(path.Dir(srcFile), p))
			rule, ok := r.Map.Match(srcTarget)
			if !ok {
				if rule, ok = r.Map.Match(srcTarget + "/"); !ok {
					unresolved = append(unresolved, rel+": "+t)
					return t
				}
			}
			dest := r.Map.DestPath(rule, srcTarget)
			switch rule.Repo {
			case r.Repo:
				if rel2, err := relPath(path.Dir(rel), dest); err == nil {
					rewritten++
					return rel2 + suffix
				}
			case pathmap.Archive:
				unresolved = append(unresolved, rel+": "+t+" (archive-only)")
				return t
			default:
				kind := "blob"
				if strings.HasSuffix(p, "/") || strings.HasSuffix(rule.Source, "/") && dest == strings.TrimSuffix(rule.Dest, "/") {
					kind = "tree"
				}
				rewritten++
				return fmt.Sprintf("https://github.com/%s/%s/%s/main/%s%s", Owner, Names[rule.Repo], kind, strings.TrimSuffix(dest, "/"), suffix)
			}
			unresolved = append(unresolved, rel+": "+t)
			return t
		})
		if next != text {
			return r.write(rel, next, fileMode(r.path(rel)))
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(unresolved) > 0 {
		return fmt.Errorf("links that resolve nowhere:\n  %s", strings.Join(unresolved, "\n  "))
	}
	var files []string
	_ = r.walk(func(rel string) error {
		if strings.HasSuffix(rel, ".md") {
			files = append(files, rel)
		}
		return nil
	})
	bad, err := links.Check(r.Dir, files)
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		var b strings.Builder
		for _, x := range bad {
			b.WriteString("\n  " + x.String())
		}
		return fmt.Errorf("broken relative links remain:%s", b.String())
	}
	if rewritten == 0 {
		return errNothingToDo
	}
	r.Log("    links: %d rewritten", rewritten)
	return nil
}

func relPath(fromDir, to string) (string, error) {
	if fromDir == "" {
		fromDir = "."
	}
	rel, err := filepath.Rel(filepath.FromSlash(fromDir), filepath.FromSlash(to))
	return filepath.ToSlash(rel), err
}

func (r *runner) webDeploy() error {
	const hostLoad = `# The target comes from the environment or a gitignored deploy.env. No real host name
# is stored in this repository.
[ -f "$ROOT/deploy.env" ] && . "$ROOT/deploy.env"
HOST="${CAIRN_DEPLOY_HOST:?set CAIRN_DEPLOY_HOST (user@host), or put it in deploy.env}"
`
	d := "deploy/deploy-ui.sh"
	if err := r.edit(d, "HOST=\"alfa@cairn.alpina.casa\"\nUI_DIR=\"/srv/cairn-ui\"\nSCRIPT_DIR=\"$(cd \"$(dirname \"$0\")\" && pwd)\"\nROOT=\"$(cd \"$SCRIPT_DIR/..\" && pwd)\"\n",
		"UI_DIR=\"/srv/cairn-ui\"\nSCRIPT_DIR=\"$(cd \"$(dirname \"$0\")\" && pwd)\"\nROOT=\"$(cd \"$SCRIPT_DIR/..\" && pwd)\"\n"+hostLoad, 1); err != nil {
		return err
	}
	// this repository IS the web layer: its own root is what gets built
	for _, e := range [][2]string{
		{`BUILD_DIR="$ROOT/ui"`, `BUILD_DIR="$ROOT"`},
		{`git -C "$ROOT" archive HEAD ui | tar -x -C "$TMP"`, `git -C "$ROOT" archive HEAD | tar -x -C "$TMP"`},
		{`BUILD_DIR="$TMP/ui"`, `BUILD_DIR="$TMP"`},
		{`cp -Rc "$ROOT/ui/node_modules" "$BUILD_DIR/node_modules" 2>/dev/null || cp -R "$ROOT/ui/node_modules" "$BUILD_DIR/node_modules"`,
			`cp -Rc "$ROOT/node_modules" "$BUILD_DIR/node_modules" 2>/dev/null || cp -R "$ROOT/node_modules" "$BUILD_DIR/node_modules"`},
		{`if [ -n "$(git -C "$ROOT" status --porcelain -- ui)" ]; then`, `if [ -n "$(git -C "$ROOT" status --porcelain)" ]; then`},
		{`note: ui/ has uncommitted changes; they are NOT in this deploy`, `note: the working tree has uncommitted changes; they are NOT in this deploy`},
		{`echo "==> Done. UI available at https://cairn.alpina.casa"`, `echo "==> Done."`},
	} {
		if err := r.edit(d, e[0], e[1], 1); err != nil {
			return err
		}
	}
	// the live Caddy site is host-specific: only an operator's own (gitignored) file is installed
	if err := r.edit(d, "echo \"==> Updating Caddy config...\"\nscp \"$SCRIPT_DIR/caddy/Caddyfile\" \"$HOST:/tmp/Caddyfile\"\nssh \"$HOST\" \"sudo mv /tmp/Caddyfile /etc/caddy/Caddyfile && sudo systemctl reload caddy\"\n",
		"if [ -f \"$SCRIPT_DIR/caddy/Caddyfile\" ]; then\n  echo \"==> Updating Caddy config...\"\n  scp \"$SCRIPT_DIR/caddy/Caddyfile\" \"$HOST:/tmp/Caddyfile\"\n  ssh \"$HOST\" \"sudo mv /tmp/Caddyfile /etc/caddy/Caddyfile && sudo systemctl reload caddy\"\nelse\n  echo \"==> No deploy/caddy/Caddyfile here (copy Caddyfile.example and set your site); leaving Caddy alone\"\nfi\n", 1); err != nil {
		return err
	}

	b := "deploy/backup-places.sh"
	if err := r.edit(b, "HOST=\"${CAIRN_HOST:-alfa@cairn.alpina.casa}\"\n",
		"[ -f \"$(dirname \"$0\")/../deploy.env\" ] && . \"$(dirname \"$0\")/../deploy.env\"\nHOST=\"${CAIRN_HOST:-${CAIRN_DEPLOY_HOST:?set CAIRN_HOST (user@host), or CAIRN_DEPLOY_HOST in deploy.env}}\"\n", 1); err != nil {
		return err
	}

	// the site address is a placeholder; the real file is the operator's own
	if err := r.edit("deploy/caddy/Caddyfile", "cairn.alpina.casa {", "cairn.example.lan {", 1); err != nil {
		return err
	}
	if _, err := gitOut(r.Dir, "mv", "deploy/caddy/Caddyfile", "deploy/caddy/Caddyfile.example"); err != nil {
		return err
	}
	var left []string
	_ = r.walk(func(rel string) error {
		b, _ := os.ReadFile(r.path(rel))
		if isText(b) && bytes.Contains(b, []byte("alpina.casa")) {
			left = append(left, rel)
		}
		return nil
	})
	if len(left) > 0 {
		return fmt.Errorf("a real host name remains in: %s", strings.Join(left, ", "))
	}
	return nil
}

func (r *runner) serverDeploy() error {
	d := "deploy/deploy-v3.sh"
	for _, e := range [][2]string{
		{"#   deploy/deploy-v3.sh <user@host> [--reset-v2-data]\n", "#   deploy/deploy-v3.sh <user@host> [--reset-v2-data] [--build-only]\n#\n# --build-only syncs and builds on the host, then stops: nothing is installed and\n# nothing is restarted, so it is safe to run against a host that is serving.\n"},
		{"#   1. rsync server/ and deploy/ to ~/cairn-v3 on the host", "#   1. rsync this repository (and deploy/) to ~/cairn-v3 on the host"},
		{"HOST=\"${1:?usage: deploy-v3.sh <user@host> [--reset-v2-data]}\"\nRESET=0; [ \"${2:-}\" = \"--reset-v2-data\" ] && RESET=1\n",
			"HOST=\"${1:?usage: deploy-v3.sh <user@host> [--reset-v2-data] [--build-only]}\"\nshift\nRESET=0; BUILD_ONLY=0\nfor a in \"$@\"; do\n  case \"$a\" in\n    --reset-v2-data) RESET=1 ;;\n    --build-only) BUILD_ONLY=1 ;;\n    *) echo \"unknown option: $a\" >&2; exit 2 ;;\n  esac\ndone\n"},
		{"  \"$ROOT/server/\" \"$HOST:cairn-v3/server/\"", "  --exclude deploy --exclude .contracts \\\n  \"$ROOT/\" \"$HOST:cairn-v3/server/\""},
		{"echo \"==> installing binaries\"\n", "if [ \"$BUILD_ONLY\" = 1 ]; then\n  echo \"==> built; --build-only, so nothing was installed or restarted\"\n  exit 0\nfi\n\necho \"==> installing binaries\"\n"},
	} {
		if err := r.edit(d, e[0], e[1], 1); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) readme() error {
	if r.Repo != pathmap.Web {
		if _, err := os.Stat(r.path("README.md")); err == nil {
			return fmt.Errorf("README.md already exists")
		}
	}
	s, err := r.static("README-" + r.Repo + ".md")
	if err != nil {
		return err
	}
	if err := r.write("README.md", s, 0o644); err != nil {
		return err
	}
	lic, err := gitOut(r.Source, "show", r.Ref+":LICENSE")
	if err != nil {
		return err
	}
	return r.write("LICENSE", lic, 0o644)
}

func (r *runner) gitignore() error {
	switch r.Repo {
	case pathmap.Web:
		cur, err := r.read(".gitignore")
		if err != nil {
			return err
		}
		add, _ := r.static("gitignore-web-append")
		return r.write(".gitignore", cur+add, 0o644)
	default:
		if _, err := os.Stat(r.path(".gitignore")); err == nil {
			return fmt.Errorf(".gitignore already exists")
		}
		s, err := r.static("gitignore-" + r.Repo)
		if err != nil {
			return err
		}
		return r.write(".gitignore", s, 0o644)
	}
}

func (r *runner) contractsPin() error {
	commit, err := gitOut(r.Source, "rev-parse", r.ContractsTag+"^{commit}")
	if err != nil {
		return err
	}
	lock, err := r.tmpl("contracts.lock.tmpl", map[string]string{
		"ContractsRepo":   fmt.Sprintf("https://github.com/%s/%s", Owner, Names[pathmap.Door]),
		"ContractsTag":    r.ContractsTag,
		"ContractsCommit": strings.TrimSpace(commit),
		"Protocols":       " " + protocols[r.Repo] + " ",
	})
	if err != nil {
		return err
	}
	if err := r.write("contracts.lock", lock, 0o644); err != nil {
		return err
	}
	f, err := r.static("fetch-contracts.sh")
	if err != nil {
		return err
	}
	return r.write("scripts/fetch-contracts.sh", f, 0o755)
}

func (r *runner) ci() error {
	for _, f := range []string{"tests/check-runners.sh", "tests/check-runners-selftest.sh"} {
		if _, err := os.Stat(r.path(f)); err == nil {
			continue // the path map already carries it (the server)
		}
		s, err := gitOut(r.Source, "show", r.Ref+":"+f)
		if err != nil {
			return err
		}
		if err := r.write(f, s, 0o755); err != nil {
			return err
		}
	}
	wf, err := r.static("ci-" + r.Repo + ".yml")
	if err != nil {
		return err
	}
	return r.write(".github/workflows/ci.yml", wf, 0o644)
}
