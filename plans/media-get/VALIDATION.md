# Media Get — validação da rodada 2026-10-01

Ambiente observado: macOS ARM64; Go 1.27.1 instalado em
`/Users/matheus/.local/share/mise/installs/go/1.27.1/bin`. Baseline e módulo
permaneceram iguais. Nenhum pacote foi instalado, nenhum download real foi
executado e o projeto Deno original foi consultado somente por leitura.

## Execuções

1. Testes focados: `PATH=<Go 1.27.1>/bin:$PATH ./scripts/check-safe.sh test
   ./internal/mediaget/... ./cmd/media-get` — PASS.
2. Primeira execução de `check-safe.sh all` — FAIL no teste
   `tools/release.TestRepositoryCommandsAreDiscovered`: expectativa antiga
   `[ai-profile repo-zip]`; descoberta real `[ai-profile media-get repo-zip]`.
   A função exata foi revisada e a expectativa foi atualizada. Não houve
   alteração no mecanismo de descoberta ou relaxamento do gate.
3. Execução final: `PATH=<Go 1.27.1>/bin:$PATH ./scripts/check-safe.sh all` — PASS,
   exit 0. Executa fmt, `go test ./...`, `go vet ./...`, shuffle/count=3,
   `go test -race ./...`, `release api check`, `release contracts check` e
   `release changes validate` em HOME/cache/TMP/Git sintéticos e offline.
4. Builds CGO_ENABLED=0/trimpath/buildvcs=false com ambiente explícito,
   GOTOOLCHAIN=local/GOPROXY=off: darwin/arm64, darwin/amd64, linux/amd64,
   linux/arm64, windows/amd64 e windows/arm64 — todos PASS (compilação).
5. Binário darwin/arm64: `--version`, `--help`, `version --json` em ambiente
   sintético — PASS. Versão individual observada: `media-get 0.1.0`.

Binário local: `dist/media-get/media-get`. Builds das outras plataformas:
`dist/media-get-check/cross/`. São artifacts locais, não uma release publicada.

## Cobertura nova

- Precedência de destino, variável vazia, default HOME sintético/Downloads.
- Interação, Referer vazio/definido, voltar, cancelar/declinar e automação sem TTY.
- Metadata/estimativa sem tamanho, soma incompleta, bitrate, overflow e infinito.
- SRT manual/automático e escaping do identificador de idioma.
- Falha de dependência antes do spawn/escrita; causas e dicas de instalação.
- Argv de consulta/download, ausência de config/plugins/componentes remotos.
- Ambiente sem tokens/HOME/PYTHONPATH herdados; saída bruta do processo descartada.
- Timeout/cancelamento Unix com morte do grupo/descendente sintético.
- Saída vazia, symlink, incompleta, múltipla; identidade da área alterada.
- Colisão com arquivo/symlink, concorrência, não sobrescrita e recuperação parcial.
- Endpoints estáticos sem HOME/backend e lock de contrato byte a byte.

## Revisão de documentação oficial

O README e parser oficiais do yt-dlp confirmaram templates/progress-delta,
selectors, merge `mp4/mkv`, sub-langs por regex e conversão SRT. A revisão
retirou a flag inexistente `--no-netrc`; `usenetrc` é opt-in/default false no
parser, com config ignorada e sem argumentos de autenticação. Componentes
remotos são explicitamente desativados. Essa é evidência de revisão, não runtime.

- https://github.com/yt-dlp/yt-dlp/blob/master/README.md
- https://github.com/yt-dlp/yt-dlp/blob/master/yt_dlp/options.py
- https://formulae.brew.sh/formula/yt-dlp
- https://formulae.brew.sh/formula/ffmpeg

## Não executado / limites

- yt-dlp/FFmpeg reais, rede, reprodução, vídeo do exemplo e contas reais.
- Runner nativo Linux ou Windows. Cross-build não comprova runtime.
- Fuzz adicional desta rodada (seeds da suíte executaram no teste normal).
- Instalação global, commit, push, preparação de release e publicação.
- Snapshot/checksums: verificados em `snapshots/009/` (CRC, entradas completas,
  contexto idêntico e Base64/SHA-256).

O comando novo é experimental. Sucesso com fakes demonstra o launcher e as
invariantes locais; não prova compatibilidade com um site/extrator upstream.

## Atualização — prévias paralelas e progresso

Go 1.27.1/macOS ARM64. `scripts/check-safe.sh all` PASS: fmt, suíte completa,
vet, shuffle/count=3, race, API pública, contratos e change records.

Regressões novas: estimativas aparecem antes dos prompts de seleção; valores
por selector preservam soma de vídeo/áudio; voltar reutiliza cache; queries
sobrepõem execução com limite de três workers; falha/timeout e parcela
incompleta mostram tamanho indisponível; cancelamento encerra workers sem
iniciar download ou preencher cache; animação aparece enquanto consulta está
bloqueada; progresso expõe bytes/percentual/speed/ETA e etapas; marcador com
texto extra é rejeitado; ETA extremo/NaN são descartados; exec.ExitError é
recuperável via errors.As e saída sensível continua oculta.

O primeiro teste focado da atualização falhou em
TestRefererAndIsolationAppliedToBothCalls porque a contagem antiga considerava
somente evento numérico. A assertion agora valida os três eventos de etapa e
transferência da fixture, inclusive processamento. A suíte completa posterior
passou. Ferramentas/sites reais e jobs nativos remotos continuam não executados.
Sem novos snapshots, commit, push ou release nesta etapa.

Após os ajustes finais de apresentação e preservação da classe de erro da CLI:
`check-safe.sh race ./internal/mediaget/...` PASS. Binário macOS ARM64 recompilado
com CGO_ENABLED=0/trimpath/buildvcs=false e ambiente sintético/offline; smokes
--help, --version e version --json PASS. `git diff --check` PASS.

## CI nativo antes do preparo de v1.3.0

Commit `805e6999036fa11230a6a23a1026719123e431fd`: CI
https://github.com/matheusvcouto/cli-tools/actions/runs/36933364310 — completed/success.
Todos os 12 jobs passaram: testes em Linux/macOS/Windows × x64/ARM64,
workflow-lint, change records, contratos, API pública, shells nativos e
cross-build. Isso comprova runtime das capabilities exercitadas pelos testes
sintéticos; download de media-get no Windows continua unsupported.

Usuário forneceu execução real Twitch com metadados, estimativas e transferência
em andamento. Não foi observada conclusão/reprodução; esta evidência do usuário
não foi usada como fixture. Suite v1.3.0 escolhida expressamente pelo usuário;
prepare --write executado após CI verde, preservando versões independentes.
CI no commit preparado e workflow de release ainda devem passar antes de
declarar publicação concluída. Sem novos snapshots.

Após prepare --write: gates completos locais novamente PASS; preflight v1.3.0
PASS (três ferramentas, nenhum record pendente). Binário em dist/media-get
reconstruído com versão de produto 0.2.0/suíte 1.3.0; smokes help/version/json
PASS. Arquivos de preparação e records arquivados conferidos por leitura.
