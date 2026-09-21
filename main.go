package main

import (
	_ "embed"

	"github.com/wesbragagt/iceman/cmd"
)

//go:embed AGENTS.md
var agentsMD string

func main() {
	cmd.SkillContent = agentsMD
	cmd.Execute()
}
