# ADR — suíte

Registro curto das decisões arquiteturais compartilhadas por toda a suíte.
Decisões específicas de cada CLI ficam próximas ao respectivo entrypoint:

- [`ai-profile`](cmd/ai-profile/ADR.md);
- [`repo-zip`](cmd/repo-zip/ADR.md).

## D001 — Monorepo Go, módulo único — Accepted

Um `go.mod`; cada CLI distribuída fica em `cmd/<nome>`. Uma CI/release, versionamento conjunto e compartilhamento privado simples.

## D002 — Código privado por padrão — Accepted

Domínio fica em `internal/<tool>`. Pacote público só existe quando houver consumidor externo real.

## D003 — Stdlib-first — Accepted

Preferir stdlib. Dependência externa só entra quando resolve uma lacuna concreta de segurança/correção/manutenção.

## D004 — Testes nunca usam estado real — Accepted

Profiles, HOME, credenciais, executáveis Claude/Codex e repos do usuário são proibidos nos testes. Git real só roda em repo sintético temporário.

## D006 — Release como suite — Accepted

Cada release empacota todos os executáveis descobertos em `cmd/*` dentro de `bin/`. O mise instala o archive inteiro.

## D007 — Atomicidade é propriedade de plataforma — Accepted

Replace e no-clobber são operações distintas. Unix usa primitivas testadas; Windows permanece não implementado onde a mesma garantia ainda não existe.

## D008 — Migrações concluídas viram histórico — Accepted

Planos, contratos legados e referências vão para `docs/history/migrations/<data>-<nome>/` via `git mv`. Docs ativas não dependem do histórico.

## D009 — Release sem CGO; race separado — Accepted

Artifacts usam `CGO_ENABLED=0`. `go test -race` roda separadamente com toolchain/runner compatíveis.

## D010 — Lógica separada de implementação específica de SO — Accepted

Criar ports/build tags somente quando a semântica realmente muda. Não criar backend por SO para código que a stdlib já torna portátil.

## D011 — Suporte é evidência, não cross-build — Accepted

`GOOS=x go build` prova compilação, não runtime. Estados: `supported`, `partial`, `untested`, `unsupported`.

## D012 — Shell não faz parte da arquitetura — Accepted

As CLIs são executáveis standalone. Nenhuma regra de negócio depende de Nushell, Bash, Fish ou Zsh. Completions são integrações opcionais geradas pelo próprio binário.

## D013 — Paridade funcional, não visual — Accepted

A migração preserva capacidades, dados e invariantes de segurança. Texto de erro, help, tabela e apresentação podem mudar quando a UX Go for mais clara.

## D014 — Mini core compartilhado, não framework — Accepted

`internal/cliapp` contém apenas IO/erro/exit code comuns. Parsing, command tree e regras permanecem em cada CLI. Não existe framework interno genérico de CLI.

## D016 — Filesystem confinado compartilhado — Accepted

`internal/safefs` representa a invariante comum “não escapar desta raiz”. Official builds com Go 1.27.1 usam `os.Root`; o fallback para toolchains antigas existe para desenvolvimento/bootstrap, não é a baseline de segurança do artifact oficial.

## D017 — Identidade de path é propriedade do filesystem — Accepted

Paths absolutos diferentes podem identificar o mesmo objeto. Comparações de
identidade de objetos existentes usam metadata + `os.SameFile`; comparações
lexicais permanecem apenas onde a grafia é o contrato. Isso não relaxa regras de
symlink nem substitui containment por raiz/relativo.

## D018 — Changelog versionado é a fonte das release notes — Accepted

Tags públicas seguem `vX.Y.Z`. A seção correspondente de `CHANGELOG.md`, com
data ISO e itens organizados em lista, é validada pelo tooling e publicada como
descrição da GitHub Release. O gerador recusa diretório de saída não vazio e não
remove conteúdo preexistente para preparar uma build.
