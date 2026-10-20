package commands

import "testing"

// Compact summary excerpts from tiny local projects, run with Go 1.27.2,
// pytest 9.1.1, Jest 30.2.0, Vitest 3.2.4, PHPUnit 11.5.56 and Pest 3.8.7.
// Expectations count Go packages; the other runners count tests.
func TestRunnerOutputSamples(t *testing.T) {
	cases := []struct {
		runner, output, outcome           string
		passed, failed, skipped, packages int
		noTests                           bool
	}{
		{"go", "--- PASS: TestPass (0.00s)\n--- SKIP: TestSkip (0.00s)\nPASS\nok  \tsample\t0.002s\n", "passed", -1, -1, 1, 1, false},
		{"go", "--- FAIL: TestFail (0.00s)\nFAIL\nFAIL\tsample\t0.002s\nFAIL\n", "failed", -1, -1, -1, 1, false},
		{"go", "--- FAIL: TestOne (0.00s)\nFAIL\nFAIL\tsample/one\t0.002s\n--- FAIL: TestTwo (0.00s)\nFAIL\nFAIL\tsample/two\t0.002s\nok\tsample/pass\t0.002s\nexit status 3\nFAIL\tsample/exit\t0.002s\nFAIL\nnext statement output\n", "failed", -1, -1, -1, 4, false},
		{"go", "FAIL\nnext statement output\n", "failed", -1, -1, -1, -1, false},
		{"go", "?   \tsample/empty\t[no test files]\n", "passed", -1, -1, -1, 1, true},
		{"go", "ok  \tsample\t0.002s [no tests to run]\n", "passed", -1, -1, -1, 1, true},
		{"go", "?   \tsample/empty\t[no test files]\nok\tsample\t(cached)\n", "passed", -1, -1, -1, 2, false},
		{"go", "--- FAIL: TestCrash (0.00s)\n", "failed", -1, -1, -1, -1, false},
		{"pytest", "2 passed, 1 skipped in 0.02s", "passed", 2, -1, 1, -1, false},
		{"pytest", "========================= 2 passed, 1 skipped in 0.01s =========================", "passed", 2, -1, 1, -1, false},
		{"pytest", "==================== 1 failed, 1 passed, 1 skipped in 0.02s ====================", "failed", 1, 1, 1, -1, false},
		{"pytest", "============================ no tests ran in 0.01s ============================", "unknown", -1, -1, -1, -1, true},
		{"pytest", "===================== 1 passed, 1 error in 0.02s =====================", "failed", 1, 1, -1, -1, false},
		{"jest", "Test Suites: 1 passed, 1 total\nTests:       1 skipped, 2 passed, 3 total\nSnapshots: 0 total", "passed", 2, -1, 1, -1, false},
		{"jest", "Tests:       1 failed, 1 skipped, 1 passed, 3 total", "failed", 1, 1, 1, -1, false},
		{"jest", "No tests found, exiting with code 1", "unknown", -1, -1, -1, -1, true},
		{"vitest", " Test Files  1 passed (1)\n      Tests  2 passed | 1 skipped (3)", "passed", 2, -1, 1, -1, false},
		{"vitest", "      Tests  1 failed | 1 passed | 1 skipped (3)", "failed", 1, 1, 1, -1, false},
		{"vitest", "No test files found, exiting with code 1", "unknown", -1, -1, -1, -1, true},
		{"phpunit", "OK (1 test, 1 assertion)", "passed", 1, -1, -1, -1, false},
		{"phpunit", "OK, but some tests were skipped!\nTests: 3, Assertions: 2, Skipped: 1.", "passed", 2, 0, 1, -1, false},
		{"phpunit", "FAILURES!\nTests: 3, Assertions: 2, Failures: 1, Skipped: 1.", "failed", 1, 1, 1, -1, false},
		{"phpunit", "FAILURES!", "failed", -1, -1, -1, -1, false},
		{"phpunit", "No tests executed!", "unknown", -1, -1, -1, -1, true},
		{"pest", "  Tests:    1 skipped, 2 passed (2 assertions)", "passed", 2, -1, 1, -1, false},
		{"pest", "  Tests:    1 failed, 1 skipped, 1 passed (2 assertions)", "failed", 1, 1, 1, -1, false},
		{"pytest", "still running", "unknown", -1, -1, -1, -1, false},
		{"jest", "Test Suites: 1 passed, 1 total", "unknown", -1, -1, -1, -1, false},
		{"pest", "\x1b[32m  Tests:    2 passed\x1b[0m", "passed", 2, -1, -1, -1, false},
	}
	for _, tc := range cases {
		t.Run(tc.runner+"/"+tc.output, func(t *testing.T) {
			s := parseOutput(tc.runner, tc.output, &TestSummary{})
			if s.Outcome != tc.outcome || s.NoTests != tc.noTests {
				t.Fatalf("outcome/no-tests: %+v", s)
			}
			counts := []*int{s.Passed, s.Failed, s.Skipped, s.Packages}
			wants := []int{tc.passed, tc.failed, tc.skipped, tc.packages}
			for i, v := range counts {
				if (wants[i] < 0 && v != nil) || (wants[i] >= 0 && (v == nil || *v != wants[i])) {
					t.Errorf("count %d got %v want %d", i, v, wants[i])
				}
			}
		})
	}
}
func TestRunScopeAndListOnly(t *testing.T) {
	cases := []struct {
		line           string
		filtered, list bool
	}{
		{"go test ./...", false, false},
		{"go test", true, false}, {"go test ./calc", true, false}, {"go test ./... -run TestAverage", true, false},
		{"go test -list .", false, true}, {"go test ./... -list=.", false, true},
		{"go test ./... -count 1 -timeout 30s", false, false},
		{"pytest --collect-only", false, true}, {"pytest --co", false, true},
		{"pytest -k median", true, false}, {"pytest tests/test_calc.py", true, false}, {"pytest tests/test_calc.py::test_one", true, false},
		{"jest --listTests", false, true}, {"jest -t average", true, false}, {"jest calc.test.js", true, false},
		{"vitest list", false, true}, {"vitest run -t average", true, false}, {"vitest run calc.test.js", true, false},
		{"phpunit --filter testAverage", true, false}, {"pest --filter=average", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			s := analyze(tc.line, "", 0).Statements[0]
			if s.Tests == nil || s.Tests.Filtered != tc.filtered || s.Tests.ListOnly != tc.list {
				t.Fatalf("flags: %+v", s.Tests)
			}
			if tc.list && (s.Tests.Passed != nil || s.Tests.Packages != nil) {
				t.Fatal("list-only run acquired test counts")
			}
		})
	}
	c := analyze("go test -list .", "TestAverage\nok\tsample\t0.002s", 0)
	if c.Statements[0].Tests.Packages != nil {
		t.Fatal("list output counted as test run")
	}
	c = analyze("pytest -k median", "=== 1 passed, 2 deselected in 0.01s ===", 0)
	if !c.Statements[0].Tests.Filtered || c.Statements[0].Tests.Passed == nil {
		t.Fatal("filtered result lost")
	}
}
