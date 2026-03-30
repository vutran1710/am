module github.com/vutran/agent-mesh/pkg/pipeline

go 1.25.3

require (
	github.com/vutran/agent-mesh/pkg/llm v0.0.0
	github.com/vutran/agent-mesh/pkg/silo v0.0.0
)

replace (
	github.com/vutran/agent-mesh/pkg/llm => ../llm
	github.com/vutran/agent-mesh/pkg/silo => ../silo
)
