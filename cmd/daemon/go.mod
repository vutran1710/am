module github.com/vutran/agent-mesh/cmd/daemon

go 1.25.3

require (
	github.com/spf13/cobra v1.9.1
	github.com/vutran/agent-mesh/pkg/adapter/composio v0.0.0
	github.com/vutran/agent-mesh/pkg/adapter/nango v0.0.0
	github.com/vutran/agent-mesh/pkg/config v0.0.0
	github.com/vutran/agent-mesh/pkg/log v0.0.0
	github.com/vutran/agent-mesh/pkg/provider v0.0.0
	github.com/vutran/agent-mesh/pkg/silo v0.0.0
	github.com/vutran/agent-mesh/pkg/store/sqlite v0.0.0
)

require (
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	golang.org/x/exp v0.0.0-20250305212735-054e65f0b394 // indirect
	golang.org/x/sys v0.31.0 // indirect
	modernc.org/libc v1.62.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.9.1 // indirect
	modernc.org/sqlite v1.37.0 // indirect
)

replace (
	github.com/vutran/agent-mesh/pkg/adapter/composio => ../../pkg/adapter/composio
	github.com/vutran/agent-mesh/pkg/adapter/nango => ../../pkg/adapter/nango
	github.com/vutran/agent-mesh/pkg/config => ../../pkg/config
	github.com/vutran/agent-mesh/pkg/log => ../../pkg/log
	github.com/vutran/agent-mesh/pkg/provider => ../../pkg/provider
	github.com/vutran/agent-mesh/pkg/silo => ../../pkg/silo
	github.com/vutran/agent-mesh/pkg/store/sqlite => ../../pkg/store/sqlite
)
