# datadog-cli

Command-line interface for querying Datadog logs, traces, and events.

## Installation

### Homebrew

```
brew install iatsiuk/tap/datadog-cli
```

### Binary releases

Download pre-built binaries for Linux and macOS from
[GitHub Releases](https://github.com/iatsiuk/datadog-cli/releases).

### From source

Requirements: Go 1.25+

```
git clone https://github.com/iatsiuk/datadog-cli
cd datadog-cli
make build
```

The binary is placed at `./datadog-cli`. Move it to a directory in your PATH:

```
mv datadog-cli /usr/local/bin/dd
```

## Authentication

datadog-cli requires a Datadog API key and Application key.

Generate them at: Datadog -> Organization Settings -> API Keys / Application Keys

### Environment variables

```
export DD_API_KEY=your_api_key
export DD_APP_KEY=your_app_key
```

### Site configuration

By default, the CLI connects to `datadoghq.com`. Override with:

```
export DD_SITE=datadoghq.eu
```

Supported sites: `datadoghq.com`, `us3.datadoghq.com`, `us5.datadoghq.com`, `datadoghq.eu`, `ap1.datadoghq.com`, `ddog-gov.com`

## Usage

```
dd [command] [flags]

Flags:
  --json        Output in JSON format
  --version     Show version
  -h, --help    Show help
```

## Commands

### logs

Search and manage Datadog logs.

```
dd logs search [--query <query>] [--from <time>] [--to <time>] [--limit <n>]
dd logs tail [--query <query>] [--service <name>] [--interval <duration>]
dd logs aggregate --compute <fn>[:<metric>] [--query <query>] [--from <time>] [--to <time>] [--group-by <facets>]
dd logs index list
dd logs index show <name>
dd logs index create --name <name> --filter <query> [--retention <days>]
dd logs index update <name> [--filter <query>] [--retention <days>]
dd logs index delete <name> --yes
dd logs pipeline list
dd logs pipeline show <id>
dd logs pipeline create --name <name> [--filter <query>] [--enabled]
dd logs pipeline update <id> --name <name> [--filter <query>] [--enabled]
dd logs pipeline delete <id> --yes
dd logs archive list
dd logs archive show <id>
dd logs archive create --name <name> --query <query> --dest-type <s3|gcs> --dest-bucket <bucket> [--dest-path <path>] [--s3-account-id <id> --s3-role-name <name>] [--gcs-client-email <email>]
dd logs archive update <id> --name <name> --query <query> --dest-type <s3|gcs> --dest-bucket <bucket> [--dest-path <path>] [--s3-account-id <id> --s3-role-name <name>] [--gcs-client-email <email>]
dd logs archive delete <id> --yes
dd logs metric list
dd logs metric show <id>
dd logs metric create --id <id> --compute-type <count|distribution> [--path <path>] --filter <query> [--group-by <facets>]
dd logs metric update <id> [--filter <query>] [--group-by <facets>]
dd logs metric delete <id> --yes
dd logs custom-destination list
dd logs custom-destination show <id>
dd logs custom-destination create --name <name> --url <url> --username <user> --password <pass>
dd logs custom-destination update <id> [--name <name>] [--query <query>]
dd logs custom-destination delete <id> --yes
dd logs restriction-query list
dd logs restriction-query show <id>
dd logs restriction-query create --query <query>
dd logs restriction-query update <id> --query <query>
dd logs restriction-query delete <id> --yes
```

Time format for `--from` / `--to`: relative (`now`, `now-15m`, `now-1h`, `now-7d`) or RFC3339.
Default `--from` is `now-15m`, default `--to` is `now`.

### metrics

Query, submit, and manage Datadog metrics.

```
dd metrics query --query <expr> --from <unix> --to <unix>
dd metrics search --query <q>
dd metrics list --from <unix>
dd metrics scalar --query <expr> --from <unix> --to <unix>
dd metrics timeseries --query <expr> --from <unix> --to <unix>
dd metrics submit --metric <name> --type <gauge|count|rate> --points <ts:value,...> [--tags <tag,...>]
dd metrics metadata show <name>
dd metrics metadata update <name> [--type <type>] [--description <text>] [--unit <unit>]
dd metrics tag-config list [--filter-tags <tags>]
dd metrics tag-config show <name>
dd metrics tag-config create <name> --tags <tag,...> [--aggregations <agg,...>]
dd metrics tag-config update <name> [--tags <tag,...>] [--aggregations <agg,...>]
dd metrics tag-config delete <name> --yes
dd metrics tags <name>
dd metrics volumes <name>
dd metrics assets <name>
dd metrics estimate <name> [--filter-tags <tags>] [--hours <n>] [--future-months <n>]
```

`--query` accepts Datadog metric query expressions (e.g. `avg:system.cpu.user{*}`).
`--from` / `--to` accept Unix timestamps.
`--points` format: `timestamp:value` pairs separated by commas (e.g. `1700000000:42.0,1700000060:43.5`).

## Shell Completion

Generate tab-completion scripts for your shell.

```
dd completion <shell>
```

Supported shells: bash, zsh, fish, powershell

Setup:
```
# bash
dd completion bash > /etc/bash_completion.d/dd

# zsh
dd completion zsh > "${fpath[1]}/_dd"

# fish
dd completion fish > ~/.config/fish/completions/dd.fish

# powershell
dd completion powershell >> $PROFILE
```
