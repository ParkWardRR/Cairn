// Package modulecheck validates `cairn.module/v1-draft` manifests and query files.
//
// It is the reference implementation of the checks in contracts/module/v1/spec.md §3 —
// the ones a JSON Schema cannot state, which are most of the ones that matter. The
// schema says a field is a string; this says the string names a column that exists.
//
// Two checks here are worth more than the rest put together, because they catch drift
// that no single-repository test can see:
//
//   - `requires.store` is resolved against contracts/store/v1/schema.json, so a module
//     asking for a column the store does not have fails validation rather than a page.
//   - the module schema's engine-field enum is compared with contracts/engine/v1's
//     acquisition enum, so the shared vocabulary cannot silently diverge.
package modulecheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Schema identifiers this package accepts.
const (
	ManifestSchema = "cairn.module/v1-draft"
	QueriesSchema  = "cairn.module-queries/v1-draft"
)

// Manifest is a module manifest. Decoded with unknown fields disallowed, so an extra
// key is an error and not an extension point (spec §2).
type Manifest struct {
	Schema   string   `json:"schema"`
	ID       string   `json:"id"`
	Version  int      `json:"version"`
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Sources  []string `json:"sources"`
	Requires *struct {
		Store        []string `json:"store"`
		EngineFields []string `json:"engine_fields"`
	} `json:"requires,omitempty"`
	Derives []Derive `json:"derives,omitempty"`
	Metrics []Metric `json:"metrics,omitempty"`
	Views   []string `json:"views,omitempty"`
	Queries string   `json:"queries,omitempty"`
	UI      *struct {
		Route string `json:"route"`
		Nav   struct {
			Label string `json:"label"`
			Group string `json:"group"`
			Order int    `json:"order"`
			Icon  string `json:"icon,omitempty"`
		} `json:"nav"`
		VehicleScope string `json:"vehicle_scope"`
	} `json:"ui,omitempty"`
	IOS *struct {
		TripSection *struct {
			Label string `json:"label"`
			Order int    `json:"order"`
		} `json:"trip_section,omitempty"`
	} `json:"ios,omitempty"`
}

// Derive is a column this module defines on a core table.
type Derive struct {
	Column string `json:"column"`
	Type   string `json:"type"`
	Expr   string `json:"expr"`
	Note   string `json:"note,omitempty"`
}

// Metric is one quantity this module contributes to the generic machinery.
type Metric struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Unit         string `json:"unit"`
	Sample       string `json:"sample"`
	SourceView   string `json:"source_view"`
	SourceColumn string `json:"source_column"`
	TripInsight  *struct {
		Label     string `json:"label"`
		Agg       string `json:"agg"`
		Precision int    `json:"precision"`
	} `json:"trip_insight,omitempty"`
	DashboardHighlight bool `json:"dashboard_highlight,omitempty"`
	IOSGauge           *struct {
		Field     string `json:"field"`
		Label     string `json:"label"`
		Unit      string `json:"unit"`
		Precision int    `json:"precision"`
	} `json:"ios_gauge,omitempty"`
}

// QueriesFile is a module's named, parameterised queries.
type QueriesFile struct {
	Schema  string  `json:"schema"`
	Module  string  `json:"module"`
	Queries []Query `json:"queries"`
}

// Query is one named question.
type Query struct {
	Name           string  `json:"name"`
	Summary        string  `json:"summary,omitempty"`
	Params         []Param `json:"params,omitempty"`
	SQL            string  `json:"sql"`
	AgeThresholdMS *int    `json:"age_threshold_ms,omitempty"`
}

// Param is one bound query parameter.
type Param struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required *bool  `json:"required,omitempty"`
	Summary  string `json:"summary,omitempty"`
}

// Store is what contracts/store/v1/schema.json pins: which columns exist.
type Store struct {
	columns map[string]bool // "table.column"
	objects map[string]bool
}

var (
	idRe     = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	colRefRe = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)
	metricRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	viewRe   = regexp.MustCompile(`^v_[a-z][a-z0-9_]*$`)
	routeRe  = regexp.MustCompile(`^/[a-z0-9][a-z0-9-/]*$`)
	qNameRe  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)
	pNameRe  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	// A $name placeholder. Deliberately matches more than pNameRe so that a wrongly
	// cased placeholder is reported as an undeclared parameter rather than not seen.
	placeholderRe = regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
)

var (
	statuses   = set("stub", "derived", "verified")
	samples    = set("sample", "pull", "boot", "trip")
	aggs       = set("min", "max", "avg", "median")
	navGroups  = set("everyday", "detail")
	scopes     = set("all", "single", "none")
	paramTypes = set("vehicle_id", "boot_id", "date", "int", "number", "string")
	duckTypes  = set("BOOLEAN", "TINYINT", "SMALLINT", "INTEGER", "BIGINT", "UTINYINT",
		"USMALLINT", "UINTEGER", "UBIGINT", "FLOAT", "DOUBLE", "VARCHAR", "DATE", "TIMESTAMP")
)

func set(xs ...string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func keys(m map[string]bool) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// Decode reads a manifest, refusing unknown fields.
func Decode(b []byte) (*Manifest, error) {
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeQueries reads a query file, refusing unknown fields.
func DecodeQueries(b []byte) (*QueriesFile, error) {
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	var q QueriesFile
	if err := dec.Decode(&q); err != nil {
		return nil, err
	}
	return &q, nil
}

// LoadStore reads contracts/store/v1/schema.json into the set of columns it pins.
func LoadStore(path string) (*Store, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Only the shape this needs: named objects, each with named columns. The full file
	// is validated by contract-check.
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	s := &Store{columns: map[string]bool{}, objects: map[string]bool{}}
	for _, group := range []string{"tables", "views", "macros"} {
		raw, ok := doc[group]
		if !ok {
			continue
		}
		var objs map[string]struct {
			Columns []struct{ Name string } `json:"columns"`
		}
		if err := json.Unmarshal(raw, &objs); err != nil {
			continue // a macro's extra fields are not this package's business
		}
		for name, o := range objs {
			s.objects[name] = true
			for _, c := range o.Columns {
				s.columns[name+"."+c.Name] = true
			}
		}
	}
	if len(s.columns) == 0 {
		return nil, fmt.Errorf("%s: pins no columns; the shape must have changed", path)
	}
	return s, nil
}

// Has reports whether the store pins a table.column.
func (s *Store) Has(ref string) bool { return s != nil && s.columns[ref] }

// EngineFields reads the `field` enum from contracts/engine/v1/acquisition.schema.json.
// That file is the owner of the vocabulary; this package must not keep its own copy.
func EngineFields(schemaPath string) (map[string]bool, error) {
	b, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Defs struct {
			Field struct {
				Enum []string `json:"enum"`
			} `json:"field"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", schemaPath, err)
	}
	if len(doc.Defs.Field.Enum) == 0 {
		return nil, fmt.Errorf("%s: $defs/field has no enum", schemaPath)
	}
	return set(doc.Defs.Field.Enum...), nil
}

// ModuleSchemaEngineFields reads the engine-field enum the module schema carries, so the
// two can be compared.
func ModuleSchemaEngineFields(schemaPath string) (map[string]bool, error) {
	b, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Defs struct {
			EngineField struct {
				Enum []string `json:"enum"`
			} `json:"engine_field"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", schemaPath, err)
	}
	if len(doc.Defs.EngineField.Enum) == 0 {
		return nil, fmt.Errorf("%s: $defs/engine_field has no enum", schemaPath)
	}
	return set(doc.Defs.EngineField.Enum...), nil
}

// Env is what a manifest is validated against: the store's columns and the engine's
// field vocabulary. Either may be nil, which skips the checks that need it — useful for
// a caller holding a manifest with no checkout to hand, never for CI.
type Env struct {
	Store        *Store
	EngineFields map[string]bool
	// Directory is the module's directory name, which the id must equal. Empty skips it.
	Directory string
}

// Check validates one manifest and returns every problem, most structural first.
func Check(m *Manifest, env Env) []error {
	var errs []error
	add := func(f string, a ...any) { errs = append(errs, fmt.Errorf(f, a...)) }

	if m.Schema != ManifestSchema {
		add("schema %q is not %q", m.Schema, ManifestSchema)
	}
	if !idRe.MatchString(m.ID) {
		add("id %q must be lowercase alphanumeric with single hyphens", m.ID)
	} else if env.Directory != "" && m.ID != env.Directory {
		add("id %q does not match the directory name %q", m.ID, env.Directory)
	}
	if len(m.ID) > 31 {
		add("id %q is longer than 31 characters", m.ID)
	}
	if m.Version < 1 || m.Version > 65535 {
		add("version %d must be between 1 and 65535", m.Version)
	}
	if strings.TrimSpace(m.Name) == "" {
		add("a module needs a name")
	}
	if !statuses[m.Status] {
		add("status %q is not stub, derived or verified", m.Status)
	}
	if len(m.Sources) == 0 {
		add("sources must name at least one source: a claim without one is not reviewable")
	}
	for i, s := range m.Sources {
		if strings.TrimSpace(s) == "" {
			add("sources[%d] is empty", i)
		}
	}

	// A stub names the module and claims nothing. Checked before the content rules so
	// the message is the useful one.
	if m.Status == "stub" {
		switch {
		case len(m.Derives) > 0:
			add("a stub claims nothing, but it derives %d column(s)", len(m.Derives))
		case len(m.Metrics) > 0:
			add("a stub claims nothing, but it states %d metric(s)", len(m.Metrics))
		case m.Requires != nil && len(m.Requires.EngineFields) > 0:
			add("a stub claims nothing, but it requires %d engine field(s)", len(m.Requires.EngineFields))
		}
	}

	if m.Requires != nil {
		for _, ref := range m.Requires.Store {
			if !colRefRe.MatchString(ref) {
				add("requires.store %q must be table.column in lowercase", ref)
				continue
			}
			// A column another module derives is legitimate, so a miss is only an error
			// when nothing in the set provides it. Here, with one manifest in hand, the
			// module's own derivations are the only other source.
			if env.Store != nil && !env.Store.Has(ref) && !derivesColumn(m, ref) {
				add("requires.store %q is not in store/v1 and no module derives it", ref)
			}
		}
		for _, f := range m.Requires.EngineFields {
			if env.EngineFields != nil && !env.EngineFields[f] {
				add("requires.engine_fields %q is not an engine/v1 capture field", f)
			}
		}
	}

	owned := map[string]bool{}
	for i, d := range m.Derives {
		if !colRefRe.MatchString(d.Column) {
			add("derives[%d].column %q must be table.column in lowercase", i, d.Column)
		} else if owned[d.Column] {
			add("derives[%d]: %q is already owned; exactly one derivation owns a column", i, d.Column)
		} else {
			owned[d.Column] = true
			if env.Store != nil {
				if tbl := strings.SplitN(d.Column, ".", 2)[0]; !env.Store.objects[tbl] {
					add("derives[%d]: %q is on %q, which store/v1 does not have", i, d.Column, tbl)
				}
			}
		}
		if !duckTypes[d.Type] {
			add("derives[%d].type %q is not a store/v1 type (%s)", i, d.Type, keys(duckTypes))
		}
		if strings.TrimSpace(d.Expr) == "" {
			add("derives[%d].expr is empty", i)
		}
		if strings.Contains(d.Expr, ";") {
			add("derives[%d].expr holds a semicolon: a derivation is one expression", i)
		}
	}

	declaredViews := map[string]bool{}
	for i, v := range m.Views {
		if err := safeRelPath(v); err != nil {
			add("views[%d] %q: %v", i, v, err)
		}
		declaredViews[v] = true
	}

	// A metric must read a view this module creates, so the metric list and the views
	// cannot drift apart. store/v1's grandfathered views are the exception, because a
	// module taking over an existing page reads them before it owns them.
	grandfathered := map[string]bool{}
	if env.Store != nil {
		for o := range env.Store.objects {
			grandfathered[o] = true
		}
	}
	seenMetric := map[string]bool{}
	for i, mt := range m.Metrics {
		if !metricRe.MatchString(mt.Key) {
			add("metrics[%d].key %q must be lowercase snake_case", i, mt.Key)
		} else if seenMetric[mt.Key] {
			add("metrics[%d].key %q is repeated", i, mt.Key)
		} else {
			seenMetric[mt.Key] = true
		}
		if strings.TrimSpace(mt.Label) == "" {
			add("metrics[%d] (%s) needs a label", i, mt.Key)
		}
		if !samples[mt.Sample] {
			add("metrics[%d] (%s): sample %q is not one of %s", i, mt.Key, mt.Sample, keys(samples))
		}
		if !viewRe.MatchString(mt.SourceView) {
			add("metrics[%d] (%s): source_view %q must be named v_<something>", i, mt.Key, mt.SourceView)
		} else if !grandfathered[mt.SourceView] && !moduleDeclaresView(m, mt.SourceView) {
			add("metrics[%d] (%s): source_view %q is neither in store/v1 nor a view this module declares (name it v_%s_* in one of views[])",
				i, mt.Key, mt.SourceView, strings.ReplaceAll(m.ID, "-", "_"))
		}
		if mt.SourceColumn == "" {
			add("metrics[%d] (%s) needs a source_column", i, mt.Key)
		}
		if ti := mt.TripInsight; ti != nil {
			if !aggs[ti.Agg] {
				add("metrics[%d] (%s): trip_insight.agg %q is not one of %s", i, mt.Key, ti.Agg, keys(aggs))
			}
			if strings.TrimSpace(ti.Label) == "" {
				add("metrics[%d] (%s): trip_insight needs a label", i, mt.Key)
			}
			if ti.Precision < 0 || ti.Precision > 4 {
				add("metrics[%d] (%s): trip_insight.precision %d is outside 0..4", i, mt.Key, ti.Precision)
			}
		}
	}

	if m.Queries != "" {
		if err := safeRelPath(m.Queries); err != nil {
			add("queries %q: %v", m.Queries, err)
		}
	}

	if ui := m.UI; ui != nil {
		if !routeRe.MatchString(ui.Route) {
			add("ui.route %q must be a lowercase path starting with /", ui.Route)
		}
		if !navGroups[ui.Nav.Group] {
			add("ui.nav.group %q is not one of %s", ui.Nav.Group, keys(navGroups))
		}
		if strings.TrimSpace(ui.Nav.Label) == "" {
			add("ui.nav needs a label")
		}
		if ui.Nav.Order < 0 || ui.Nav.Order > 999 {
			add("ui.nav.order %d is outside 0..999", ui.Nav.Order)
		}
		if !scopes[ui.VehicleScope] {
			add("ui.vehicle_scope %q is not one of %s", ui.VehicleScope, keys(scopes))
		}
	}

	return errs
}

func derivesColumn(m *Manifest, ref string) bool {
	for _, d := range m.Derives {
		if d.Column == ref {
			return true
		}
	}
	return false
}

// moduleDeclaresView reports whether any declared SQL file could create the view. With
// only the manifest in hand the file contents are unknown, so this accepts a view named
// for the module; a consumer with the files reads them and is stricter.
func moduleDeclaresView(m *Manifest, view string) bool {
	if len(m.Views) == 0 {
		return false
	}
	return strings.HasPrefix(view, "v_"+strings.ReplaceAll(m.ID, "-", "_")+"_")
}

// safeRelPath refuses anything that could reach outside the module's own directory.
func safeRelPath(p string) error {
	if p == "" {
		return fmt.Errorf("path is empty")
	}
	if strings.HasPrefix(p, "/") {
		return fmt.Errorf("path must be relative to the module directory")
	}
	if p != path.Clean(p) || strings.HasPrefix(path.Clean(p), "..") {
		return fmt.Errorf("path must not leave the module directory")
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." || seg == "." || seg == "" {
			return fmt.Errorf("path must not leave the module directory")
		}
	}
	return nil
}

// CheckQueries validates a query file. module is the owning manifest's id; empty skips
// the ownership check.
func CheckQueries(q *QueriesFile, module string) []error {
	var errs []error
	add := func(f string, a ...any) { errs = append(errs, fmt.Errorf(f, a...)) }

	if q.Schema != QueriesSchema {
		add("schema %q is not %q", q.Schema, QueriesSchema)
	}
	if !idRe.MatchString(q.Module) {
		add("module %q must be lowercase alphanumeric with single hyphens", q.Module)
	} else if module != "" && q.Module != module {
		add("module %q does not match the manifest's module %q", q.Module, module)
	}
	if len(q.Queries) == 0 {
		add("queries must hold at least one query")
	}

	seen := map[string]bool{}
	for i, qq := range q.Queries {
		where := fmt.Sprintf("queries[%d]", i)
		if qq.Name != "" {
			where = fmt.Sprintf("queries[%d] (%s)", i, qq.Name)
		}
		if !qNameRe.MatchString(qq.Name) {
			add("%s: name %q must be lowercase, digits and hyphens", where, qq.Name)
		} else if seen[qq.Name] {
			add("%s: duplicate query name %q", where, qq.Name)
		} else {
			seen[qq.Name] = true
		}

		declared := map[string]bool{}
		for j, p := range qq.Params {
			if !pNameRe.MatchString(p.Name) {
				add("%s: params[%d] name %q must be lowercase snake_case", where, j, p.Name)
			} else if declared[p.Name] {
				add("%s: params[%d] name %q is repeated", where, j, p.Name)
			} else {
				declared[p.Name] = true
			}
			if !paramTypes[p.Type] {
				add("%s: params[%d] (%s) type %q is not one of %s", where, j, p.Name, p.Type, keys(paramTypes))
			}
		}

		sql := strings.TrimSpace(qq.SQL)
		if sql == "" {
			add("%s: sql is empty", where)
			continue
		}
		upper := strings.ToUpper(sql)
		if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
			add("%s: a query must be a SELECT (or a WITH leading to one); nothing a module declares may write to the store", where)
		}
		if strings.Contains(stripStrings(sql), ";") {
			add("%s: sql holds a semicolon outside a string; one statement per query", where)
		}

		// Both directions. A query may neither read an unbound parameter nor silently
		// ignore an argument a caller passed.
		used := map[string]bool{}
		for _, mm := range placeholderRe.FindAllStringSubmatch(stripStrings(sql), -1) {
			used[mm[1]] = true
		}
		for name := range used {
			if !declared[name] {
				add("%s: sql reads $%s, which is not declared in params", where, name)
			}
		}
		for name := range declared {
			if !used[name] {
				add("%s: params declares %s, which does not appear as $%s in the sql", where, name, name)
			}
		}
	}
	return errs
}

// stripStrings blanks single-quoted SQL string literals, so a semicolon or a $ inside
// one is not mistaken for a statement separator or a placeholder. Doubled quotes escape.
func stripStrings(sql string) string {
	out := make([]byte, 0, len(sql))
	in := false
	for i := 0; i < len(sql); i++ {
		c := sql[i]
		if c == '\'' {
			if in && i+1 < len(sql) && sql[i+1] == '\'' {
				out = append(out, ' ', ' ')
				i++
				continue
			}
			in = !in
			out = append(out, ' ')
			continue
		}
		if in {
			out = append(out, ' ')
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

// CheckSet validates a whole module set: per-module checks plus the ones that only make
// sense across modules — one owner per derived column, and no cycle in derivation order.
func CheckSet(ms []*Manifest, env Env) []error {
	var errs []error
	add := func(f string, a ...any) { errs = append(errs, fmt.Errorf(f, a...)) }

	byID := map[string]bool{}
	ownerOf := map[string]string{}
	for _, m := range ms {
		if byID[m.ID] {
			add("two modules share the id %q", m.ID)
		}
		byID[m.ID] = true
		for _, d := range m.Derives {
			if prev, ok := ownerOf[d.Column]; ok {
				add("column %q is derived by both %q and %q; exactly one module owns a column", d.Column, prev, m.ID)
				continue
			}
			ownerOf[d.Column] = m.ID
		}
	}

	// Order modules so a module runs after every module whose derived column it reads.
	// A cycle means no order exists and the store's contents would depend on load order.
	deps := map[string]map[string]bool{}
	for _, m := range ms {
		deps[m.ID] = map[string]bool{}
		if m.Requires == nil {
			continue
		}
		for _, ref := range m.Requires.Store {
			if owner, ok := ownerOf[ref]; ok && owner != m.ID {
				deps[m.ID][owner] = true
			}
		}
	}
	if cycle := findCycle(deps); len(cycle) > 0 {
		add("derivation order has a cycle: %s", strings.Join(cycle, " -> "))
	}
	return errs
}

// findCycle returns one cycle as a path, or nil. Iterative depth-first search with the
// standard three-colour marking; ids are visited in sorted order so the reported cycle
// is the same on every run.
func findCycle(deps map[string]map[string]bool) []string {
	const (
		white = 0
		grey  = 1
		black = 2
	)
	state := map[string]int{}
	var path []string
	var walk func(string) []string
	walk = func(n string) []string {
		state[n] = grey
		path = append(path, n)
		next := make([]string, 0, len(deps[n]))
		for d := range deps[n] {
			next = append(next, d)
		}
		sort.Strings(next)
		for _, d := range next {
			if _, known := deps[d]; !known {
				continue
			}
			switch state[d] {
			case grey:
				for i, p := range path {
					if p == d {
						return append(append([]string{}, path[i:]...), d)
					}
				}
			case white:
				if c := walk(d); c != nil {
					return c
				}
			}
		}
		path = path[:len(path)-1]
		state[n] = black
		return nil
	}
	ids := make([]string, 0, len(deps))
	for id := range deps {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if state[id] == white {
			path = nil
			if c := walk(id); c != nil {
				return c
			}
		}
	}
	return nil
}

// CheckVocabulary compares the engine-field enum the module schema carries with
// engine/v1's own, so the shared vocabulary cannot drift between two files in one
// repository — the exact failure engine/v1 was created to end.
func CheckVocabulary(moduleSchema, engineSchema string) []error {
	mine, err := ModuleSchemaEngineFields(moduleSchema)
	if err != nil {
		return []error{err}
	}
	theirs, err := EngineFields(engineSchema)
	if err != nil {
		return []error{err}
	}
	var errs []error
	for f := range mine {
		if !theirs[f] {
			errs = append(errs, fmt.Errorf("module/v1 lists engine field %q, which engine/v1 does not", f))
		}
	}
	for f := range theirs {
		if !mine[f] {
			errs = append(errs, fmt.Errorf("engine/v1 has capture field %q, which module/v1 does not list", f))
		}
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
	return errs
}

// Vector is one vector file: a bare valid document, or an invalid one wrapped with the
// text its rejection must contain.
type Vector struct {
	Path        string
	ExpectError string
	Directory   string
	// Module is the owning manifest's id, for a query-file vector. Empty skips the
	// ownership check, since a query file alone cannot know who points at it.
	Module    string
	Body      []byte
	IsQueries bool
}

// LoadVectors reads every *.json vector under dir, recursively.
func LoadVectors(dir string) ([]Vector, error) {
	var out []Vector
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".json" {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var wrap struct {
			ExpectError string          `json:"expect_error"`
			Note        string          `json:"note"`
			Directory   string          `json:"directory"`
			Module      string          `json:"module"`
			Manifest    json.RawMessage `json:"manifest"`
			Queries     json.RawMessage `json:"queries_file"`
		}
		_ = json.Unmarshal(b, &wrap)
		v := Vector{Path: p, ExpectError: wrap.ExpectError, Directory: wrap.Directory,
			Module: wrap.Module, Body: b}
		switch {
		case wrap.ExpectError != "" && len(wrap.Queries) > 0:
			v.Body, v.IsQueries = wrap.Queries, true
		case wrap.ExpectError != "" && len(wrap.Manifest) > 0:
			v.Body = wrap.Manifest
		case wrap.ExpectError != "":
			return fmt.Errorf("%s: expect_error without a manifest or queries_file", p)
		default:
			// A bare valid document. Which kind it is comes from its schema field.
			var probe struct{ Schema string }
			_ = json.Unmarshal(b, &probe)
			v.IsQueries = probe.Schema == QueriesSchema
		}
		out = append(out, v)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no vectors", dir)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Run checks one vector and returns why it failed, or "" when it passed.
func (v Vector) Run(env Env) string {
	var errs []error
	if v.IsQueries {
		q, err := DecodeQueries(v.Body)
		if err != nil {
			errs = []error{err}
		} else {
			errs = CheckQueries(q, v.Module)
		}
	} else {
		e := env
		e.Directory = v.Directory
		m, err := Decode(v.Body)
		if err != nil {
			errs = []error{err}
		} else {
			if e.Directory == "" {
				e.Directory = m.ID // a valid vector is not a directory; skip that check
			}
			errs = Check(m, e)
		}
	}

	joined := make([]string, len(errs))
	for i, e := range errs {
		joined[i] = e.Error()
	}
	all := strings.Join(joined, "; ")

	if v.ExpectError == "" {
		if all != "" {
			return "should be accepted, but was rejected: " + all
		}
		return ""
	}
	if all == "" {
		return fmt.Sprintf("should be rejected mentioning %q, but was accepted", v.ExpectError)
	}
	if !strings.Contains(all, v.ExpectError) {
		return fmt.Sprintf("was rejected as %q, which does not mention %q", all, v.ExpectError)
	}
	return ""
}
