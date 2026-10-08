package modulecheck

import (
	"path/filepath"
	"strings"
	"testing"
)

func contractsDir() string { return filepath.Join("..", "..", "contracts") }

func testEnv(t *testing.T) Env {
	t.Helper()
	store, err := LoadStore(filepath.Join(contractsDir(), "store", "v1", "schema.json"))
	if err != nil {
		t.Fatalf("store/v1: %v", err)
	}
	fields, err := EngineFields(filepath.Join(contractsDir(), "engine", "v1", "acquisition.schema.json"))
	if err != nil {
		t.Fatalf("engine/v1: %v", err)
	}
	return Env{Store: store, EngineFields: fields}
}

// TestVectors runs every committed vector. Same thing cmd/module-check does, here so
// `go test ./...` covers it.
func TestVectors(t *testing.T) {
	env := testEnv(t)
	vectors, err := LoadVectors(filepath.Join(contractsDir(), "module", "v1", "vectors"))
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) < 20 {
		t.Fatalf("only %d vectors; the directory looks wrong", len(vectors))
	}
	valid := 0
	for _, v := range vectors {
		if v.ExpectError == "" {
			valid++
		}
		if why := v.Run(env); why != "" {
			rel, _ := filepath.Rel(contractsDir(), v.Path)
			t.Errorf("%s: %s", rel, why)
		}
	}
	if valid == 0 {
		t.Error("no valid vectors; a checker only ever shown bad input has not been tested")
	}
}

// TestVocabularyMatchesEngineV1 is the drift guard. module/v1 and engine/v1 each carry
// the capture-field enum, and engine/v1 exists precisely because two files in two places
// drifted. If this fails, add the field to both or neither.
func TestVocabularyMatchesEngineV1(t *testing.T) {
	errs := CheckVocabulary(
		filepath.Join(contractsDir(), "module", "v1", "module.schema.json"),
		filepath.Join(contractsDir(), "engine", "v1", "acquisition.schema.json"))
	for _, err := range errs {
		t.Error(err)
	}
}

// TestValidVectorsFormAUsableSet: the valid manifests must also work together, which is
// where the cross-module rules live.
func TestValidVectorsFormAUsableSet(t *testing.T) {
	env := testEnv(t)
	vectors, err := LoadVectors(filepath.Join(contractsDir(), "module", "v1", "vectors"))
	if err != nil {
		t.Fatal(err)
	}
	var ms []*Manifest
	for _, v := range vectors {
		if v.ExpectError != "" || v.IsQueries {
			continue
		}
		m, err := Decode(v.Body)
		if err != nil {
			t.Fatalf("%s: %v", v.Path, err)
		}
		ms = append(ms, m)
	}
	for _, err := range CheckSet(ms, env) {
		t.Error(err)
	}
}

// stub builds a minimal manifest for the set-level tests.
func stub(id string, derives []Derive, requires []string) *Manifest {
	m := &Manifest{
		Schema: ManifestSchema, ID: id, Version: 1, Name: id, Status: "derived",
		Sources: []string{"a fixture"}, Derives: derives,
	}
	if len(requires) > 0 {
		m.Requires = &struct {
			Store        []string `json:"store"`
			EngineFields []string `json:"engine_fields"`
		}{Store: requires}
	}
	return m
}

// TestTwoModulesCannotOwnOneColumn: two owners would make the value depend on load
// order, which is the thing invariant 5 cannot tolerate.
func TestTwoModulesCannotOwnOneColumn(t *testing.T) {
	a := stub("alpha", []Derive{{Column: "boost.boost_psi", Type: "DOUBLE", Expr: "1"}}, nil)
	b := stub("beta", []Derive{{Column: "boost.boost_psi", Type: "DOUBLE", Expr: "2"}}, nil)
	errs := CheckSet([]*Manifest{a, b}, Env{})
	if len(errs) == 0 {
		t.Fatal("two owners of boost.boost_psi was accepted")
	}
	if !strings.Contains(errs[0].Error(), "exactly one module owns") {
		t.Fatalf("wrong reason: %v", errs[0])
	}
}

// TestDerivationCycleIsRejected: alpha reads what beta derives and beta reads what alpha
// derives, so no load order exists.
func TestDerivationCycleIsRejected(t *testing.T) {
	a := stub("alpha",
		[]Derive{{Column: "boost.a_col", Type: "DOUBLE", Expr: "1"}},
		[]string{"boost.b_col"})
	b := stub("beta",
		[]Derive{{Column: "boost.b_col", Type: "DOUBLE", Expr: "1"}},
		[]string{"boost.a_col"})
	errs := CheckSet([]*Manifest{a, b}, Env{})
	if len(errs) == 0 {
		t.Fatal("a derivation cycle was accepted")
	}
	var found bool
	for _, e := range errs {
		if strings.Contains(e.Error(), "cycle") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no cycle reported, got %v", errs)
	}
}

// TestChainIsNotACycle: a legitimate chain must be allowed, or a module could never read
// another's derived column — and fuel-economy genuinely needs the lambda that
// fuel-mixture derives.
func TestChainIsNotACycle(t *testing.T) {
	a := stub("alpha", []Derive{{Column: "boost.a_col", Type: "DOUBLE", Expr: "1"}}, nil)
	b := stub("beta", []Derive{{Column: "boost.b_col", Type: "DOUBLE", Expr: "1"}},
		[]string{"boost.a_col"})
	c := stub("gamma", nil, []string{"boost.b_col"})
	if errs := CheckSet([]*Manifest{a, b, c}, Env{}); len(errs) != 0 {
		t.Fatalf("a three-module chain was rejected: %v", errs)
	}
}

// TestSelfReferenceIsNotACycle: a module may read a column it derives itself.
func TestSelfReferenceIsNotACycle(t *testing.T) {
	a := stub("alpha", []Derive{{Column: "boost.a_col", Type: "DOUBLE", Expr: "1"}},
		[]string{"boost.a_col"})
	if errs := CheckSet([]*Manifest{a}, Env{}); len(errs) != 0 {
		t.Fatalf("a module reading its own derived column was rejected: %v", errs)
	}
}

// TestDuplicateModuleID: ids namespace views, queries and generated symbols, so two
// modules sharing one would collide silently.
func TestDuplicateModuleID(t *testing.T) {
	errs := CheckSet([]*Manifest{stub("alpha", nil, nil), stub("alpha", nil, nil)}, Env{})
	if len(errs) == 0 || !strings.Contains(errs[0].Error(), "share the id") {
		t.Fatalf("duplicate ids were accepted: %v", errs)
	}
}

// TestRequiresStoreIsResolvedAgainstStoreV1: the check that makes a typo fail validation
// rather than a page.
func TestRequiresStoreIsResolvedAgainstStoreV1(t *testing.T) {
	env := testEnv(t)
	if !env.Store.Has("boost.map_kpa") {
		t.Fatal("store/v1 should pin boost.map_kpa; the schema shape may have changed")
	}
	if env.Store.Has("boost.turbo_rpm") {
		t.Fatal("store/v1 should not pin boost.turbo_rpm")
	}
	m := stub("alpha", nil, []string{"boost.turbo_rpm"})
	errs := Check(m, Env{Store: env.Store, EngineFields: env.EngineFields, Directory: "alpha"})
	if len(errs) == 0 || !strings.Contains(errs[0].Error(), "store/v1") {
		t.Fatalf("an unknown column was accepted: %v", errs)
	}
}

// TestQueryParametersBothDirections: a query may neither read an unbound parameter nor
// silently ignore an argument a caller passed.
func TestQueryParametersBothDirections(t *testing.T) {
	cases := []struct {
		name, want string
		q          Query
	}{
		{"reads an unbound parameter", "not declared",
			Query{Name: "q", SQL: "SELECT 1 WHERE a = $vehicle_id"}},
		{"ignores a declared parameter", "does not appear",
			Query{Name: "q", Params: []Param{{Name: "boot_id", Type: "boot_id"}}, SQL: "SELECT 1"}},
	}
	for _, c := range cases {
		errs := CheckQueries(&QueriesFile{Schema: QueriesSchema, Module: "alpha",
			Queries: []Query{c.q}}, "alpha")
		var all []string
		for _, e := range errs {
			all = append(all, e.Error())
		}
		if !strings.Contains(strings.Join(all, "; "), c.want) {
			t.Errorf("%s: expected %q, got %v", c.name, c.want, all)
		}
	}
}

// TestSemicolonAndPlaceholderInsideAStringAreIgnored: the SQL scan must not mistake a
// literal for a statement separator or a placeholder, or a legitimate query would be
// refused and the rule would be worked around instead of followed.
func TestSemicolonAndPlaceholderInsideAStringAreIgnored(t *testing.T) {
	q := Query{
		Name:   "q",
		Params: []Param{{Name: "vehicle_id", Type: "vehicle_id"}},
		SQL:    "SELECT 'a;b' AS sep, '$not_a_param' AS lit FROM boost WHERE vehicle_id = $vehicle_id",
	}
	errs := CheckQueries(&QueriesFile{Schema: QueriesSchema, Module: "alpha",
		Queries: []Query{q}}, "alpha")
	if len(errs) != 0 {
		t.Fatalf("a query with a semicolon and a $ inside string literals was rejected: %v", errs)
	}
	// And a doubled quote (SQL's escape) must not unbalance the scan.
	q.SQL = "SELECT 'it''s; fine' FROM boost WHERE vehicle_id = $vehicle_id"
	if errs := CheckQueries(&QueriesFile{Schema: QueriesSchema, Module: "alpha",
		Queries: []Query{q}}, "alpha"); len(errs) != 0 {
		t.Fatalf("a doubled quote broke the scan: %v", errs)
	}
}

// TestWithIsAllowed: v_pulls-style queries are common-table expressions, so a query
// beginning WITH must be accepted while a write must not.
func TestWithIsAllowed(t *testing.T) {
	ok := Query{Name: "q", SQL: "WITH x AS (SELECT 1 AS n) SELECT n FROM x"}
	if errs := CheckQueries(&QueriesFile{Schema: QueriesSchema, Module: "a",
		Queries: []Query{ok}}, "a"); len(errs) != 0 {
		t.Fatalf("a WITH query was rejected: %v", errs)
	}
	for _, bad := range []string{"DELETE FROM boost", "UPDATE boost SET x = 1",
		"INSERT INTO boost VALUES (1)", "CREATE TABLE t (a INT)", "ATTACH 'x.db'"} {
		errs := CheckQueries(&QueriesFile{Schema: QueriesSchema, Module: "a",
			Queries: []Query{{Name: "q", SQL: bad}}}, "a")
		if len(errs) == 0 {
			t.Errorf("%q was accepted", bad)
		}
	}
}

// TestPathsCannotLeaveTheModule.
func TestPathsCannotLeaveTheModule(t *testing.T) {
	for _, bad := range []string{"/etc/passwd", "../other/views.sql", "a/../../b.sql", "./x.sql", ""} {
		if err := safeRelPath(bad); err == nil {
			t.Errorf("%q was accepted as a module-relative path", bad)
		}
	}
	for _, good := range []string{"store/views.sql", "api/queries.yaml", "x.sql"} {
		if err := safeRelPath(good); err != nil {
			t.Errorf("%q was rejected: %v", good, err)
		}
	}
}

// TestStubClaimsNothing covers each way a stub can overreach.
func TestStubClaimsNothing(t *testing.T) {
	base := func() *Manifest {
		return &Manifest{Schema: ManifestSchema, ID: "alpha", Version: 1, Name: "Alpha",
			Status: "stub", Sources: []string{"s"}}
	}
	withDerive := base()
	withDerive.Derives = []Derive{{Column: "boost.x", Type: "DOUBLE", Expr: "1"}}
	withMetric := base()
	withMetric.Metrics = []Metric{{Key: "k", Label: "K", Sample: "boot",
		SourceView: "v_alpha_x", SourceColumn: "c"}}
	withField := base()
	withField.Requires = &struct {
		Store        []string `json:"store"`
		EngineFields []string `json:"engine_fields"`
	}{EngineFields: []string{"rpm"}}

	for name, m := range map[string]*Manifest{
		"derives": withDerive, "metrics": withMetric, "engine_fields": withField,
	} {
		errs := Check(m, Env{Directory: "alpha"})
		var all []string
		for _, e := range errs {
			all = append(all, e.Error())
		}
		if !strings.Contains(strings.Join(all, "; "), "a stub claims nothing") {
			t.Errorf("a stub with %s was not refused: %v", name, all)
		}
	}
	// And a bare stub is fine: the manifest-only case.
	if errs := Check(base(), Env{Directory: "alpha"}); len(errs) != 0 {
		t.Errorf("a bare stub was rejected: %v", errs)
	}
}

// TestUnknownFieldIsAnError: a manifest is not a place to stash data (spec §2), and
// `additionalProperties: false` only binds a validator that reads the schema. This binds
// the Go reader.
func TestUnknownFieldIsAnError(t *testing.T) {
	_, err := Decode([]byte(`{"schema":"cairn.module/v1-draft","id":"a","version":1,
		"name":"A","status":"stub","sources":["s"],"cache_ttl_s":30}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("an unknown field was accepted: %v", err)
	}
}
