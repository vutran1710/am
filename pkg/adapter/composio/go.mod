module github.com/vutran/agent-mesh/pkg/adapter/composio

go 1.25.3

require (
	github.com/vutran/agent-mesh/pkg/config v0.0.0
	github.com/vutran/agent-mesh/pkg/provider v0.0.0
	github.com/vutran/agent-mesh/pkg/silo v0.0.0
)

require github.com/BurntSushi/toml v1.5.0 // indirect

replace (
	github.com/vutran/agent-mesh/pkg/config => ../../config
	github.com/vutran/agent-mesh/pkg/provider => ../../provider
	github.com/vutran/agent-mesh/pkg/silo => ../../silo
)
