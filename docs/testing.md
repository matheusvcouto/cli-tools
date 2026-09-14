# Testes

## Regra central

Nenhum teste usa estado real do usuário. HOME/USERPROFILE/XDG/TMP/caches são sintéticos; Git real somente em repo temporário; Claude/Codex/ACP reais, credentials e config Git global/sistema são proibidos.

## CLI Core

A suíte cobre:

- compiler invariants e stable IDs;
- strict/partial parse pela mesma máquina;
- codecs, constraints, diagnostics e help;
- lazy initialization, capability/requirement e interaction;
- protocol completion v1, Unicode, NUL safety e limite de candidatos;
- conformance dos cinco adapters;
- install/uninstall/status/doctor apenas em HOME/XDG temporários;
- schema/contract diff e locks;
- goldens de help, schema, contract, Fish/Nu/Bash/Zsh/PowerShell, Markdown e man;
- fuzz de parser/protocolo;
- benchmarks de compile, parse, completion, help e schema/contract.

Regenerar goldens é deliberado:

```sh
go test ./cli -run TestGeneratedArtifactsGolden -args -update-golden
```

Inspecione o diff antes de aceitar.

## Shell E2E

A conformance roda sempre. Testes nativos executam o adapter quando o shell existe no runner e fazem skip explícito caso contrário. O CI possui um job dedicado que disponibiliza Bash, Fish, Nushell, Zsh e PowerShell e executa probes comportamentais de todos os adapters; Nushell é fixado e validado por SHA-256. Localmente, shells ausentes continuam sendo skip explícito. Não chamar cross-build ou comparação de strings de “native E2E”.

## Domínio/processos/Git/filesystem

Mantêm as camadas existentes: runner sintético para subprocessos, repos Git temporários, adversarial filesystem/symlink, rollback/no-clobber e E2E externo com executáveis falsos. `ai-profile delete` também prova que input não interativo falha antes da mutação.

## Fuzz

Seeds rodam no teste normal. Smoke limitado e hermético:

```sh
./scripts/check-safe.sh fuzz
```

Crashers nunca devem ser gravados em estado real do usuário.

## Gates

Preferência para o runner sandboxed:

```sh
./scripts/check-safe.sh fmt
./scripts/check-safe.sh test
./scripts/check-safe.sh vet
./scripts/check-safe.sh shuffle
./scripts/check-safe.sh race
./scripts/check-safe.sh fuzz
# ou tudo:
./scripts/check-safe.sh all
```

Ele usa ambiente mínimo, caches temporários, `GOTOOLCHAIN=local`, `GOPROXY=off` e `GOVCS=*:off`. `mise run check` é conveniência, mas não substitui o runner quando é necessário provar isolamento.

Se algum gate exceder a janela do runner ou não puder ser executado, registrar como inconclusivo/não executado.

## CI e plataforma

CI Linux/macOS executa format, test, vet, shuffle e race. Cross-build Windows continua compile-only. Tests de shell adicionais rodam automaticamente quando o shell está instalado no runner. Suporte de runtime só é promovido com evidência nativa.

## Contract locks

`go run ./tools/release contracts check` é um gate dedicado: gera cada contrato
por `__cli contract` em ambiente sintético/offline e compara byte a byte com o
lock versionado. Drift não é corrigido silenciosamente pelo CI.
