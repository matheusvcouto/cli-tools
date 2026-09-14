# ADR — suíte

Registro curto das decisões arquiteturais compartilhadas por toda a suíte.
Decisões específicas de cada CLI ficam próximas ao respectivo entrypoint:

- [`ai-profile`](cmd/ai-profile/ADR.md);
- [`repo-zip`](cmd/repo-zip/ADR.md).

O CLI Core de `cli/` é a arquitetura ativa das CLIs. A implementação e os
contratos vigentes são descritos pelas docs ativas; planos de migração concluídos
são arquivados conforme D008 e não são dependência normativa da arquitetura.

## D001 — Monorepo Go, módulo único — Accepted

Um `go.mod`; cada CLI distribuída fica em `cmd/<nome>`. O módulo possui uma
release da suíte, enquanto executáveis podem manter versões de produto
independentes conforme D020.

## D002 — Domínio privado; CLI Core é a exceção pública deliberada — Accepted

Regra de negócio permanece em `internal/<tool>`. O pacote `cli/` é uma exceção
intencional: ele define o contrato reutilizável do CLI Core e pode ser importado
por outros projetos. Novos pacotes públicos continuam exigindo necessidade real
e decisão arquitetural explícita.

Enquanto o módulo estiver abaixo de `v1`, breaking changes do pacote público
podem ocorrer de forma deliberada e documentada. Após `v1`, sua compatibilidade
segue o SemVer do módulo Go e as regras de versionamento de módulos do Go.

## D003 — Stdlib-first — Accepted

Preferir stdlib. Dependência externa só entra quando resolve uma lacuna concreta
de segurança/correção/manutenção. O CLI Core não deve recriar uma dependência
madura apenas para manter contagem de dependências artificialmente baixa, mas
qualquer dependência precisa justificar custo, transitivas e contrato assumido.

## D004 — Testes nunca usam estado real — Accepted

Profiles, HOME, credenciais, executáveis Claude/Codex e repos do usuário são
proibidos nos testes. Git real só roda em repo sintético temporário. O mesmo vale
para testes do CLI Core, completions e installers de shell.

## D006 — Release da suíte + versões individuais de ferramentas — Accepted

Uma GitHub Release continua empacotando todos os executáveis descobertos em
`cmd/*`. A tag da release versiona o módulo/suíte e, portanto, também a API
pública de `cli/`.

Cada CLI distribuída possui uma versão de produto própria em
`cmd/<tool>/tool.json`. Alterações não editam versões manualmente a cada commit:
registram impacto em `changes/`; o tooling de preparação de release calcula os
bumps, atualiza manifests e changelogs e valida o contrato.

## D007 — Atomicidade é propriedade de plataforma — Accepted

Replace e no-clobber são operações distintas. Implementações nativas devem
preservar a garantia exigida; uma capability ausente retorna erro explícito.

## D008 — Migrações concluídas viram histórico — Accepted

Planos, contratos legados e referências vão para `docs/history/migrations/<data>-<nome>/`
via `git mv` quando concluídos. Docs ativas não dependem do histórico.

## D009 — Release sem CGO; race separado — Accepted

Artifacts usam `CGO_ENABLED=0`. `go test -race` roda separadamente com
toolchain/runner compatíveis.

## D010 — Lógica separada de implementação específica de SO — Accepted

Criar ports/build tags somente quando a semântica realmente muda. Não criar
backend por SO para código que a stdlib já torna portátil. O CLI Core expressa
requisitos como capabilities; regra de negócio não seleciona SO.

## D011 — Suporte é evidência, não cross-build — Accepted

`GOOS=x go build` prova compilação, não runtime. Estados: `supported`, `partial`,
`untested`, `unsupported`. O CLI Core pode bloquear um comando antes do handler
quando a capability requerida não existe.

## D012 — Shell é adapter do CLI Core, nunca dependência do domínio — Accepted

As CLIs continuam executáveis standalone e a regra de negócio não depende de
Nushell, Bash, Fish, Zsh ou PowerShell. Integrações de shell fazem parte do CLI
Core como adapters explícitos, derivados da mesma especificação tipada da CLI.

Help, parsing, schema e completion não mantêm command trees paralelas. Um novo
shell implementa a interface de adapter e passa por uma conformance suite antes
de ser declarado suportado.

## D013 — Paridade funcional, não visual — Accepted

Migrações preservam capacidades, dados e invariantes de segurança. Texto de
erro, help, tabela e apresentação podem mudar quando o novo contrato for melhor.
Breaking changes de CLI são permitidas durante o cutover quando registradas no
contract diff/changeset e cobertas pela migração.

## D016 — Filesystem confinado compartilhado — Accepted

`internal/safefs` representa a invariante comum “não escapar desta raiz”.
Official builds com Go 1.27.1 usam `os.Root`; o fallback para toolchains antigas
existe para desenvolvimento/bootstrap, não é a baseline de segurança do artifact
oficial.

## D017 — Identidade de path é propriedade do filesystem — Accepted

Paths absolutos diferentes podem identificar o mesmo objeto. Comparações de
identidade de objetos existentes usam metadata + `os.SameFile`; comparações
lexicais permanecem apenas onde a grafia é o contrato. Isso não relaxa regras de
symlink nem substitui containment por raiz/relativo.

## D018 — Histórico de release é derivado de mudanças versionadas — Accepted

`changes/` registra mudanças por componente e impacto (`none`, `patch`, `minor`,
`major`, além de `breaking` quando aplicável). O tooling de release consolida os
registros consumidos nos changelogs/version manifests e gera a descrição da
GitHub Release. O processo deve ser determinístico, auditável e não depender de
release notes automáticas como única fonte histórica.

O fluxo está materializado em `tools/release`: change records são validados em
CI e `release prepare` calcula/aplica bumps antes da tag. `docs/release.md` é o
procedimento operacional vigente.

## D019 — CLI Core declarativo, tipado e compilado — Accepted

`cli/` é a plataforma comum de construção das CLIs e substitui integralmente a antiga decisão D014. Cada ferramenta declara uma
única especificação tipada. Essa especificação é compilada em um grafo imutável
que alimenta parsing estrito/parcial, binding de tipos, constraints, roteamento,
help, diagnostics, disponibilidade/capabilities, completion, schema,
documentação e contract lock.

Princípios obrigatórios:

- não manter listas paralelas de comandos/flags;
- `CompiledApp` é imutável após validação;
- domínio não conhece parser, shell ou renderer;
- adapters não conhecem regra de negócio;
- completion dinâmica é side-effect-free, cancelável e estruturada;
- `help`, `version`, schema e completion estática não inicializam domínio;
- extensibilidade ocorre por interfaces/descritores coerentes, não registries
globais mutáveis nem `init()` mágico;
- Go `plugin` não é o mecanismo de extensões runtime portáveis; extensões
externas futuras usam processo separado e protocolo versionado.

A arquitetura normativa vigente fica em `cli/`, `docs/architecture.md`,
`docs/adding-tools.md`, `docs/testing.md` e `docs/release.md`.

## D020 — Versionamento possui três contratos distintos — Accepted

1. **Módulo/suíte:** a tag `vX.Y.Z` versiona o módulo Go, artifacts da suíte e a
   API pública do pacote `cli/`.
2. **CLI individual:** `cmd/<tool>/tool.json` mantém a versão de produto exibida
   pelo executável e usada em changelog/contract diff daquela ferramenta.
3. **Protocolos/schemas:** completion protocol, CLI schema e contract format têm
   inteiros próprios de versão e só mudam quando o formato/protocolo muda.

Esses números não devem ser artificialmente mantidos iguais. `--version` exibe
a versão individual; `version --json` pode expor também suite/module version,
commit/VCS, Go version, OS/arch e protocol/schema versions.

## D021 — Contrato público de CLI é materializado e comparável — Accepted

Cada CLI gera deterministicamente `cmd/<tool>/cli.contract.json` a partir do
`CompiledApp`. O arquivo é derivado, mas versionado no repositório como lock do
contrato público. CI compara o Spec atual com o lock e classifica diferenças
como aditivas, breaking ou sem efeito público quando possível.

Comandos, flags e argumentos possuem IDs internos estáveis distintos dos nomes
visíveis. Isso permite detectar rename/deprecation sem depender somente de
texto, alimentar docs e manter schema/introspection estáveis.
