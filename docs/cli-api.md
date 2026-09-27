# Public Go API (`cli`)

The package `github.com/matheusvcouto/cli-tools/v2/cli` is intentionally reusable by other Go projects. It is not an internal implementation detail of the bundled CLIs.

```go
import cli "github.com/matheusvcouto/cli-tools/v2/cli"

app, err := cli.Compile(cli.App{
    ID:   "example",
    Name: "example",
    Root: cli.Command{
        ID:   "example.root",
        Name: "example",
        Commands: []cli.Command{{
            ID:   "example.hello",
            Name: "hello",
            Args: []cli.Arg{{
                ID:       "example.hello.name",
                Name:     "name",
                Value:    cli.StringValue(),
                Required: true,
            }},
            Handler: func(inv *cli.Invocation) error {
                name, _ := cli.ValueAs[string](inv, "example.hello.name")
                _ = name
                return nil
            },
        }},
    },
})
```

The external-package test `cli/public_api_external_test.go` compiles and exercises the package exactly as a separate consumer would: compile, typed invocation, completion, help, schema, contract and shell generation. Dynamic completers can read earlier parsed non-sensitive values with `CompletionValueAs[T]` / `CompletionValuesAs[T]`; sensitive values are never placed in `CompleteContext`.

## Compatibility policy

The suite/module Git tag versions the public Go API. Go 1.27.1 is the minimum for this codebase and the next incompatible release is planned as `v2.0.0`; the import path is `github.com/matheusvcouto/cli-tools/v2`. Consumers of the former `v1` module retain the original (unsuffixed) import path until they intentionally migrate. Future incompatible changes to the stable `v2` public API require a `v3` major release with `/v3` imports; changes within `v2` must preserve Go import compatibility.

- removing an exported symbol or changing a locked declaration is allowed only after explicit review, a `module` change record and `api write --allow-breaking`;
- additive exported API is allowed, but the API lock must be regenerated so the new symbol becomes protected too;
- implementation details under `cli/internal/` are never public API;
- CLI syntax compatibility is a separate contract guarded by each tool's `cli.contract.json`.

The compatibility lock is `cli/api.contract.json` and is generated only from exported Go declarations. Private named struct fields are intentionally excluded; embedded fields remain part of the lock because they affect promoted API.

Check it with:

```sh
go run ./tools/release api check
```

After an intentional additive API change:

```sh
go run ./tools/release api write
```

`api write` refuses an existing breaking API change by default. After explicit review of an intentional breaking module change, the baseline may be replaced with:

```sh
go run ./tools/release api write --allow-breaking
```

That command is not a substitute for the required `module` change record and review of the incompatible change.

## Stable release gate

A v1 tag should be cut only after all of the following are green on the exact commit being tagged:

```text
go test ./...
go vet ./...
go test -shuffle=on -count=3 ./...
go test -race ./...
go run ./tools/release api check
go run ./tools/release contracts check
go run ./tools/release changes validate
```

The native shell completion CI must also pass for Bash, Fish, Nushell, Zsh and PowerShell. Cross-build is compile evidence only; it is not runtime-support evidence.
