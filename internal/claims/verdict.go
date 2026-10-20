package claims

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/vincentsch/gaugestone/internal/commands"
	"github.com/vincentsch/gaugestone/internal/gitstate"
	"github.com/vincentsch/gaugestone/internal/pathutil"
	"github.com/vincentsch/gaugestone/internal/session"
)

type coverage struct {
	paths     []string
	recursive bool
	filters   []string
	unknown   bool
}

var failedGoPackages = regexp.MustCompile(`(?m)^FAIL[ \t]+([^ \t\r\n]+)`)

type candidate struct {
	entry    commands.Entry
	run      Run
	area     coverage
	position int
}

func within(parent, child string) bool { return pathutil.Within(parent, child) }

func resolve(root, cwd, target string) string {
	// Resolve Go import paths using the recorded repository's module name.
	if b, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		fields := strings.Fields(string(b))
		for i, v := range fields {
			if v == "module" && i+1 < len(fields) {
				module := strings.Trim(fields[i+1], "\"")
				if target == module {
					return root
				}
				if strings.HasPrefix(target, module+"/") {
					return pathutil.Resolve(root, strings.TrimPrefix(target, module+"/"))
				}
				break
			}
		}
	}
	return pathutil.Resolve(cwd, target)
}
func area(root string, st commands.Statement) coverage {
	st.Cwd = pathutil.Resolve(root, st.Cwd)
	a := coverage{unknown: st.Class == "test" && st.Scope == "unknown"}
	// Filters are separate from paths: a passing filtered run covers only that filter.
	for i, arg := range st.Argv {
		for _, flag := range []string{"-run", "-bench", "-k", "-m", "-t", "--filter", "--testNamePattern", "--testsuite"} {
			if arg == flag && i+1 < len(st.Argv) {
				a.filters = append(a.filters, st.Argv[i+1])
			}
			if v, ok := strings.CutPrefix(arg, flag+"="); ok {
				a.filters = append(a.filters, v)
			}
		}
	}
	for _, target := range invocationTargets(st) {
		filter := false
		for _, f := range a.filters {
			if target == f {
				filter = true
			}
		}
		if filter {
			continue
		}
		if p, _, ok := strings.Cut(target, "::"); ok {
			a.paths = append(a.paths, resolve(root, st.Cwd, p))
			a.filters = append(a.filters, target)
			continue
		}
		recursive := strings.HasSuffix(target, "/...") || st.Runner != "go" && target == "."
		target = strings.TrimSuffix(target, "/...")
		a.recursive = a.recursive || recursive
		a.paths = append(a.paths, resolve(root, st.Cwd, target))
	}
	if len(a.paths) == 0 {
		a.paths = []string{st.Cwd}
		a.recursive = st.Scope == "whole" || st.Class != "test"
	}
	return a
}
func claimArea(root, cwd string, c Claim) coverage {
	cwd = pathutil.Resolve(root, cwd)
	if c.invocation != nil {
		st := *c.invocation
		st.Cwd = cwd
		return area(root, st)
	}
	a := coverage{paths: []string{cwd}, recursive: true}
	if c.Scope == "whole" || len(c.Targets) == 0 {
		return a
	}
	a.paths = nil
	a.recursive = false
	for _, t := range c.Targets {
		if strings.HasPrefix(t, "Test") && !strings.Contains(t, "/") {
			a.filters = append(a.filters, t)
			continue
		}
		if p, _, ok := strings.Cut(t, "::"); ok {
			a.paths = append(a.paths, resolve(root, cwd, p))
			a.filters = append(a.filters, t)
			continue
		}
		a.paths = append(a.paths, resolve(root, cwd, strings.TrimSuffix(t, "/...")))
	}
	if len(a.paths) == 0 {
		a.paths = []string{cwd}
	}
	return a
}
func covers(a, b coverage) bool {
	if a.unknown {
		return false
	}
	if len(a.filters) > 0 {
		if len(b.filters) == 0 {
			return false
		}
		for _, bf := range b.filters {
			found := false
			for _, af := range a.filters {
				if af == bf {
					found = true
				}
			}
			if !found {
				return false
			}
		}
	}
	for _, bp := range b.paths {
		found := false
		for _, ap := range a.paths {
			if ap == bp && (!b.recursive || a.recursive) || a.recursive && within(ap, bp) && ap != bp || filepath.Dir(bp) == ap && filepath.Ext(bp) != "" {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func overlaps(a, b coverage) bool {
	if len(a.filters) > 0 && len(b.filters) > 0 {
		same := false
		for _, af := range a.filters {
			for _, bf := range b.filters {
				if af == bf {
					same = true
				}
			}
		}
		if !same {
			return false
		}
	}
	for _, ap := range a.paths {
		for _, bp := range b.paths {
			if within(ap, bp) || within(bp, ap) {
				return true
			}
		}
	}
	return false
}
func finalTime(s *session.Session) time.Time {
	if !s.FinalTime.IsZero() {
		return s.FinalTime
	}
	return s.Ended
}

func Evaluate(root string, s *session.Session, ledger commands.Ledger) ([]Claim, []Gap) {
	root = pathutil.Resolve("", root)
	claims, gaps := Extract(s.Final, finalTime(s))
	originals := map[int]*session.Command{}
	for _, e := range s.Events {
		if e.Command != nil {
			originals[e.Seq] = e.Command
		}
	}
	for i := range claims {
		evaluate(root, s, &claims[i], ledger, originals)
	}
	return claims, gaps
}
func evaluate(root string, s *session.Session, c *Claim, ledger commands.Ledger, originals map[int]*session.Command) {
	desired := claimArea(root, s.Cwd, *c)
	candidates := []candidate{}
	for pos, e := range ledger.Entries {
		st := e.Command
		if st == nil || st.Execution == "skipped" {
			continue
		}
		kind := st.Class
		if kind == "test" {
			kind = "tests"
		}
		if kind != c.Type || c.Runner != "" && c.Runner != st.Runner {
			continue
		}
		original := originals[e.Seq]
		if original == nil {
			continue
		}
		end := original.End
		if end.IsZero() {
			end = e.Time
		}
		if end.After(c.Time) {
			continue
		}
		a := area(root, *st)
		wanted := desired
		// An explicit whole-suite command describes the suite at the run's cwd.
		if relativeWhole(*c) && within(root, st.Cwd) {
			wanted = wholeAt(st.Cwd)
		}
		if !overlaps(a, wanted) {
			continue
		}
		candidates = append(candidates, candidate{e, Run{e.Seq, e.Statement, st.Line, st.Cwd, original.Start, end, st.Result(), st.ExitNote}, a, pos})
	}
	c.NextStep = defaultNext(*c)
	if len(candidates) == 0 {
		c.Verdict = "no_matching_run"
		c.Explanation = "No matching run found in the session log."
		return
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.run.End.Equal(b.run.End) {
			return a.position < b.position
		}
		return a.run.End.Before(b.run.End)
	})
	chosen := len(candidates) - 1
	for j := range candidates {
		wanted := desired
		if relativeWhole(*c) && within(root, candidates[j].run.Cwd) {
			wanted = wholeAt(candidates[j].run.Cwd)
		}
		if covers(candidates[j].area, wanted) {
			chosen = j
		}
	}
	run := candidates[chosen]
	c.Runs = append(c.Runs, run.run)
	c.NextStep = "Run " + run.run.Command + " again."
	// Keep failures until a later passing run covers the failed target. A passing
	// different package or filter cannot erase contradictory evidence.
	for j := len(candidates) - 1; j >= 0; j-- {
		failure := candidates[j]
		if failure.run.Result != "failed" {
			continue
		}

		failedArea := failure.area
		if failure.entry.Command.Runner == "go" {
			original := originals[failure.entry.Seq]
			raw := original.RawOutput
			if raw == "" {
				raw = original.Output
			}
			// Go names failed packages. A rerun of those packages can resolve
			// an earlier failure without repeating unrelated passing packages.
			if matches := failedGoPackages.FindAllStringSubmatch(raw, -1); len(matches) > 0 {
				failedArea.paths = nil
				failedArea.recursive = false
				for _, match := range matches {
					failedArea.paths = append(failedArea.paths, resolve(root, failure.run.Cwd, match[1]))
				}
			}
		}
		wanted := desired
		if relativeWhole(*c) && within(root, failure.run.Cwd) {
			wanted = wholeAt(failure.run.Cwd)
		}
		if !overlaps(failedArea, wanted) {
			continue
		}
		superseded := false
		for k := j + 1; k < len(candidates); k++ {
			later := candidates[k]
			if later.run.Result == "passed" && later.entry.Command.Runner == failure.entry.Command.Runner && covers(later.area, failedArea) {
				superseded = true
				break
			}
		}
		if !superseded {
			c.Verdict = "contradicted"
			if failure.position != run.position {
				c.Runs = append(c.Runs, failure.run)
			}
			c.Explanation = fmt.Sprintf("%s failed at %s within the claimed scope.", failure.run.Command, clock(failure.run.End))
			c.NextStep = "Fix the failure and run " + failure.run.Command + " again."
			return
		}
	}
	// A resolved or unrelated failure cannot supply passing coverage. Retain
	// that earlier run as context and decide from the latest passing rerun.
	if run.run.Result == "failed" {
		for j := len(candidates) - 1; j >= 0; j-- {
			if candidates[j].run.Result == "passed" {
				run = candidates[j]
				c.Runs = append(c.Runs, run.run)
				c.NextStep = "Run " + run.run.Command + " again."
				break
			}
		}
		if run.run.Result == "failed" {
			c.Verdict = "cant_tell"
			c.Explanation = run.run.Command + " failed outside the claimed scope; no passing run for this claim was found."
			return
		}
	}
	if run.run.Result != "passed" || c.Type == "tests" && run.area.unknown {
		c.Verdict = "cant_tell"
		c.Explanation = run.run.Command + ": the recorded result, runner or test scope is unknown."
		if run.run.ExitNote != "" {
			c.Explanation += " " + run.run.ExitNote + "."
		}
		return
	}
	during, unknown := false, false
	for pos, e := range ledger.Entries {
		if e.Seq == run.entry.Seq {
			if e.Statement <= run.entry.Statement {
				continue
			}
		} else {
			if e.Time.Before(run.run.Start) || e.Time.After(c.Time) {
				continue
			}
			if e.Time.Equal(run.run.End) && pos <= run.position {
				continue
			}
		}
		relevant := false
		if e.Edit != nil {
			path := pathutil.Resolve(s.Cwd, e.Edit.Path)
			if within(root, path) {
				rel, _ := filepath.Rel(root, path)
				if gitstate.Group(filepath.ToSlash(rel), true) != "docs" {
					relevant = true
					c.Edits = append(c.Edits, Edit{filepath.ToSlash(rel), e.Time, e.Seq})
				}
			}
		}
		unresolved := e.Command != nil && e.Command.MayEditFiles && e.Command.Execution != "skipped"
		if !relevant && !unresolved {
			continue
		}
		if e.Seq != run.entry.Seq && !e.Time.After(run.run.End) {
			during = true
		}
		unknown = unknown || unresolved
	}
	if during {
		c.Verdict = "cant_tell"
		c.Explanation = run.run.Command + " passed, but files may have changed while it ran."
		return
	}
	if unknown {
		c.Verdict = "cant_tell"
		c.Explanation = run.run.Command + " passed, then a command may have edited unresolved files."
		return
	}
	if len(c.Edits) > 0 {
		c.Verdict = "stale"
		e := c.Edits[0]
		c.Explanation = fmt.Sprintf("%s passed at %s, then %s changed at %s.", run.run.Command, clock(run.run.End), e.Path, clock(e.Time))
		return
	}
	wanted := desired
	if relativeWhole(*c) && within(root, run.run.Cwd) {
		wanted = wholeAt(run.run.Cwd)
	}
	partial := !covers(run.area, wanted)
	reason := "the run covered less than the claim"
	if t := run.entry.Command.Tests; t != nil {
		if t.ListOnly {
			partial = true
			reason = "it only listed tests"
		}
		if t.NoTests {
			partial = true
			reason = "the run found no tests"
		}
	}
	if partial {
		c.Verdict = "partial"
		c.Explanation = run.run.Command + " succeeded, but " + reason + "."
		c.NextStep = "Run a check covering the claimed scope; the recorded command was " + run.run.Command + "."
		return
	}
	c.Verdict = "verified"
	c.Explanation = fmt.Sprintf("%s passed at %s with matching coverage and no later relevant edits.", run.run.Command, clock(run.run.End))
	c.NextStep = "No rerun needed for this claim in the recorded session."
}
func clock(t time.Time) string { return t.Local().Format("15:04") }
func defaultNext(c Claim) string {
	if c.invocation != nil {
		return "Run " + c.invocation.Line + "."
	}
	if c.Runner != "" {
		return "Run the matching " + c.Runner + " check for the claimed scope."
	}
	switch c.Type {
	case "tests":
		return "Run the test command covering the claimed scope."
	case "build":
		return "Run the build command for the claimed scope."
	default:
		return "Run the " + c.Type + " command for the claimed scope."
	}
}

func relativeWhole(c Claim) bool {
	return c.Scope == "whole" && (c.invocation == nil || c.invocation.Scope == "whole")
}
func wholeAt(cwd string) coverage {
	return coverage{paths: []string{pathutil.Resolve("", cwd)}, recursive: true}
}

// Check commands use conventional positional paths. Script bodies remain hints.
func invocationTargets(st commands.Statement) []string {
	if st.Class == "test" {
		return st.Targets
	}
	if st.Runner == "make" {
		return nil
	}
	start := -1
	for i, arg := range st.Argv {
		if filepath.Base(arg) == st.Runner {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}
	if st.Runner == "go" || st.Runner == "cargo" || st.Runner == "ruff" || st.Runner == "biome" {
		start++
	}
	var targets []string
	skip := false
	for _, arg := range st.Argv[start:] {
		if skip {
			skip = false
			continue
		}
		if strings.HasPrefix(arg, "-") {
			switch arg {
			case "-p", "--project", "-o", "--outDir", "--config", "-c", "--configuration", "--output-format", "--format":
				skip = true
			}
			continue
		}
		if arg == "run" || arg == "analyse" || arg == "analyze" {
			continue
		}
		targets = append(targets, arg)
	}
	return targets
}
