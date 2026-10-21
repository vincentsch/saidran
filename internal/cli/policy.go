package cli

import (
	"strings"

	"github.com/vincentsch/rungrad"
	"github.com/vincentsch/saidran/internal/report"
)

func parsePolicy(value string) (map[string]bool, error) {
	allowed := strings.Fields("verified stale contradicted partial no_matching_run cant_tell test_skipped test_deleted check_disabled ci_ignores_failure test_command_changed")
	policy := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		valid := false
		for _, v := range allowed {
			if part == v {
				valid = true
				break
			}
		}
		if !valid {
			return nil, rungrad.NewError(rungrad.ExitUsage, "Unknown or empty --fail-on value. Choose from "+strings.Join(allowed, ", ")+".")
		}
		policy[part] = true
	}
	return policy, nil
}
func policyResult(r report.Report, policy map[string]bool) error {
	seen := map[string]bool{}
	matches := []string{}
	add := func(value string) {
		if policy[value] && !seen[value] {
			matches = append(matches, value)
			seen[value] = true
		}
	}
	for _, claim := range r.Claims {
		add(string(claim.Verdict))
	}
	dismissed := map[string]bool{}
	for _, d := range r.Dismissals {
		dismissed[d.ID] = true
	}
	for _, finding := range r.Findings {
		if !dismissed[finding.ID] {
			add(finding.Code)
		}
	}
	if len(matches) > 0 {
		return rungrad.NewError(10, "Failure policy matched: "+strings.Join(matches, ", ")+".")
	}
	return nil
}
