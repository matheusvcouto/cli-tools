# Testes

## Regra central

Nenhum teste usa estado real do usuário. HOME/USERPROFILE/XDG/TMP e caches de aplicação são sintéticos; caches de ferramentas Go são dedicados e reutilizáveis; Git real somente em repo temporário; Claude/Codex/Grok/ACP reais, credentials e config Git global/sistema são proibidos.

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
./scripts/check-safe.sh prepare # preparação com rede quando faltarem módulos
./scripts/check-safe.sh fmt
./scripts/check-safe.sh test
./scripts/check-safe.sh vet
./scripts/check-safe.sh shuffle
./scripts/check-safe.sh race
./scripts/check-safe.sh fuzz
# ou tudo:
./scripts/check-safe.sh all
```

A preparação usa o Go já selecionado no PATH, o proxy público padrão e a
checksum database. `GOPROXY`/`GOSUMDB` podem ser configurados explicitamente
nessa etapa (por exemplo, proxy local em uma regressão sintética); não desative
a autenticação dos módulos públicos para contornar erros de download.
Não acessa configuração pessoal nem credenciais. Para adicionar/alterar versões,
use `go get pacote@versão`; `go mod download` prepara as versões declaradas.

Os checks usam ambiente mínimo, `GOFLAGS=-mod=readonly`, `GOTOOLCHAIN=local`,
`GOPROXY=off` e `GOVCS=*:off`. Cache de módulos e compilação permanece em
`dist/go-cache/`; HOME/TMP/Git e estado da aplicação são descartáveis. Se faltar
um módulo ou checksum, o preflight indica a preparação. `-mod=readonly` recusa atualizações necessárias de go.mod; os checksums
necessários em go.sum devem ser preparados antes dos checks. Essa flag não é
uma garantia geral de imutabilidade byte a byte de go.sum; o CI verifica os
manifests após a preparação. O bloqueio de downloads do Go não é firewall para testes.
Não limpe os caches por execução; um cache vazio é um cenário de diagnóstico,
não o requisito de todas as rodadas.

Fuzz executa diretamente na worktree. Crashers ficam em `testdata/fuzz/` e o
cache de fuzz no GOCACHE dedicado; revisar/preservar falhas antes de limpar.
`mise run check` é conveniência, mas não substitui o runner quando é necessário
provar isolamento. CI prepara módulos antes de checks offline e usa go.mod e,
quando presente, go.sum como chaves do cache do setup-go. A preparação no CI
executa go mod verify e falha se modificar ou criar go.mod/go.sum: esses arquivos
precisam chegar completos e revisados no commit. Localmente, a preparação pode
completar go.sum; inspecionar seu diff antes de integrar.

Se algum gate exceder a janela do runner ou não puder ser executado, registrar como inconclusivo/não executado.

## CI e plataforma

CI executa testes nativos em Linux, macOS, Windows x64 (`windows-2025`) e Windows ARM64 (`windows-11-vs2026-arm`). Windows roda `go test ./...`, vet, shuffle, primitives específicas, lock entre processos, Job Object com encerramento real de descendentes e PowerShell completion; race permanece nos runners onde é suportado pelo projeto. Cross-build das seis combinações darwin/linux/windows × amd64/arm64 continua como evidência separada de compilação. Suporte de runtime só é promovido com evidência nativa observada.

## Contract locks

`go run ./tools/release contracts check` é um gate dedicado: gera cada contrato
por `__cli contract` em ambiente sintético/offline e compara byte a byte com o
lock versionado. Drift não é corrigido silenciosamente pelo CI.
