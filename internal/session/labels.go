package session

// AgentLabel is the human name; persisted agent identities stay unchanged.
func AgentLabel(agent string) string {
	switch agent {
	case "claude-code":
		return "Claude Code"
	case "codex":
		return "Codex"
	case "opencode":
		return "OpenCode"
	case "pi":
		return "Pi"
	default:
		return agent
	}
}
