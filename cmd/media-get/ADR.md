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
sem substituir um destino existente; colisões recebem sufixo. Falha segue a política de descarte/retenção de M011; sucesso remove somente
a área gerada, revalidada. Não há
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

## M008 — Menus pesquisáveis e controle nativo de terminal — Accepted

A interface numérica permanece fallback para saída redirecionada/TERM=dumb;
macOS/Linux com stdin/stderr TTY usam selector com setas, Enter, busca textual e
Esc. Interação injetada na composição; nenhum comando ou flag duplicado fora do
CompiledApp. O core público permanece intacto. As linhas são limitadas à largura
e removem controles/bidi de dados externos; redraw usa somente escapes próprios.

A stdlib não oferece modo raw/restauração portáveis nem detecção real de TTY.
Adotar golang.org/x/term v0.46.0 e x/sys v0.48.0 (Go oficial, BSD 3-Clause;
x/sys única transitiva de x/term). x/sys também fornece Poll para cancelar reads
Unix em até aproximadamente 50 ms sem goroutine leitora residual. Restaurar modo
raw com defer inclusive em erro/Ctrl+C/SIGTERM e restaurar cursor. Downloads
Windows seguem indisponíveis. Terminais normais de texto usam o port existente.

Fontes upstream/README/licença/código Unix revisados em 2026-10-01. Bibliotecas
não executam scripts de instalação; sys inclui geradores/fontes nativas para
outros sistemas, mas os seis alvos usam código Go com CGO_ENABLED=0. go.sum fixa
integridade; a partir da revisão v1.3.2, caches preparados permitem checks
offline sem vendor (ADR da suíte D023). Não rodar geradores.
THIRD_PARTY_NOTICES.txt é incluído na raiz dos archives de release.

## M009 — Configuração opcional e progresso honesto — Accepted

Referer sai do fluxo inicial e fica em catálogo fechado na revisão final; falha
na consulta oferece configurar/repetir/cancelar. Alterar Referer invalida cache
e renova metadados e seleção. Fragmentos paralelos (1..8) afetam só download,
default yt-dlp 1; alterar não repete queries. Nenhuma configuração é persistida
nem habilita flags livres/cookies. --referer e demais flags ficam compatíveis.

Uma goroutine desenha barra/status a cada 125 ms enquanto backend roda separado;
eventos têm canal limitado. Callback não escreve diretamente no terminal.
Templates do filho têm intervalo de 0.2 s. Percentual é por stream; total
estimado recebe ≈ e ausência não inventa percentual. Processamento/publicação
mostram animação. Erro de escrita no progresso cancela subprocesso e preserva
incompletos. Falta de fragmento aborta para evitar publicação silenciosa parcial.
Inspeção inicial/legendas tolera ausência de formatos de vídeo; seleções reais
de vídeo/áudio continuam exigindo formatos disponíveis.

Referências: https://pkg.go.dev/golang.org/x/term e
https://github.com/yt-dlp/yt-dlp/blob/master/README.md .

## M010 — Correção do progresso HLS e contabilização dos streams — Accepted

A revisão após execução do usuário encontrou totais HLS fracionários descartados
por ParseInt, perda da previsão selecionada entre wizard e renderer e limpeza
da última linha em erro. Aceitar números finitos/bounded (inteiros, decimais e
notação científica); enviar a previsão ao renderer; desenhar desde preparação,
animar antes do primeiro byte e preservar a última linha na saída/cancelamento.

O template agrega info.format_id em JSON. O adapter transforma IDs (até 128 bytes)
em hashes SHA-256 opacos também usados no metadata; nenhum ID bruto é exibido.
Ao existir previsão global, somar bytes por identidade (até 16 streams), sem
somar eventos repetidos. Refinar partes previstas com totais recebidos, conservar
≈ enquanto alguma parte ainda é aproximada e estimar ETA global a partir dos
bytes restantes/speed. Na ausência de identidades de metadata, conservar a
previsão global aproximada; na ausência de previsão, mostrar o stream atual.
Não inferir fronteiras de streams apenas porque bytes diminuíram. Estimativa
não certifica conclusão: limitar percentual aproximado a 99%, mostrar etapas
reais de processamento/publicação e tamanho final somente após publicar.
Esta decisão substitui a apresentação exclusivamente por stream de M007/M009.

Após confirmar um selector nativo, substituir somente suas linhas por um resumo;
não apagar histórico externo ao menu. Se todas as alturas de vídeo são conhecidas,
melhor qualidade cobre o máximo e caps iguais/superiores são redundantes: omitir
esses caps no menu/query sem remover valores das flags. Descartar formatos sem
codec de vídeo ao formar alturas. Alturas desconhecidas mantêm caps.

Check de áudio valida o ffprobe que yt-dlp realmente resolve ao lado do caminho
ffmpeg fornecido. Um probe explícito diferente é rejeitado por identidade física
(os.Stat + SameFile); não confiar só em basename ou PATH.

Revisão oficial: yt-dlp/downloader/fragment.py calcula total_bytes_estimate com
divisão real; downloader/common.py admite total/ETA numéricos e expõe info no
template; YoutubeDL.py propaga format_id de cada formato separado ao downloader.
Fontes consultadas em 2026-10-01 no repositório oficial yt-dlp. Sem inferir
compatibilidade com uma instalação real somente por essa revisão.

## M011 — Descarte padrão e retenção visível — Accepted

Por solicitação do usuário, falhas/cancelamentos descartam incompletos por padrão.
Request.KeepIncomplete delega uma decisão explícita à CLI interativa; o domínio
retorna um handle privado após encerrar o backend. Renderer termina antes de
mostrar a escolha, evitando escrita concorrente. A decisão usa novo contexto
cancelável por sinais e prazo de 30 s, pois o contexto de download já pode estar
cancelado. Enter/default, Esc, outra interrupção, EOF e timeout descartam; manter
exige seleção afirmativa. --yes não delega, descarta sem interação.

Pasta privada visível no mesmo filesystem: Media Get — Incompletos-<aleatório>.
Handle guarda identidades da raiz e da pasta, revalidadas antes da remoção
confinada. Tamanho soma arquivos regulares sem seguir symlinks. Falha de limpeza
informa caminho; erro original/130 permanece reconhecível. Nunca varrer ou
remover pastas de execuções anteriores. Não há retomada automática nem garantia
de limpeza após SIGKILL/crash/desligamento; nenhum cleanup pode rodar nesse caso.
Esta decisão substitui a preservação automática descrita em M004/M009.

## M012 — Prefetch e menus não bloqueantes — Accepted

Substituir lotes aguardados de M007 por sessão sincronizada em memória. Iniciar
vídeo/áudio junto da inspeção inicial (duas estimativas mais metadata, máximo três
probes ativos); após metadata enfileirar qualidades aplicáveis e eventual flag
explícita, antes da escolha. Três workers, fila limitada, deduplicação por
Selection e prazo de 20 s por fonte. Falhas/ausências são cacheadas. Legendas
continuam na metadata inicial sem estimativa especulativa de SRT.

Selector nativo aceita snapshot de labels e lê input com Poll de 150 ms para
atualizar spinner/tamanhos sem goroutine escrevendo no terminal. Busca usa labels
estáveis sem sufixo de tamanho; índice da opção não muda ao receber estimativa.
Fallback numérico não espera e mostra snapshot estático; nunca escrever updates
concorrentes sobre prompts de texto. Seleção/revisão não aguarda estimates.

Manter sessão durante nome/confirmação. Encerrar/recolher workers antes de
baixar, transferindo a última estimativa pronta à barra; não esperar tamanho
pendente para iniciar download. Trocar Referer encerra probes/cache antigos
antes de atualizar metadata; erro/cancelamento em qualquer passo também encerra
workers. Não persistir URLs, estimates ou metadata; nenhuma dependência nova.
Automação --yes mantém apenas a consulta da seleção solicitada.

## M013 — Edição nativa de texto e cancelamento legível — Accepted

O leitor de linha simples do core não interpreta setas. Somente a composição
media-get em TTY Unix passa a editar runes com cursor, inserção, Backspace/Delete,
Home/End e viewport horizontal de largura limitada. Usar o mesmo Poll/key decoder
cancelável; consumir sequências CSI completas, incluindo modificadores e
marcadores de paste, sem colocar escapes em nomes/URLs. Nenhuma dependência nova.
Fallback numerado/redirecionado e API pública do core permanecem intactos.

ask chama o editor nativo de forma síncrona: restauração raw deve terminar antes
de devolver cancelamento ao entrypoint; o fallback mantém estratégia cancelável
existente. Sanitizar controles/bidi no desenho. Limite de 8192 runes; não prometer
edição por grapheme/word ou histórico. Valor padrão continua placeholder e Enter
vazio o aceita. O diagnóstico de cancelamento não é falha técnica: exibir
Cancelado e resultado de cleanup via erro tipado, conservando Unwrap e saída 130.
Falhas normais continuam diagnósticos com causa; falha de cleanup mantém caminho.

## M014 — Paralelismo limitado e prioridade de tamanho exato — Accepted

Pedido do usuário: revisar totais variáveis e melhorar velocidade após concluir
v1.3.2. Fonte oficial confirma N=1 como default do yt-dlp e paralelismo em
hlsnative/DASH. media-get passa a adotar quatro fragmentos, com faixa existente
1..8 e override explícito por --concurrent-fragments e pelo catálogo interativo.
Zero no modelo interno resolve o mesmo default; zero explícito na flag é erro.
Enviar a opção uma única vez, preservar modo serial e abortar fragmentos ausentes.
Nenhum limit-rate, downloader externo, componente remoto ou configuração pessoal
é acrescentado. M009 é substituída apenas quanto ao default e à flag; alterar
concorrência continua sem invalidar metadata, pois selectors não mudam.

Totais reais (filesize/total_bytes) prevalecem sobre total_bytes_estimate. Eventos
exatos podem corrigir metadata; amostras aproximadas posteriores não degradam o
valor exato. HLS desconhecido continua oscilando com ≈; não congelar um número
como se fosse real. Somar por StreamID e substituir snapshots, sem contar eventos
repetidos. Unificar construção da previsão entre automação e prefetch.

OpenSelena/omniget foi referência de leitura para N limitado/controle por host;
nenhum código GPL foi copiado nem programa instalado. Não adotar automaticamente
seu tuner/aria2c/chunk size/clientes/flags: não são universais. O extrator oficial
YouTube documenta formats=dashy, mas seu código pode omitir HTTP sem filesize
quando solicitado; ativação automática alteraria formatos disponíveis e exige
validação específica. Não aplicar nesta rodada. Medição real de throughput/site
continua pendente; argv/testes sintéticos comprovam política, não ganho de rede.
Fontes pinadas e análise: docs/media-get-throughput.md.

## M015 — Paralelismo até 64 e revisão final (2026-10-02)

Pedido explícito para aceitar 25 fragmentos e revisar opções antes do download.
Substitui apenas o teto 8 de M014 por 64, preservando default 4 e validação única.
Não deduzir throughput da potência do computador; o downloader/servidor determinam
o ganho real. Nenhum limite de velocidade ou backend adicional é introduzido.

Resumo final em stderr antes da confirmação/transferência, também em --yes:
mídia, tipo, qualidade/faixa, estimativa, paralelismo, nome base e destino.
Quadro limitado à largura do terminal, texto simples quando redirecionado;
sanear controles e ocultar URLs/Referer. Falha de escrita aborta antes do download.
--name explícito dispensa a pergunta de nome; --yes permanece opt-in explícito.
Cobrir faixa 25/64/65, revisão antes da confirmação, largura, privacidade e falha
com regressões sintéticas. Teste real de desempenho continua pendente do usuário.

## M016 — Preferência de fragmentos por ambiente e teto 256 (2026-10-02)

Pedido explícito para variável global e valores acima de 64. Usar o provider
canônico CLI_TOOLS_MEDIA_GET_CONCURRENT_FRAGMENTS, resolvido lazily pelo CLI Core:
flag > env > default 4. O catálogo pode mudar a preferência somente na execução.
Env definido vazio/inválido falha antes de probes; flag explícita evita resolver
env, inclusive inválido. Endpoints estáticos não o leem/validam. Não escrever no
shell do usuário. Substitui teto 64 de M015 por 256 como limite operacional de
recursos, sem promessa de throughput. Codec numérico validado, sem enumerar 256
valores nos contratos/completions. Testes sintéticos cobrem limites, precedência,
resumo efetivo, endpoints estáticos e repasse único ao yt-dlp.

## M017 — Edição depois do resumo final (2026-10-02)

Pedido explícito para editar na etapa final. Substituir s/N por seletor Baixar /
Editar opções / Cancelar, usando interação existente. Permitir nome, destino,
tipo/qualidade/legenda e catálogo Referer/fragmentos. Flags/env sem --yes apenas
inicializam valores; edição explícita pode substituí-los durante esta execução.
Reexibir resumo após editar/voltar. Validar destino antes de aceitá-lo; reutilizar
sessão/cache salvo Referer alterado, que encerra workers e renova metadata.
Wizard de edição ignora seeds de qualidade/legenda das flags. --yes preserva
fluxo automático. Sem gravação global. Regressões sintéticas verificam pedido
final efetivo, destino inválido, resumos atualizados, voltar e cancelar sem baixar.
