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
