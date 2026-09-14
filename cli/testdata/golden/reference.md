# golden-cli

Golden fixture

```text
golden-cli

Usage:
  golden-cli <command> [options]

Commands:
  completion         manage generated shell completions
  help               show help for a command
  serve              serve a target
  version            show product and build version information

Options:
  -v, --verbose            increase verbosity
  -h, --help               Show help
  --version                Show version
```

### `golden-cli completion`

manage generated shell completions

```text
completion — manage generated shell completions

Usage:
  golden-cli completion <command> [options]

Commands:
  doctor             diagnose completion installation
  generate           generate completion code
  install            install generated completion
  list               list supported shells
  status             show completion installation status
  uninstall          uninstall generated completion

Options:
  -v, --verbose            increase verbosity
  -h, --help               Show help
```

#### `golden-cli completion doctor`

diagnose completion installation

```text
doctor — diagnose completion installation

Usage:
  golden-cli completion doctor [shell] [options]

Options:
  -v, --verbose            increase verbosity
  -h, --help               Show help
```

#### `golden-cli completion generate`

generate completion code

```text
generate — generate completion code

Usage:
  golden-cli completion generate <shell> [options]

Options:
  -v, --verbose            increase verbosity
  -h, --help               Show help
```

#### `golden-cli completion install`

install generated completion

```text
install — install generated completion

Usage:
  golden-cli completion install [shell] [options]

Options:
  -v, --verbose            increase verbosity
  -h, --help               Show help
```

#### `golden-cli completion list`

list supported shells

```text
list — list supported shells

Usage:
  golden-cli completion list [options]

Options:
  -v, --verbose            increase verbosity
  -h, --help               Show help
```

#### `golden-cli completion status`

show completion installation status

```text
status — show completion installation status

Usage:
  golden-cli completion status [shell] [options]

Options:
  -v, --verbose            increase verbosity
  -h, --help               Show help
```

#### `golden-cli completion uninstall`

uninstall generated completion

```text
uninstall — uninstall generated completion

Usage:
  golden-cli completion uninstall [shell] [options]

Options:
  -v, --verbose            increase verbosity
  -h, --help               Show help
```

### `golden-cli help`

show help for a command

```text
help — show help for a command

Usage:
  golden-cli help [command]... [options]

Options:
  -v, --verbose            increase verbosity
  -h, --help               Show help
```

### `golden-cli serve`

serve a target

```text
serve — serve a target

Usage:
  golden-cli serve <target> [options]

Serve one target using the selected format.

Options:
  -f, --format <enum>      output format
  -v, --verbose            increase verbosity
  -h, --help               Show help
```

### `golden-cli version`

show product and build version information

```text
version — show product and build version information

Usage:
  golden-cli version [options]

Options:
  --json                   emit structured version information
  -v, --verbose            increase verbosity
  -h, --help               Show help
```
