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
mv datadog-cli /usr/local/bin/datadog-cli
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
datadog-cli [command] [flags]

Flags:
  --json        Output in JSON format
  --version     Show version
  -h, --help    Show help
```

## Commands

### logs

Search and manage Datadog logs.

```
datadog-cli logs search [--query <query>] [--from <time>] [--to <time>] [--limit <n>]
datadog-cli logs tail [--query <query>] [--service <name>] [--interval <duration>]
datadog-cli logs aggregate --compute <fn>[:<metric>] [--query <query>] [--from <time>] [--to <time>] [--group-by <facets>]
datadog-cli logs index list
datadog-cli logs index show <name>
datadog-cli logs index create --name <name> --filter <query> [--retention <days>]
datadog-cli logs index update <name> [--filter <query>] [--retention <days>]
datadog-cli logs index delete <name> --yes
datadog-cli logs pipeline list
datadog-cli logs pipeline show <id>
datadog-cli logs pipeline create --name <name> [--filter <query>] [--enabled]
datadog-cli logs pipeline update <id> --name <name> [--filter <query>] [--enabled]
datadog-cli logs pipeline delete <id> --yes
datadog-cli logs archive list
datadog-cli logs archive show <id>
datadog-cli logs archive create --name <name> --query <query> --dest-type <s3|gcs> --dest-bucket <bucket> [--dest-path <path>] [--s3-account-id <id> --s3-role-name <name>] [--gcs-client-email <email>]
datadog-cli logs archive update <id> --name <name> --query <query> --dest-type <s3|gcs> --dest-bucket <bucket> [--dest-path <path>] [--s3-account-id <id> --s3-role-name <name>] [--gcs-client-email <email>]
datadog-cli logs archive delete <id> --yes
datadog-cli logs metric list
datadog-cli logs metric show <id>
datadog-cli logs metric create --id <id> --compute-type <count|distribution> [--path <path>] --filter <query> [--group-by <facets>]
datadog-cli logs metric update <id> [--filter <query>] [--group-by <facets>]
datadog-cli logs metric delete <id> --yes
datadog-cli logs custom-destination list
datadog-cli logs custom-destination show <id>
datadog-cli logs custom-destination create --name <name> --url <url> --username <user> --password <pass>
datadog-cli logs custom-destination update <id> [--name <name>] [--query <query>]
datadog-cli logs custom-destination delete <id> --yes
datadog-cli logs restriction-query list
datadog-cli logs restriction-query show <id>
datadog-cli logs restriction-query create --query <query>
datadog-cli logs restriction-query update <id> --query <query>
datadog-cli logs restriction-query delete <id> --yes
```

Time format for `--from` / `--to`: relative (`now`, `now-15m`, `now-1h`, `now-7d`) or RFC3339.
Default `--from` is `now-15m`, default `--to` is `now`.

### metrics

Query, submit, and manage Datadog metrics.

```
datadog-cli metrics query --query <expr> --from <unix> --to <unix>
datadog-cli metrics search --query <q>
datadog-cli metrics list --from <unix>
datadog-cli metrics scalar --query <expr> --from <unix> --to <unix> [--aggregator <avg|sum|min|max|last>]
datadog-cli metrics timeseries --query <expr> --from <unix> --to <unix>
datadog-cli metrics submit --metric <name> --type <gauge|count|rate> --points <ts:value> [--points <ts:value> ...] [--tags <tag> ...]
datadog-cli metrics metadata show <name>
datadog-cli metrics metadata update <name> [--type <type>] [--description <text>] [--unit <unit>] [--per-unit <unit>] [--short-name <name>]
datadog-cli metrics tag-config list
datadog-cli metrics tag-config show <name>
datadog-cli metrics tag-config create <name> --tags <tag> [--tags <tag> ...] [--metric-type <gauge|count|rate|distribution>]
datadog-cli metrics tag-config update <name> [--tags <tag> ...]
datadog-cli metrics tag-config delete <name> --yes
datadog-cli metrics tags <name>
datadog-cli metrics volumes <name>
datadog-cli metrics assets <name>
datadog-cli metrics estimate <name> [--filter-groups <tags>] [--filter-num-aggregations <n>] [--filter-pct] [--filter-hours-ago <n>] [--filter-timespan-h <n>]
```

`--query` accepts Datadog metric query expressions (e.g. `avg:system.cpu.user{*}`).
`--from` / `--to` accept Unix timestamps.
`--points` is a repeatable flag; each value is a `timestamp:value` pair (e.g. `--points 1700000000:42.0 --points 1700000060:43.5`).
`--aggregator` defaults to `avg`.

### apm

Search spans, list services, manage APM retention filters and span-based metrics.

```
datadog-cli apm search [--query <query>] [--from <time>] [--to <time>] [--limit <n>] [--sort <field>]
datadog-cli apm tail [--query <query>] [--service <name>]
datadog-cli apm aggregate --compute <fn>[:<metric>] [--query <query>] [--from <time>] [--to <time>] [--group-by <facets>]
datadog-cli apm services --env <env>
datadog-cli apm retention-filter list
datadog-cli apm retention-filter show <id>
datadog-cli apm retention-filter create --name <name> --filter <query> --rate <0.0-1.0>
datadog-cli apm retention-filter update <id> --name <name> --filter <query> --rate <0.0-1.0> --enabled <true|false>
datadog-cli apm retention-filter delete <id> --yes
datadog-cli apm span-metric list
datadog-cli apm span-metric show <id>
datadog-cli apm span-metric create --id <id> --compute <count|distribution> [--path <attr>] --filter <query> [--group-by <facets>]
datadog-cli apm span-metric update <id> [--filter <query>] [--group-by <facets>]
datadog-cli apm span-metric delete <id> --yes
```

Search output columns: `TIMESTAMP | SERVICE | RESOURCE | DURATION | STATUS`

Time format for `--from` / `--to`: relative (`now`, `now-15m`, `now-1h`, `now-7d`) or RFC3339.
Default `--from` is `now-15m`, default `--to` is `now`.

### rum

Search RUM events, manage applications, metrics, retention filters, session replay, and audiences.

```
datadog-cli rum search [--query <query>] [--from <time>] [--to <time>] [--limit <n>] [--sort <field>]
datadog-cli rum aggregate --compute <fn>[:<metric>] [--query <query>] [--from <time>] [--to <time>] [--group-by <facets>]
datadog-cli rum app list
datadog-cli rum app show <id>
datadog-cli rum app create --name <name> --type <browser|ios|android|react-native|flutter|roku|electron|unity|kotlin-multiplatform>
datadog-cli rum app update <id> [--name <name>] [--type <type>]
datadog-cli rum app delete <id> --yes
datadog-cli rum metric list
datadog-cli rum metric show <id>
datadog-cli rum metric create --id <id> --compute <aggregation>[:<path>] [--filter <query>] [--group-by <facets>]
datadog-cli rum metric update <id> [--filter <query>] [--group-by <facets>]
datadog-cli rum metric delete <id> --yes
datadog-cli rum retention-filter list --app <app-id>
datadog-cli rum retention-filter show --app <app-id> <filter-id>
datadog-cli rum retention-filter create --app <app-id> --name <name> [--query <query>] [--sample-rate <0.1-100>]
datadog-cli rum retention-filter update --app <app-id> <filter-id> [--name <name>] [--query <query>] [--sample-rate <0.1-100>]
datadog-cli rum retention-filter delete --app <app-id> <filter-id> --yes
datadog-cli rum playlist list
datadog-cli rum playlist show <id>
datadog-cli rum playlist create --name <name>
datadog-cli rum playlist update <id> [--name <name>]
datadog-cli rum playlist delete <id> --yes
datadog-cli rum playlist sessions <id>
datadog-cli rum playlist add-session <playlist-id> <session-id>
datadog-cli rum playlist remove-session <playlist-id> <session-id>
datadog-cli rum heatmap list --view <view-name>
datadog-cli rum heatmap create --app <app-id> --device <device> --event <event-id> --name <name> --view <view-name> --start <epoch-ms> [--session <session-id>] [--view-id <view-id>]
datadog-cli rum heatmap update <id> --event <event-id> --start <epoch-ms> [--session <session-id>] [--view-id <view-id>]
datadog-cli rum heatmap delete <id> --yes
datadog-cli rum session segments --view <view-id> --session <session-id>
datadog-cli rum session watchers <session-id>
datadog-cli rum session watch <session-id> --app <app-id> --event <event-id>
datadog-cli rum session history
datadog-cli rum audience connections list --entity <entity>
datadog-cli rum audience connections create --entity <entity> [flags]
datadog-cli rum audience connections update <id> --entity <entity> [flags]
datadog-cli rum audience connections delete <id> --entity <entity> --yes
datadog-cli rum audience mapping --entity <entity>
datadog-cli rum audience query-users [--query <query>]
datadog-cli rum audience query-accounts [--query <query>]
```

Search output columns: `TIMESTAMP | TYPE | APPLICATION | VIEW | DURATION`

Time format for `--from` / `--to`: relative (`now`, `now-15m`, `now-1h`, `now-7d`) or RFC3339.
Default `--from` is `now-15m`, default `--to` is `now`.

### dashboards

Manage Datadog dashboards and dashboard lists.

```
datadog-cli dashboards list
datadog-cli dashboards show --id <id>
datadog-cli dashboards create --title <title> --layout-type <ordered|free> [--description <text>] [--tags <tag,...>] [--widgets-json <json>]
datadog-cli dashboards update --id <id> --title <title> --layout-type <ordered|free> [--description <text>] [--tags <tag,...>] [--widgets-json <json>]
datadog-cli dashboards update --id <id> --body <json>
datadog-cli dashboards delete --id <id> --yes
datadog-cli dashboards lists list
datadog-cli dashboards lists show --id <id>
datadog-cli dashboards lists create --name <name>
datadog-cli dashboards lists update --id <id> --name <name>
datadog-cli dashboards lists delete --id <id> --yes
datadog-cli dashboards lists add-items --id <list-id> --dashboard <dash-id> --type <type>
datadog-cli dashboards lists remove-items --id <list-id> --dashboard <dash-id> --type <type>
```

Dashboards list output columns: `ID | TITLE | LAYOUT | URL | CREATED | MODIFIED`
Dashboard lists list output columns: `ID | NAME | COUNT | CREATED | MODIFIED`

`--layout-type` accepts: `ordered` (grid-based), `free` (pixel-positioned)
`--tags` accepts a single comma-separated string (e.g. `--tags team:infra,env:prod`)
`--widgets-json` accepts an inline JSON array of widget objects
`--body` accepts a full dashboard JSON (replaces all individual flags)
`--type` for list items accepts: `custom_timeboard`, `custom_screenboard`, `integration_timeboard`, `integration_screenboard`, `host_timeboard`

### events

Search, view, create, and tail Datadog events.

```
datadog-cli events list [--query <query>] [--from <time>] [--to <time>] [--limit <n>] [--sort <field>]
datadog-cli events search --query <query> [--from <time>] [--to <time>] [--limit <n>]
datadog-cli events show <event-id> [--json]
datadog-cli events create --title <title> [--text <text>] [--tags <tag,...>] [--alert-type <type>]
datadog-cli events tail [--query <query>] [--interval <duration>]
```

List/search output columns: `TIMESTAMP | TITLE | SOURCE | TAGS`

Show output fields: ID, Date, Tags, Message

`--alert-type` accepts: `ok`, `warning`, `error` (default: `ok`)

Time format for `--from` / `--to`: relative (`now`, `now-15m`, `now-1h`, `now-7d`) or RFC3339.
Default `--from` is `now-24h`, default `--to` is `now`.

### hosts

List and manage Datadog hosts and their tags.

```
datadog-cli hosts list [--filter <query>] [--from <unix>] [--count <n>] [--start <n>]
datadog-cli hosts totals
datadog-cli hosts mute --name <hostname> [--message <text>] [--end <unix>] [--override]
datadog-cli hosts unmute --name <hostname>
datadog-cli hosts tags list [--source <source>]
datadog-cli hosts tags show --name <hostname> [--source <source>]
datadog-cli hosts tags create --name <hostname> --tags <tag1,tag2> [--source <source>]
datadog-cli hosts tags update --name <hostname> --tags <tag1,tag2> [--source <source>]
datadog-cli hosts tags delete --name <hostname> --yes [--source <source>]
```

List output columns: `NAME | ID | ALIASES | APPS | SOURCES | UP | LAST_REPORTED`

Totals output columns: `TOTAL_ACTIVE | TOTAL_UP`

`--filter` accepts Datadog host search query (e.g. `env:prod`).
`--from` accepts a Unix timestamp; only hosts active since that time are returned.
`--tags` accepts a comma-separated list of `key:value` tag pairs.
`--source` filters tags by source identifier (e.g. `users`, `datadog`, `chef`).
`--end` accepts a Unix timestamp for when the mute expires.
`--override` allows muting a host that is already muted.

### monitors

Manage Datadog monitors, downtimes, and monitor configuration policies.

```
datadog-cli monitors list [--name <name>] [--tags <tag,...>] [--page-size <n>]
datadog-cli monitors show --id <id>
datadog-cli monitors search [--query <query>] [--page <n>] [--per-page <n>]
datadog-cli monitors create --name <name> --type <type> --query <query> [--message <text>] [--tags <tag,...>] [--priority <1-5>] [--thresholds <json>]
datadog-cli monitors update --id <id> [--name <name>] [--query <query>] [--message <text>] [--tags <tag,...>] [--priority <1-5>] [--thresholds <json>]
datadog-cli monitors delete --id <id> --yes
datadog-cli monitors mute --id <id> [--scope <scope>] [--end <unix>]
datadog-cli monitors unmute --id <id> [--scope <scope>]
datadog-cli monitors downtime list [--current-only]
datadog-cli monitors downtime show --id <id>
datadog-cli monitors downtime create --scope <scope> [--monitor-id <id>] [--monitor-tags <tag> ...] [--message <text>] [--start <RFC3339>] [--end <RFC3339>]
datadog-cli monitors downtime update --id <id> [--scope <scope>] [--message <text>] [--start <RFC3339>] [--end <RFC3339>]
datadog-cli monitors downtime cancel --id <id> --yes
datadog-cli monitors policy list
datadog-cli monitors policy show --id <id>
datadog-cli monitors policy create --tag-key <key> [--tag-key-required] [--valid-values <val> ...]
datadog-cli monitors policy update --id <id> [--tag-key <key>] [--tag-key-required] [--valid-values <val> ...]
datadog-cli monitors policy delete --id <id> --yes
```

Monitors list output columns: `ID | NAME | TYPE | STATUS | QUERY`

Downtime list output columns: `ID | SCOPE | MONITOR_ID | STATUS | START | END`

Policy list output columns: `ID | POLICY_TYPE | TAG_KEY | TAG_KEY_REQUIRED | VALID_VALUES`

`--type` accepts Datadog monitor types (e.g. `metric alert`, `query alert`, `composite`, `service check`, `event alert`)
`--thresholds` accepts a JSON object (e.g. `{"critical":90,"warning":80}`)
`--scope` accepts a Datadog scope expression (e.g. `env:prod`, `*`)
`--start` / `--end` for downtimes accept RFC3339 timestamps (e.g. `2026-03-13T10:00:00Z`)
`--priority` accepts 1 (highest) to 5 (lowest)

### slos

Manage Datadog Service Level Objectives (SLOs) and corrections.

```
datadog-cli slos list [--query <query>] [--tags <tags>]
datadog-cli slos show --id <id>
datadog-cli slos history --id <id> --from <time> --to <time>
datadog-cli slos create --name <name> --type <metric|monitor> --thresholds <json> [--description <text>] [--tags <tag,...>] [--numerator <query> --denominator <query>] [--monitor-ids <ids>]
datadog-cli slos update --id <id> [--name <name>] [--description <text>] [--tags <tag,...>] [--thresholds <json>] [--numerator <query>] [--denominator <query>] [--monitor-ids <ids>]
datadog-cli slos delete --id <id> --yes
datadog-cli slos can-delete --id <id>
datadog-cli slos correction list
datadog-cli slos correction show --id <id>
datadog-cli slos correction create --slo-id <id> --category <category> --start <time> [--end <time>] [--description <text>] [--timezone <tz>]
datadog-cli slos correction update --id <id> [--category <category>] [--start <time>] [--end <time>] [--description <text>] [--timezone <tz>]
datadog-cli slos correction delete --id <id> --yes
```

SLOs list output columns: `ID | NAME | TYPE | TARGET | TIMEFRAME | TAGS`

Correction list output columns: `ID | SLO_ID | CATEGORY | START | END | DESCRIPTION`

`--type` accepts: `metric` (custom query), `monitor` (based on existing monitors)
`--thresholds` accepts a JSON array, e.g. `[{"timeframe":"30d","target":99.9}]`
`--timeframe` accepts: `7d`, `30d`, `90d`, `custom`
`--numerator` / `--denominator` are metric queries for metric-based SLOs
`--monitor-ids` is a comma-separated list of monitor IDs for monitor-based SLOs
`--category` accepts: `Scheduled Maintenance`, `Deployment`, `Infrastructure Issue`, `Other`
`--start` / `--end` accept unix timestamps or relative time (e.g. `now-7d`)

## Shell Completion

Generate tab-completion scripts for your shell.

```
datadog-cli completion <shell>
```

Supported shells: bash, zsh, fish, powershell

Setup:
```
# bash
datadog-cli completion bash > /etc/bash_completion.d/datadog-cli

# zsh
datadog-cli completion zsh > "${fpath[1]}/_datadog-cli"

# fish
datadog-cli completion fish > ~/.config/fish/completions/datadog-cli.fish

# powershell
datadog-cli completion powershell >> $PROFILE
```
