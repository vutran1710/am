module github.com/vutran/agent-mesh/pkg/adapter/discord

go 1.25.3

require (
	github.com/bwmarrin/discordgo v0.28.1
	github.com/vutran/agent-mesh/pkg/config v0.0.0
	github.com/vutran/agent-mesh/pkg/provider v0.0.0
	github.com/vutran/agent-mesh/pkg/silo v0.0.0
)

require (
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b // indirect
	golang.org/x/sys v0.0.0-20201119102817-f84b799fce68 // indirect
)

replace (
	github.com/vutran/agent-mesh/pkg/config => ../../config
	github.com/vutran/agent-mesh/pkg/provider => ../../provider
	github.com/vutran/agent-mesh/pkg/silo => ../../silo
)
