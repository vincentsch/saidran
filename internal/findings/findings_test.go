package findings

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vincentsch/saidran/internal/gitstate"
)

func added(p, content string) gitstate.File {
	f := gitstate.File{Path: p, Status: "?", New: []byte(content), Group: gitstate.Group(p, true)}
	h := gitstate.Hunk{}
	for i, l := range strings.Split(content, "\n") {
		h.Lines = append(h.Lines, gitstate.Line{Kind: '+', Text: l, New: i + 1})
	}
	f.Hunks = []gitstate.Hunk{h}
	return f
}
func TestSkipCallForms(t *testing.T) {
	cases := []struct {
		path               string
		positive, negative []string
	}{
		{"calc_test.go", []string{`t.Skip("why")`, `t.SkipNow()`, `t.Skipf("why %s", reason)`}, []string{`// t.Skip("why")`, `/* t.SkipNow() */`, `fmt.Println("t.SkipNow()")`, `other.Skip()`, `// t.Skipf("why")`, `fmt.Println("t.Skipf()")`}},
		{"test_calc.py", []string{`@pytest.mark.skip`, `pytest.skip("why")`, `@unittest.skip("why")`, `@pytest.mark.xfail`}, []string{`# @pytest.mark.skip`, `"""pytest.skip('why')"""`, `pytest.mark.skipper`}},
		{"calc.test.ts", []string{`it.skip('a', fn)`, `test.skip('a', fn)`, `describe.skip('a', fn)`, `it.only('a', fn)`, `test.only('a', fn)`, `describe.only('a', fn)`, `xit('a', fn)`, `xdescribe('a', fn)`}, []string{`// it.skip('a', fn)`, `/* test.only('a', fn) */`, `const text = 'xit(';`, `test.skipped()`}},
		{"tests/CalcTest.php", []string{`$this->markTestSkipped('why');`, `test('a')->skip();`, `#[Skip]`}, []string{`// $this->markTestSkipped('why')`, `# test('a')->skip()`, `/* #[Skip] */`, `$test->skipped()`}},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			for _, line := range tc.positive {
				f := Detect([]gitstate.File{added(tc.path, line)})
				if len(f) != 1 || f[0].Code != "test_skipped" {
					t.Fatalf("%s: %+v", line, f)
				}
			}
			for _, line := range tc.negative {
				if f := Detect([]gitstate.File{added(tc.path, line)}); len(f) != 0 {
					t.Fatalf("negative %s: %+v", line, f)
				}
			}
		})
	}
	for _, p := range []string{"source.go", "docs/example.md", "README.md"} {
		if got := Detect([]gitstate.File{added(p, `t.Skip(); it.only(); pytest.skip()`)}); len(got) != 0 {
			t.Fatal(p, got)
		}
	}
	f := added("calc_test.go", "/*\nt.SkipNow()\n*/\nt.SkipNow()")
	if got := Detect([]gitstate.File{f}); len(got) != 1 || got[0].Line != 4 {
		t.Fatal(got)
	}
}
func TestSuppressionAndCIMarkers(t *testing.T) {
	for _, marker := range []string{"// eslint-disable", "// @ts-ignore", "// @ts-nocheck", "# type: ignore", "# noqa", "//nolint", "/** @phpstan-ignore */", "/** @psalm-suppress */"} {
		got := Detect([]gitstate.File{added("src/a.ts", marker)})
		if len(got) != 1 || got[0].Code != "check_disabled" {
			t.Fatal(marker, got)
		}
	}
	for _, s := range []string{"eslint-enabled", "@ts-ignored", "# type: check", "# noquality", "//nolinter", "@phpstan-ignored", "@psalm-suppressed", `const example = "eslint-disable @ts-ignore # noqa";`, "const example = `//nolint @phpstan-ignore`;"} {
		if got := Detect([]gitstate.File{added("src/a.ts", s)}); len(got) != 0 {
			t.Fatal(s, got)
		}
	}
	for _, s := range []string{"continue-on-error: true", "allow_failure: true", "- run: go test ./... || true", "- run: |\n    go test ./... || true", "script:\n  - pytest || true", "- run:\n    command: pytest || true"} {
		got := Detect([]gitstate.File{added(".github/workflows/check.yml", s)})
		if len(got) != 1 || got[0].Code != "ci_ignores_failure" {
			t.Fatal(s, got)
		}
	}
	for _, s := range []string{"continue-on-error: false", "allow_failure: false", "# allow_failure: true", "name: a || true", "- run: go test ./... || false"} {
		if got := Detect([]gitstate.File{added(".github/workflows/check.yml", s)}); len(got) != 0 {
			t.Fatal(s, got)
		}
	}
	if got := Detect([]gitstate.File{added("src/check.sh", "continue-on-error: true\ngo test || true")}); len(got) != 0 {
		t.Fatal(got)
	}
}
func changed(p, old, newValue string) gitstate.File {
	f := added(p, newValue)
	f.Status = "M"
	f.Old = []byte(old)
	h := gitstate.Hunk{}
	for i, l := range strings.Split(old, "\n") {
		h.Lines = append(h.Lines, gitstate.Line{Kind: '-', Text: l, Old: i + 1})
	}
	h.Lines = append(h.Lines, f.Hunks[0].Lines...)
	f.Hunks = []gitstate.Hunk{h}
	return f
}
func TestTestCommands(t *testing.T) {
	for _, p := range []string{"package.json", "composer.json", "Makefile", "web/package.json", "api/composer.json", "tools/Makefile"} {
		old, newValue := `{"scripts":{"test":"go test ./..."}}`, `{"scripts":{"test":"go test ./calc"}}`
		absent := `{"scripts":{"post-install-cmd":"install"}}`
		if strings.HasSuffix(p, "Makefile") {
			old = "test:\n\tgo test ./...\n"
			newValue = "test:\n\tgo test ./calc\n"
			absent = "build:\n\tgo build ./...\n"
		}
		for _, sides := range [][2]string{{old, newValue}, {absent, newValue}, {old, absent}} {
			got := Detect([]gitstate.File{changed(p, sides[0], sides[1])})
			if len(got) != 1 || got[0].Code != "test_command_changed" || got[0].Path != p {
				t.Fatal(p, got)
			}
			b, e := json.Marshal(got[0])
			if e != nil {
				t.Fatal(e)
			}
			if sides[0] == absent && !strings.Contains(string(b), `"old":null`) {
				t.Fatal(string(b))
			}
			if sides[1] == absent && !strings.Contains(string(b), `"new":null`) {
				t.Fatal(string(b))
			}
		}
	}
	f := changed("composer.json", `{"scripts":{"test":"phpunit","post-install-cmd":"old"}}`, `{"scripts":{"test":"phpunit","post-install-cmd":"new"}}`)
	if got := Detect([]gitstate.File{f}); len(got) != 0 {
		t.Fatal(got)
	}
	f = changed("package.json", `{"scripts":{"test":"old"}}`, `{"scripts":{"test":"new"}}`)
	first := Detect([]gitstate.File{f})[0].ID
	f = changed("package.json", `{"scripts":{"test":"different"}}`, `{"scripts":{"test":"new"}}`)
	if Detect([]gitstate.File{f})[0].ID == first {
		t.Fatal("old command excluded from ID")
	}
}
func TestFindingIdentity(t *testing.T) {
	f := added("calc_test.go", "t.SkipNow(); t.SkipNow()")
	f.Hunks = append(f.Hunks, gitstate.Hunk{Lines: append([]gitstate.Line{}, f.Hunks[0].Lines...)})
	got := Detect([]gitstate.File{f})
	if len(got) != 4 {
		t.Fatal(got)
	}
	ids := map[string]bool{}
	for _, v := range got {
		if ids[v.ID] || len(v.ID) != 14 {
			t.Fatal(got)
		}
		ids[v.ID] = true
	}
	for i := range f.Hunks {
		for j := range f.Hunks[i].Lines {
			f.Hunks[i].Lines[j].New += 20
			f.Hunks[i].Lines[j].Text += " \t"
		}
	}
	f.New = []byte(strings.Repeat("\n", 20) + string(f.New))
	again := Detect([]gitstate.File{f})
	for i := range got {
		if got[i].ID != again[i].ID {
			t.Fatal("line shifts changed ID", got, again)
		}
	}
	f.Hunks[0].Lines = append([]gitstate.Line{{Kind: '-', Text: "old context"}}, f.Hunks[0].Lines...)
	if Detect([]gitstate.File{f})[0].ID == got[0].ID {
		t.Fatal("content change did not change ID")
	}
	f = added("tests/a_test.go", "")
	f.Status = "D"
	if got := Detect([]gitstate.File{f}); len(got) != 1 || got[0].Code != "test_deleted" {
		t.Fatal(got)
	}
	f.Status = "R"
	f.OldPath = "tests/old_test.go"
	if got := Detect([]gitstate.File{f}); len(got) != 0 {
		t.Fatal("rename counted as deletion", got)
	}
}

func TestCommandSpanAndContext(t *testing.T) {
	f := gitstate.File{Path: "package.json", Group: "config", Status: "M", Old: []byte("{\n  \"scripts\": {\n    \"test\": \"old\"\n  },\n  \"other\": true\n}"), New: []byte("{\n  \"scripts\": {\n    \"test\": \"new\"\n  },\n  \"other\": false\n}")}
	f.Hunks = gitstate.ParseHunks("@@ -3 +3 @@\n-    \"test\": \"old\"\n+    \"test\": \"new\"\n@@ -5 +5 @@\n-  \"other\": true\n+  \"other\": false\n")
	got := Detect([]gitstate.File{f})
	if len(got) != 1 || got[0].Line != 3 {
		t.Fatal(got)
	}
	f.Hunks[1].Lines[0].Text = "unrelated old setting"
	if again := Detect([]gitstate.File{f}); len(again) != 1 || again[0].ID != got[0].ID {
		t.Fatal("unrelated hunk changed identity", again)
	}
}

func TestGoSkipAndSkipfInOneDiff(t *testing.T) {
	got := Detect([]gitstate.File{added("calc/calc_test.go", `t.Skip("later"); t.Skipf("later %s", reason)
// t.Skipf("comment")
fmt.Println("t.Skipf()")`)})
	if len(got) != 2 || got[0].Code != "test_skipped" || got[1].Code != "test_skipped" || got[0].ID == got[1].ID {
		t.Fatalf("%+v", got)
	}
}

func TestNestedUnrelatedScripts(t *testing.T) {
	for _, p := range []string{"web/package.json", "api/composer.json", "tools/Makefile"} {
		old, newValue := `{"scripts":{"test":"check","build":"old"}}`, `{"scripts":{"test":"check","build":"new"}}`
		if strings.HasSuffix(p, "Makefile") {
			old = "test:\n\tcheck\n\nbuild:\n\told\n"
			newValue = "test:\n\tcheck\n\nbuild:\n\tnew\n"
		}
		if got := Detect([]gitstate.File{changed(p, old, newValue)}); len(got) != 0 {
			t.Fatalf("%s: %+v", p, got)
		}
	}
}
