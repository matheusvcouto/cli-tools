# Release, versões e change records

## Três versões independentes

1. **suíte/módulo:** tag Git `vX.Y.Z`, que também versiona a API pública de `cli/`;
2. **produto:** `cmd/<tool>/tool.json`, exibido por `<tool> --version`;
3. **protocolos:** inteiros próprios para completion/schema/contract.

`version --json` expõe versão do produto, suíte, revision/VCS, Go, OS/arch e versões de protocolo/schema. Não force esses números a permanecer iguais.

## Registrar uma mudança

Mudança relevante adiciona um JSON em `changes/`:

```json
{
  "schema_version": 1,
  "changes": [
    {
      "component": "repo-zip",
      "impact": "major",
      "breaking": true,
      "summary": "Reserve --version for product version"
    }
  ]
}
```

Componentes válidos são `module` e cada tool com `tool.json`. Impactos: `none`, `patch`, `minor`, `major`; `none` exige justificativa. Produto >=1.0 com breaking change exige `major`.

Em PR, CI compara os caminhos alterados com os records: mudança em `cli/` exige impacto de `module` **e** dos tools consumidores; mudança específica exige o produto correspondente.

Validação manual:

```sh
go run ./tools/release changes validate
go run ./tools/release contracts check
go run ./tools/release api check
```

`api check` protege a API Go pública de `cli/` na versão corrente. Adições são compatíveis, mas precisam ser gravadas no lock com `api write` para também ficarem protegidas nas releases seguintes. Remoção/mudança de assinatura existente é breaking; `api write --allow-breaking` só deve ser usado após revisão explícita e com change record de `module`. A partir de `v1`, uma quebra pública do módulo exige novo major e sufixo `/vN` no `go.mod` e em todos os imports.

A baseline atual exige Go 1.27.1 e permanece no módulo sem sufixo /v2.
v1.3.1 / 5ba6009 foi integrada na branch desta melhoria, preservando a versão
0.2.1 do media-get. O destino é v1.3.2; preparar seus manifests/changelogs somente
após gates nativos verdes da árvore integrada e revalidar o commit preparado.

`contracts check` regenera os contratos em HOME/XDG/TMP sintéticos, reutilizando os caches Go do invocador, com downloads e
VCS de módulos desabilitados, e falha se algum `cli.contract.json` estiver fora
de sincronia. O CI executa esse gate independentemente do preparo da release.

Em checkout Git, também é possível validar cobertura:

```sh
go run ./tools/release changes validate --base <sha> --head <sha>
```

## Aprovar a numeração

Antes de materializar uma versão diferente da sequência esperada/pedida pelo
usuário, apresentar o resumo das mudanças, as versões atual/esperada/proposta da
suíte, as versões individuais dos produtos e a causa da divergência. Perguntar e
aguardar aprovação explícita; apenas comunicar o resultado calculado ou receber
um pedido genérico de publicar não autoriza a divergência.

O prepare usa o impacto de `module` para minor/major da suíte. Alterações das
CLIs, com versões individuais, provocam patch da suíte e mantêm o impacto
próprio de cada produto. O lock da API Go pública e os gates de breaking changes
continuam obrigatórios. Versões explicitamente pedidas precisam coincidir com
essa política; divergências exigem decisão explícita antes da preparação.

## Preparar a release

O prepare é preview por padrão:

```sh
go run ./tools/release prepare --suite-version v1.3.2
```

Ele lê o último release do `CHANGELOG.md`, change records e `tool.json`; calcula bump da suíte e de cada produto e recusa versão pedida incompatível.

Somente após revisar o preview **e obter os gates nativos reais**:

```sh
go run ./tools/release prepare --suite-version v1.3.2 --write
```

O modo write atualiza manifests/changelogs, regenera `cli.contract.json` e arquiva records consumidos sob `changes/archive/<suite-version>/`. Antes de mutar, o tooling snapshotta todos os arquivos envolvidos; qualquer erro retornado durante a operação dispara rollback do conjunto, além das escritas individuais permanecerem atômicas. Isso protege contra falhas normais do comando, embora nenhum filesystem ofereça commit atômico multi-arquivo contra encerramento abrupto/power loss. O commit resultante deve passar todos os gates antes da tag.

Nunca edite versões de `tool.json` manualmente como substituto desse fluxo.


### Promoção para stable / v1

A estabilidade do produto é parte do `tool.json`, mas a promoção não é feita por edição manual. Um change record pode declarar:

```json
{
  "component": "repo-zip",
  "impact": "major",
  "breaking": true,
  "summary": "Freeze the v1 product contract",
  "stability": "stable"
}
```

O tooling permite apenas progressão `experimental -> alpha -> beta -> stable`, nunca downgrade. Quando um produto cruza de `<1.0.0` para `1.x`, `stability: "stable"` é obrigatório e aparece no preview de `release prepare`. O componente `module` não aceita `stability`; a estabilidade da API Go do módulo é representada pelo próprio SemVer e pelo `cli/api.contract.json`.

## Criar a tag

A tag estável usa exatamente `vX.Y.Z`, nunca é movida/reutilizada, e deve apontar para o commit **já preparado** e verde:

```sh
git tag v1.3.2
git push origin v1.3.2
```

O workflow valida nativamente os seis targets publicados (Linux/macOS/Windows em amd64+arm64) e só então publica. O bundle de release é construído uma única vez; os jobs de smoke baixam esse mesmo artifact imutável e o job de publicação reutiliza exatamente os mesmos archives, sem rebuild entre validação e upload. Actions externas ficam presas a commit SHA completo; Dependabot acompanha atualizações dessas referências.

## Build de artifacts

O workflow executa, conceitualmente:

```sh
go run ./tools/release build \
  --version v1.3.2 \
  --changelog CHANGELOG.md \
  --notes-out dist/release-notes.md \
  --out dist/release
```

O alias legado sem `build` continua aceito pelo tooling. Release builds usam `CGO_ENABLED=0`; race é gate separado.

Cada archive contém todos os executáveis descobertos em `cmd/*`. A versão da suíte é injetada por linker em `internal/version.SuiteVersion`; a versão individual vem do `tool.json` embutido.
O builder exige exatamente os seis archives esperados e recusa arquivos extras no diretório de saída; os smokes também exigem exatamente o conjunto de CLIs derivado de `cmd/*/tool.json`. Assim, archive incompleto ou artifact inesperado falha antes da publicação.

Completions e man pages **não são empacotadas nos artifacts desta fase**. Essa conveniência de distribuição foi deliberadamente adiada: `completion generate/install` continua sendo o caminho canônico para integrações de shell, e referências/man pages continuam derivadas deterministicamente do mesmo grafo. Adicionar esses arquivos aos archives no futuro é mudança de packaging, não requisito do CLI Core.

Smoke test exige para cada binário:

```text
<tool> --version             == <tool> <product-version>
<tool> version --json       .suite_version == tag da release
```

`SHA256SUMS` é verificado com cwd em `dist/release/`. Output de build deve estar vazio; tooling não apaga recursivamente conteúdo fornecido pelo usuário.

## Changelog e GitHub Release

`release prepare --write` materializa a seção versionada em `CHANGELOG.md`. O builder recusa tag/formato/seção inválidos e extrai apenas a seção correspondente para a descrição da GitHub Release; notas automáticas não são a fonte canônica.

## Targets e suporte

Artifacts atuais: darwin/linux/windows em amd64+arm64. Windows usa `.zip` com executáveis `.exe`; o workflow executa smoke nativo em x64 e ARM64 sobre os mesmos archives gerados pelo job `build-release` e posteriormente enviados à GitHub Release. O `publish` não recompila. Cross-build permanece evidência de compilação separada do runtime nativo descrito em `docs/platforms.md`.

## mise e proveniência

Instalação mise deve ser validada em HOME/MISE_*/GH_CONFIG_DIR sintéticos, com discovery de config/tokens desabilitado. Repositório público recebe artifact attestation para archives listados em `SHA256SUMS`; checksums permanecem obrigatórios sempre.

## Gates de GitHub Actions revisados (2026-09-26)

- `ci.yml` aceita push, PR, `workflow_dispatch` e `merge_group`. O job `change-records` roda nos quatro eventos; em PR faz checkout do SHA original do PR com histórico completo para que `--base/--head` existam até em forks. Nos demais eventos verifica a integridade dos records sem fabricar cobertura diferencial de PR.
- `workflow-lint` executa `scripts/check-workflows.sh` (actionlint upstream v1.7.12, SHA-256 oficial verificado, com `.github/actionlint.yaml`). O catálogo embutido dessa versão não conhece o runner hospedado `windows-11-vs2026-arm`; o arquivo declara só esse rótulo, publicado pelo GitHub, para o gate não recusar a matriz ARM64. A mesma checagem é repetida no commit exato da tag antes de construir arquivos de release. Falha de download ou checksum interrompe o gate.
- A instalação de Fish/Zsh/Nushell usada em CI e release está centralizada em `scripts/install-test-shells.sh`, com download HTTPS, SHA-256 upstream e extração em diretório temporário privado.
- Os runners são explícitos (`ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15-intel`, `macos-15`, `windows-2025`, `windows-11-vs2026-arm`) e `setup-go` lê a baseline de `go.mod`. `GOTOOLCHAIN=local` impede download de outro Go por efeito colateral do código. Módulos são preparados antes dos checks offline; `go.mod` e, quando presente, `go.sum` compõem a chave de cache. Não presumir ausência permanente de dependências externas.
- A release falha se o commit da tag não for ancestral da branch padrão atual; o `fetch-depth: 0` no build garante o histórico de referência. O workflow não assina/verifica criptograficamente tags: configure rulesets de branches/tags e restrinja quem pode fazer push de tags. `gh release create --verify-tag` apenas exige que a tag exista remotamente.
- Publicação permanece após seis smokes nativos dos archives imutáveis, com SHA256SUMS verificado em `dist/`. `gh release create --draft` carrega os assets e `gh release edit --draft=false` só publica depois do upload completo. Erro deixa draft para revisão manual, sem clobber silencioso de releases existentes. Uma única execução por tag fica serializada por `concurrency` sem cancelamento.

Após integrar v1.3.1 e esta melhoria, execute a CI no commit combinado,
revise os gates nativos e prepare a release seguinte. Repita CI no commit
preparado antes de criar a tag. Resultados antigos não aprovam código novo.

## Publicação segura e retomada do draft (auditoria 2026-09-26)

O job `build-release` valida o commit/tag remoto, constrói os seis arquivos uma única vez, gera `SHA256SUMS` e **atesta a proveniência na máquina que realmente gerou os bytes** (em repositório público). Em seguida faz upload dos artifacts de workflow; smoke Linux/macOS/Windows baixa esses mesmos arquivos, valida SHA-256 e executa o binário nativo. O job `publish` tem apenas `contents: write` e só roda depois de **todos** os smokes.

`scripts/publish-release.sh` cria um draft vazio para a tag existente, ou retoma um draft anterior quando título, notas e cada asset remoto já enviado coincidirem exatamente com a nova tentativa. Ele envia apenas arquivos ausentes, **sem clobber**; depois baixa integralmente o draft de GitHub Releases e compara os sete arquivos contra o bundle local, inclusive `SHA256SUMS`. Antes de torná-lo público, confirma novamente que a tag remota resolve para `GITHUB_SHA`. Caso API, checksum ou metadados falhem, **não publica**. Não modifica releases já públicas.

Reexecutar o workflow completo substitui artifacts do mesmo workflow run (`overwrite: true`) e os jobs consumidores revalidam seus hashes; retenção 7 dias. Não promova um draft manualmente sem executar os guards. Tags assinadas/protegidas, restrições de force push, branch protection, required checks, aprovação de release e opção de **immutable releases** devem ser configuradas separadamente no repositório GitHub: não podem ser inferidas por análise local do YAML.

Os testes do comportamento remoto (`gh release create/view/download/upload/edit`, GitHub Actions em seis runners e reruns de falhas parciais) seguem **não executados** até um workflow real. Uma execução local de parser ou `bash -n` não comprova esses estados.

## Pré-validação antes dos seis runners

Uma tag `v*` inicia agora um job `preflight` **antes** de `verify` e `native-shell-completion`. Ele instala somente ferramentas do runner Ubuntu x64, executa o actionlint com checksum upstream verificado, valida os change records, confere a seção exata do changelog e o SemVer estável canônico, o path do módulo contra o major da tag, o Go mínimo e a existência de CLIs. Confere ainda que o commit pertence à branch padrão e que a tag remota não foi movida. O `build-release` mantém verificações repetidas, defesa em profundidade.

Registros pendentes precisam ser consumidos pelo preparo da release, após
integração e gates reais. Antes da tag, revise manifests/changelogs/contracts,
revalide o commit preparado e execute o preflight. Não contorne gates para
publicar a versão pretendida.

## Versão deliberada da suíte

A versão calculada continua obrigatória por padrão. Somente por pedido explícito
do usuário, uma versão superior no mesmo major calculado pode ser preparada com:

```sh
go run ./tools/release prepare --suite-version v1.3.0 --allow-suite-version-override
```

O flag não altera os bumps dos produtos nem permite diminuir o bump necessário
ou saltar para outro major. Preview, CI nativo, --write, rollback e novo CI no
commit preparado continuam obrigatórios antes da tag. O destino desta melhoria é v1.3.2, após v1.3.1; não alterar versões manualmente.
