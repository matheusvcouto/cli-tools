# ADR — media-get

## M001 — Migração por capacidade, não tradução literal — Accepted

Extrair do projeto Deno em `/Users/matheus/pessoal/videos` a escolha de mídia,
qualidade, legenda, estimativa e nome. Implementar domínio Go privado em
`internal/mediaget`, composição em `internal/mediaget/cli` e backend em
`internal/mediaget/ytdlp`. O projeto original é referência somente de leitura;
não migrar seu banco, downloads ou estado do usuário. Nenhuma nova dependência
Go foi necessária: o CLI Core já fornece interação de texto, parsing e contratos.

## M002 — Backend substituível com semântica de domínio — Accepted

`Backend.Check`, `Inspect` e `Download` recebem Source/Selection/Request do
domínio. Selectors, JSON e flags yt-dlp pertencem ao adapter. Não criar registry
por site enquanto só existe um backend. Um extrator futuro pode implementar a
mesma fronteira sem duplicar publicação e fluxo interativo.

## M003 — Dependências externas por operação — Accepted

Sistema fornece yt-dlp e FFmpeg/ffprobe. Metadados só exigem yt-dlp. Merge de
vídeo e conversão SRT exigem FFmpeg; áudio exige também ffprobe. Resolver antes
de escrever, falhar explicitamente e recomendar Homebrew no macOS ou apt em
Debian/Ubuntu. Não instalar nada nem acionar ferramentas reais na suíte offline.

## M004 — Publicação sem clobber e recuperação explícita — Accepted

Trabalho em pasta privada criada através da raiz confinada, no filesystem do
destino. Revalidar identidade física antes/depois do processo. Rejeitar saídas
múltiplas, links, vazias e extensões inesperadas. Hard link confinado publica
sem substituir um destino existente; colisões recebem sufixo. Falha mantém a
área de trabalho; sucesso remove somente a área gerada, revalidada. Não há
rollback de arquivos anteriores porque nunca são substituídos. Processo externo
é uma dependência confiável do sistema, e não é tornado sandbox por `os.Root`.
Não alegar proteção contra binário malicioso ou isolamento contra alterações
hostis simultâneas de toda a árvore enquanto o processo externo escreve.

## M005 — Terminal e processos — Accepted

Interação numerada usa o port `Interaction` do CLI Core, sem ler stdin no handler.
Prompt cancelável permite devolver controle ao entrypoint mesmo durante leitura
bloqueada. Unix executa subprocesso em grupo dedicado; cancelamento mata o grupo
para encerrar também FFmpeg. WaitDelay limita pipes retidos por descendentes.
O adapter de outras plataformas retorna indisponível antes de executar; não
usar shell ou wrapper `.cmd` como fallback. Windows futuro exige implementação
e teste de contenção nativa, não somente cross-build.

## M006 — Privacidade e formatos — Accepted

Desativar config/plugin/cache/netrc; ambiente mínimo. Não logar URLs/Referer ou
stderr bruto. Progresso é numérico. M4A e SRT são contratos explícitos; vídeo
mantém a extensão real e pode ser MP4/MKV. H.264 forçado, histórico, TXT,
playlists, cookies e retomada não entram neste primeiro escopo e só podem entrar
com seus testes e decisões. Transferência estimada não é tamanho final garantido.

Fontes oficiais de revisão (consultadas em 2026-10-01):
- https://github.com/yt-dlp/yt-dlp/blob/master/README.md
- https://github.com/yt-dlp/yt-dlp/wiki/Installation
- https://formulae.brew.sh/formula/yt-dlp
- https://formulae.brew.sh/formula/ffmpeg

## M007 — Estimativas antecipadas e progresso por etapa — Accepted

Antes da escolha, mostrar estimativa de vídeo/áudio no menu de tipo e de cada
limite no menu de qualidade. Consultar selectors reais em até três goroutines
concorrentes, com orçamento total de 20 segundos por lote; falhas/timeouts
resultam em tamanho indisponível. Cache local ao wizard evita repetir consultas
ao voltar/ajustar. Backend.Inspect deve ser concorrente e cancelável. Somente
a goroutine de interação escreve status, menus e animação.

Exibir contador de estimativas concluídas, indicador de carregamento e etapas
de transferência, processamento e publicação. Percentual/bytes/speed/ETA são do
stream em transferência; não prometer percentual global nem de FFmpeg. Aceitar
apenas marcador fixo de processamento e campos numéricos validados do filho.
Erros de saída preservam exec.ExitError internamente com mensagem segura.
