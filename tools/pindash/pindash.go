// Package pindash builds the table of what each repository is pinned to, from the lock
// files those repositories already carry, so the front door's README never has a table
// somebody edited by hand.
package pindash

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Markers delimit the generated block in a markdown file.
const (
	Start = "<!-- pins:start -->"
	End   = "<!-- pins:end -->"
)

type pin struct {
	Repo   string            `json:"repo"`
	Tag    string            `json:"tag"`
	Commit string            `json:"commit"`
	Protos map[string]string `json:"protocols"`
	Needs  map[string]string `json:"requires"`
}

// Releases is the contract releases that exist, oldest first, as tag names. Render uses it
// to say how far behind a pin is. Nil means "unknown", and the column is then omitted
// rather than guessed: a wrong "up to date" is worse than no answer.
type Releases []string

// Behind reports how many releases were cut after tag. It returns false when the tag is
// not one of the releases at all, which is a pin that was never a release or a tag that
// has been deleted -- either way a thing to look at, not a number.
func (r Releases) Behind(tag string) (int, bool) {
	for i, t := range r {
		if t == tag {
			return len(r) - 1 - i, true
		}
	}
	return 0, false
}

// Repo is one repository's lock files, as raw JSON (a missing file is nil).
type Repo struct {
	Name      string // display name
	URL       string // https URL of the repository
	Contracts []byte // contracts.lock
	Server    []byte // server.lock
	Interop   []byte // interop.lock
}

func short(c string) string {
	if len(c) > 7 {
		return c[:7]
	}
	return c
}

func parse(b []byte, key string) (*pin, error) {
	if b == nil {
		return nil, nil
	}
	var m map[string]pin
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	p, ok := m[key]
	if !ok {
		return nil, fmt.Errorf("no %q object", key)
	}
	return &p, nil
}

func protocols(p *pin) string {
	var ks []string
	for k := range p.Protos {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var out []string
	for _, k := range ks {
		out = append(out, k+" "+p.Protos[k])
	}
	return strings.Join(out, ", ")
}

// Render returns the markdown table (without the markers). releases may be nil.
func Render(repos []Repo, releases Releases) (string, error) {
	var b strings.Builder
	head := "| Repository | Contracts it is pinned to | Protocols | Also pinned |\n|---|---|---|---|\n"
	if len(releases) > 0 {
		head = "| Repository | Contracts it is pinned to | Behind | Protocols | Also pinned |\n|---|---|---|---|---|\n"
	}
	b.WriteString(head)
	for _, r := range repos {
		c, err := parse(r.Contracts, "contracts")
		if err != nil {
			return "", fmt.Errorf("%s contracts.lock: %w", r.Name, err)
		}
		if c == nil {
			return "", fmt.Errorf("%s has no contracts.lock", r.Name)
		}
		pinned := fmt.Sprintf("`%s` (`%s`)", c.Tag, short(c.Commit))
		var also []string
		if s, err := parse(r.Server, "server"); err != nil {
			return "", fmt.Errorf("%s server.lock: %w", r.Name, err)
		} else if s != nil {
			also = append(also, fmt.Sprintf("vehicle server `%s`", short(s.Commit)))
		}
		if i, err := parse(r.Interop, "firmware"); err != nil {
			return "", fmt.Errorf("%s interop.lock: %w", r.Name, err)
		} else if i != nil {
			also = append(also, fmt.Sprintf("firmware `%s` (interop)", short(i.Commit)))
		}
		other := "none"
		if len(also) > 0 {
			other = strings.Join(also, "; ")
		}
		if len(releases) == 0 {
			fmt.Fprintf(&b, "| [%s](%s) | %s | %s | %s |\n", r.Name, r.URL, pinned, protocols(c), other)
			continue
		}
		fmt.Fprintf(&b, "| [%s](%s) | %s | %s | %s | %s |\n",
			r.Name, r.URL, pinned, behind(releases, c.Tag), protocols(c), other)
	}
	return b.String(), nil
}

// behind renders the drift as something a reader acts on. "current" rather than "0", and a
// count rather than a tick, because the whole failure this column exists to catch is a pin
// nobody looked at: the dashboard sat four releases behind for days.
func behind(releases Releases, tag string) string {
	n, ok := releases.Behind(tag)
	switch {
	case !ok:
		return "**not a release**"
	case n == 0:
		return "current"
	case n == 1:
		return "**1 release**"
	default:
		return fmt.Sprintf("**%d releases**", n)
	}
}

// Replace puts table between the markers of doc. It reports whether doc changed.
func Replace(doc, table string) (string, bool, error) {
	i := strings.Index(doc, Start)
	j := strings.Index(doc, End)
	if i < 0 || j < 0 || j < i {
		return "", false, fmt.Errorf("the document has no %s ... %s block", Start, End)
	}
	out := doc[:i+len(Start)] + "\n" + table + doc[j:]
	return out, out != doc, nil
}
