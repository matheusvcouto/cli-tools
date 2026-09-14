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
      "impact": "minor",
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

`api check` protege a API Go pública de `cli/`. Adições são compatíveis, mas precisam ser gravadas no lock com `api write` para também ficarem protegidas nas releases seguintes. Remoção/mudança de assinatura existente é breaking; `api write --allow-breaking` só deve ser usado durante uma mudança major deliberada.

`contracts check` regenera os contratos em HOME/XDG/TMP sintéticos, com rede e
VCS de módulos desabilitados, e falha se algum `cli.contract.json` estiver fora
de sincronia. O CI executa esse gate independentemente do preparo da release.

Em checkout Git, também é possível validar cobertura:

```sh
go run ./tools/release changes validate --base <sha> --head <sha>
```

## Preparar a release

O prepare é preview por padrão:

```sh
go run ./tools/release prepare --suite-version 0.2.0
```

Ele lê o último release do `CHANGELOG.md`, change records e `tool.json`; calcula bump da suíte e de cada produto e recusa versão pedida incompatível.

Somente após revisar o preview:

```sh
go run ./tools/release prepare --suite-version 0.2.0 --write
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
git tag v0.2.0
git push origin v0.2.0
```

O workflow valida Linux/macOS e só então publica.

## Build de artifacts

O workflow executa, conceitualmente:

```sh
go run ./tools/release build \
  --version v0.2.0 \
  --changelog CHANGELOG.md \
  --notes-out /tmp/release-notes.md \
  --out dist
```

O alias legado sem `build` continua aceito pelo tooling. Release builds usam `CGO_ENABLED=0`; race é gate separado.

Cada archive contém todos os executáveis descobertos em `cmd/*`. A versão da suíte é injetada por linker em `internal/version.SuiteVersion`; a versão individual vem do `tool.json` embutido.

Completions e man pages **não são empacotadas nos artifacts desta fase**. Essa conveniência de distribuição foi deliberadamente adiada: `completion generate/install` continua sendo o caminho canônico para integrações de shell, e referências/man pages continuam derivadas deterministicamente do mesmo grafo. Adicionar esses arquivos aos archives no futuro é mudança de packaging, não requisito do CLI Core.

Smoke test exige para cada binário:

```text
<tool> --version             == <tool> <product-version>
<tool> version --json       .suite_version == tag da release
```

`SHA256SUMS` é verificado com cwd em `dist/`. Output de build deve estar vazio; tooling não apaga recursivamente conteúdo fornecido pelo usuário.

## Changelog e GitHub Release

`release prepare --write` materializa a seção versionada em `CHANGELOG.md`. O builder recusa tag/formato/seção inválidos e extrai apenas a seção correspondente para a descrição da GitHub Release; notas automáticas não são a fonte canônica.

## Targets e suporte

Artifacts atuais: darwin/linux amd64+arm64. Cross-build de Windows em CI é compile-only e não muda o estado de suporte descrito em `docs/platforms.md`.

## mise e proveniência

Instalação mise deve ser validada em HOME/MISE_*/GH_CONFIG_DIR sintéticos, com discovery de config/tokens desabilitado. Repositório público recebe artifact attestation para archives listados em `SHA256SUMS`; checksums permanecem obrigatórios sempre.
