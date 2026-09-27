# Revisão de GitHub Actions — 2026-09-26

Escopo: `ci.yml`, `release.yml`, `scripts/check-workflows.sh`, contracts, registros e documentação. Fonte: snapshot ZIP anterior, sem assumir que alterações locais foram publicadas no GitHub.

## Achados e tratamento

| Risco | Ajuste nesta árvore | Critério pendente |
| --- | --- | --- |
| `change-records` usava `head.sha` não garantidamente presente no checkout do merge de um PR de fork | checkout HEAD explícito + `fetch-depth: 0`; SHA base/head transmitidos por env (sem interpolação de shell). | Executar PR real de fork. |
| Merge Queue não executava o job `change-records`, tornando o check omitido | job sempre executado; cobertura diferencial só em PR, schema em push/merge_group/manual. | Executar merge queue real se habilitada. |
| Ausência de lint semântico de GitHub Actions | `actionlint` v1.7.12 upstream, binário Linux amd64 verificado com SHA-256 oficial; gate obrigatório no CI e no build da tag. | Baixar e executar em GitHub hosted runner. |
| Drift dos aliases `*-latest` | runners explícitos `ubuntu-24.04`, `macos-15`, `windows-2025`, preservando matrizes arm64 já declaradas. | Executar os seis runners. |
| Go duplicado no workflow e `go.mod`; `go.sum` ausente | `go-version-file: go.mod`, chave de cache `go.mod`, `GOTOOLCHAIN=local`. | `setup-go@v7` obter 1.27.1 em todos os runners. |
| Tag não vinculada à branch padrão; assets publicamente visíveis se upload parcial falhar | checkout histórico completo, guard de ancestralidade de commit, draft antes do upload; só publicar após sucesso. | Testar o fluxo remoto com tag real aprovada. |
| Diagnóstico insuficiente no `gofmt` e no GCC/race Windows | não mascarar falhas do comando externo pelo pipeline/substituição. | Executar no Windows AMD64. |
| Installer Nushell duplicado entre CI e release, extraindo tarball em nome fixo sob `/tmp` | `scripts/install-test-shells.sh` compartilhado, temp privado `mktemp`, checksum oficial, HTTPS-only inclusive redirects e `sudo install` após validação. | Instalação e execução nativas nos runners Ubuntu. |
| Corridas entre dois pushes de mesma tag | `concurrency` com `cancel-in-progress: false`. | Confirmar política de proteção de tags no repositório. |
| Contrato CI não cobria scripts/dependabot e dependia de runner alias antigo | cobertura do `module` estendida a scripts/workflows/Dependabot; asserts de teste atualizados. | Rodar Go 1.27.1 nativo. |

## Garantias e limites

- `gh release create --verify-tag` verifica **existência remota**, não assinatura criptográfica. Configurar rulesets de tag assinada/protegida no próprio GitHub quando exigida essa propriedade.
- SHA-256 de assets é gerado pelo builder e verificado no download e antes da publicação; `actions/download-artifact@v8` também aplica seu controle próprio de hash. Atestação Sigstore continua condicional a repositório público; em privado apenas os checksums são garantidos pelo workflow atual.
- GH Actions pode falhar por indisponibilidade do runner, downloads externos (Go, Nushell, actionlint) e limites do plano. Falhar fechado, **não** marcar sucesso por análise local.
- Não alterar `GOTOOLCHAIN` para versões antigas nem remover os testes Win32 só para tornar CI verde. Não há evidência de execução remota neste snapshot.
- Risco funcional Windows fora do escopo de Actions permanece: ACL explícita permissiva em descendentes legados (`CODE_REVIEW.md`).

## Fontes oficiais consultadas

- `checkout` para PR HEAD: https://github.com/actions/checkout/blob/main/README.md
- `setup-go` leitura de `go.mod` e cache: https://github.com/actions/setup-go/blob/main/README.md
- Runners: https://docs.github.com/en/actions/reference/runners/github-hosted-runners
- Required checks / merge queue: https://docs.github.com/en/pull-requests/how-tos/merge-and-close-pull-requests/troubleshooting-required-status-checks
- Artifacts: https://github.com/actions/upload-artifact e https://github.com/actions/download-artifact
- Attest `subject-checksums`: https://github.com/actions/attest
- `gh release create`: https://cli.github.com/manual/gh_release_create
- `actionlint` binário/checksum: https://github.com/rhysd/actionlint/releases/tag/v1.7.12
- Go 1.27.1, inclusive Windows ARM64: https://go.dev/dl/

## Continuação da auditoria — publicação remota e repetibilidade (2026-09-26)

| Risco concreto | Endurecimento aplicado | Limite da prova |
| --- | --- | --- |
| `gh release create --draft` podia terminar o upload localmente sem confirmar os bytes armazenados no GitHub | `scripts/publish-release.sh` baixa **todos** os assets do draft em diretório temporário privado, confronta nomes de metadados com nomes baixados e verifica SHA-256 de **cada** asset contra os bytes que os seis smokes consumiram antes de tornar a release pública. | Precisa de execução real da API `gh release download` com draft no workflow. |
| Upload interrompido deixava draft que falhava em qualquer nova tentativa | Uma nova execução pode retomar draft **somente** se tag, título, notas, conjunto de nomes e todos os assets já enviados corresponderem ao bundle local. Nunca usa `--clobber`; assets incompatíveis, release já pública e metadata duvidosa falham fechado. | Retomar exige o mesmo commit/tag e que os artifacts originais estejam disponíveis; não tenta reparar release pública. |
| Tag remota podia ser movida depois da validação de ancestry | `scripts/verify-remote-release-tag.sh` consulta `gh api`, resolve tags leves e anotadas até o commit, compara com `GITHUB_SHA` antes de build, antes de criar o draft e novamente imediatamente antes de publicar. | Rulesets/proteção de tags e imutabilidade da release devem ser habilitados no GitHub para impedir alterações após o último check. |
| Atestação no job `publish` atribuía proveniência ao job que só baixava o arquivo | Atestação `subject-checksums` movida para o `build-release` que gerou os bytes. Publicação usa apenas `contents: write`; builder tem permissões OIDC/attestation isoladas. | Atestação continua condicional a repositório público; não confundir proveniência com execução dos smokes. |
| `upload-artifact` com nome fixo e overwrite padrão `false` podia impedir `re-run all jobs` | Os dois artifacts do builder usam `overwrite: true` (mesmo workflow run) e retenção 7 dias. Os smoke/publish jobs sempre baixam e verificam SHA após novo upload; concorrência por tag serializada. | Disponibilidade e comportamento da plataforma serão confirmados em rerun real. |

`gh release create --verify-tag` confere que uma tag existe, mas não a assinatura, nem que ela permaneceu apontando ao commit esperado: por isso existem os guards por API. `gh release download` e `gh release view --json` são usados com tag explícita e token limitado ao job. Uma falha em qualquer leitura remota **mantém o draft fechado**.

Fontes: https://cli.github.com/manual/gh_release_create ; https://cli.github.com/manual/gh_release_download ; https://cli.github.com/manual/gh_release_view ; https://github.com/actions/attest ; https://github.com/actions/upload-artifact ; https://docs.github.com/en/rest/git/refs#get-a-reference ; https://docs.github.com/en/rest/git/tags#get-a-tag .
