# Code review — Windows, release e módulo Go

Revisado em 2026-09-26 sobre o snapshot fornecido em 2026-09-16. Esta revisão distingue implementação, inspeção estática e execução real.

## Correções consolidadas neste snapshot

1. **Go modules / SemVer:** o módulo público estava em `v1.0.1`, mas o requisito mínimo passou a Go 1.27.1 e o backend legado de filesystem foi removido. Corrigidos `go.mod` e todos os imports de produção/testes para `github.com/matheusvcouto/cli-tools/v2`. O change record agora declara `module: major, breaking: true` e projeta `v2.0.0`; as duas CLIs mantêm versionamento independente (`1.1.0` planejado). O tooling `release prepare` e `release build` impede usar uma tag incompatível com o sufixo do módulo, e `planRelease` rejeita `module: breaking` sem `major` após v1. Incluídos testes reais de regressão para o guard e o planejamento, **não executados neste ambiente**.
2. **Win32:** `syscall.ERROR_NO_MORE_FILES` não é constante exportada pela versão de Go presente para inspeção. `job_windows.go` agora usa `syscall.Errno(18)` explicitamente tipado conforme `WinError.h`, sem criar um stub ou relaxar o tratamento de falhas na enumeração da thread inicial.
3. **Smoke Windows:** os jobs x64/ARM64 rejeitam agora `.exe` que seja reparse point/symlink, além de diretórios e entradas inesperadas. Não é aceito um link como substituto de executável publicado.
4. **Documentação ativa:** ADR, API pública, engenharia e procedimento de release agora refletem a migração `/v2` e o requisito de gates nativos antes da tag. Documentação histórica foi preservada sem reescrever fatos de releases anteriores.

## Implementações Windows herdadas e revisadas

- `internal/filelock`: lock Win32 sobre handle aberto (`LockFileEx`/`UnlockFileEx`).
- `internal/fscommit` e `internal/repozip`: replace e no-clobber distintos, usando `os.Root` confinado (rename sem pré-exclusão; hard-link para publicação sem sobrescrever).
- `internal/aiprofile/platform`: spawn direto sem `cmd.exe`, processo suspenso antes de ingressar no Job Object, encerramento da árvore, status de saída e stdin/stdout/stderr preservados; resolutor explícito de entrypoints npm conhecidos.
- `internal/aiprofile`: chaves de ambiente case-insensitive no Windows e DACL protegida, incluindo herança para novos arquivos.
- CI/release: matriz nativa Linux/macOS/Windows amd64+arm64; actions presas a SHAs; bundle construído uma única vez e reutilizado em smoke e publicação; Windows AMD64 tem race gate, ARM64 não simula suporte do race detector.

## Revisão posterior de Actions

A auditoria de 2026-09-26 sobre o snapshot mais recente está em `ACTIONS_REVIEW.md`. Ela corrige checkout de PRs de fork, validação da Merge Queue, actionlint, aliases de runners, baseline Go, ancestry de tags e publicação em draft; não substitui CI real.

## Risco residual e gates de produção

**ACL de dados antigos:** a DACL protegida no diretório de perfis propaga ACEs *herdáveis*, mas a documentação Win32 não garante a remoção de ACEs **explícitas** já gravadas em descendentes de árvores legadas. Não considerar uma árvore legada com ACE explícita permissiva automaticamente saneada. Antes de declarar migração de armazenamento legado totalmente protegida, validar DACL de descendentes reais e implementar saneamento/verificação fail-closed por handle, sem seguir reparse points. Isso não afeta a política de herança de uma árvore recém-criada, mas é uma ressalva de privacidade concreta para upgrades.

**Evidência:** Go 1.27.1 não foi executável neste sandbox (Go local 1.23.2 e acesso externo ao toolchain bloqueado). Sintaxe/gofmt, JSON/YAML, grafo das Actions, formato das referências imutáveis e import paths foram inspecionados estaticamente. `go test`, `go vet`, cross-build desta árvore, PowerShell e execução nativa Windows continuam **não executados** aqui. Não promover `untested` para `supported` sem registrar os resultados reais do CI na revisão correspondente.

## Documentação de referência

- Go Semantic Import Versioning: https://go.dev/doc/modules/major-version
- Go 1.27.1 release: https://go.dev/doc/devel/release
- Windows `ERROR_NO_MORE_FILES` = 18: https://learn.microsoft.com/windows/win32/debug/system-error-codes--0-499-
- Windows ACL propagation (`SetSecurityInfo`): https://learn.microsoft.com/windows/win32/api/aclapi/nf-aclapi-setsecurityinfo
- Go `os.Root`: https://pkg.go.dev/os#Root
- GitHub-hosted runner labels: https://docs.github.com/actions/reference/runners/github-hosted-runners
- Windows ARM64 VS2026 GA: https://github.blog/changelog/2026-08-20-windows-11-arm64-vs2026-image-generally-available/
