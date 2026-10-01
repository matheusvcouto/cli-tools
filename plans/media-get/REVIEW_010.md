# Revisão Media Get — SNAPSHOT-010

Data: 2026-10-01. Escopo: revisão e explicação da implementação atual a partir
do pedido original. Código de produção, testes e contratos não foram alterados.

## Parecer

A arquitetura segue o projeto: entrypoint em cmd/media-get, Spec única do CLI
Core, domínio privado e adapter yt-dlp substituível. Publicação create-exclusive,
colisões, cancelamento Unix e preservação de incompletos possuem testes sintéticos.
A primeira versão é experimental e não foi comprovada com yt-dlp/FFmpeg reais.

## Achados

1. **P2 — legenda depende de consulta que exige formatos de mídia.**
   internal/mediaget/cli/app.go:132 faz Inspect com Selection vazia antes de
   escolher o tipo, inclusive em automação de legenda. adapter.go:113 usa
   dump-single-json/skip-download sem ignore-no-formats-error; o download de
   legenda também não relaxa essa exigência. Uma mídia cujo extrator retorne
   legendas mas nenhum formato baixável pode falhar antes de oferecer legendas.
   O README oficial documenta que no-formats é erro por padrão e que
   ignore-no-formats-error permite metadata nessa condição. Achado por revisão,
   sem reprodução real. Recomenda-se separar descoberta de metadados/legendas
   da seleção de formato, preservando exigência de formato para vídeo/áudio.

2. **P2 — ffprobe checado não é necessariamente o usado.**
   adapter.go:69 resolve ffprobe por PATH ou campo FFprobe, mas descarta o caminho.
   adapter.go:143 passa somente ffmpeg-location ao yt-dlp. O upstream procura
   ffprobe junto do ffmpeg quando essa opção é usada. Instalações em diretórios
   diferentes ou FFprobe explícito podem passar no preflight sem que esse
   executável seja utilizado. Na instalação usual de ambos juntos não há essa
   divergência. O upstream pode usar ffmpeg como fallback para algumas operações;
   portanto isto não implica falha em toda extração. Recomenda-se verificar os
   executáveis efetivamente utilizados e testar layout com diretórios distintos.

3. **P2 — erros de saída perdem a causa interna.**
   adapter.go:185 cria erro textual novo para exec.ExitError, sem Unwrap/%w.
   errors.As deixa de recuperar a causa/código de forma estruturada, contrariando
   o plano e docs/engineering.md. O código aparece na mensagem, mas o stderr é
   descartado e não há diagnóstico específico para ferramenta antiga, dependência
   JS, HTTP ou autenticação. Recomenda-se erro tipado com causa preservada e
   mensagem segura; não expor stderr bruto ou URLs assinadas.

## Decisões de produto tomadas pela IA

- Nome media-get e variável CLI_TOOLS_MEDIA_GET_DOWNLOAD_DIR.
- Repair interpretado como Referer pelo exemplo --referer; não existe reparo
  de arquivo corrompido. Campo opcional, vazio por padrão.
- Menus numerados com Enter, sem navegação com setas.
- Vídeo com extensão efetiva; merge MP4/MKV sem impor H.264/QuickTime.
- Áudio M4A e legenda SRT, sem escolha de MP3/TXT nesta versão.
- Qualidade best ou limites 2160/1080/720/480/360, não todos os formatos de -F.
- Legenda manual preferida: a automática do mesmo idioma é omitida em decodeInfo.
- Downloads deve existir; flag > env > HOME/Downloads. Sem descoberta XDG.
- Colisões recebem sufixos; incompletos ficam em pasta oculta no destino.
- Histórico, retomada, playlists, cookies e outros backends ficam para depois.

O histórico mostra execução autorizada após o planejamento, mas não uma definição
conjunta dessas preferências. Elas são escolhas da primeira versão, não requisitos
expressamente confirmados individualmente pelo usuário.

## Fluxo atual

URL → Referer opcional → consulta de mídia → vídeo/áudio/legenda (se disponível)
→ limite de qualidade/faixa → estimativa → continuar/ajustar/cancelar → nome
→ confirmar → progresso → arquivo publicado. 0 volta nos menus de qualidade/faixa;
q cancela nos menus; Enter na confirmação cancela. Sem terminal exige URL/kind/yes.

## Validação desta rodada

PASS: scripts/check-safe.sh test ./internal/mediaget/... ./cmd/media-get com
Go 1.27.1 em macOS ARM64; três pacotes de testes aprovados e entrypoint compilado.
PASS: git diff --check antes dos documentos novos.
Não executados novamente: gates completos, cross-build, race e fuzz.
Não executados: ferramentas de mídia reais, sites, contas, CI Linux/Windows.
O PASS completo da rodada 009 é histórico registrado, não reexecução desta revisão.
Nenhum pacote instalado, estado real alterado, commit, push ou release.

## Fontes oficiais consultadas

- https://github.com/yt-dlp/yt-dlp/blob/master/README.md
- https://github.com/yt-dlp/yt-dlp/blob/master/yt_dlp/postprocessor/ffmpeg.py

Consulta de master é evidência de revisão atual, não teste de uma versão instalada.
