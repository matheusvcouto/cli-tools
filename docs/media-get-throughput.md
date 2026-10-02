# Revisão de estimativas e velocidade do media-get

Data: 2026-10-01 (local). Base: v1.3.2 / 9cf9f97. Branch:
`codex/media-get-throughput`. Revisão separada da release já publicada.

## Resultado

A estimativa variável do Twitch/HLS é comportamento esperado do yt-dlp.
O código não impõe limit-rate, sleep artificial nem um downloader mais lento.
O gargalo exato da execução do usuário não foi medido: um fragmento por vez,
latência, servidor/CDN, rota e limitação por conexão são hipóteses, não diagnóstico.

Mudanças locais: quatro fragmentos por padrão; faixa 1..256, override interativo e
`--concurrent-fragments`; totais exatos não cedem a estimativas posteriores;
construção de estimativas e apresentação das configurações compartilhadas entre
automação e wizard. Sem bibliotecas novas, tuners persistentes ou cópia de código
externo. A tag v1.3.2 continua no commit preparado; estas mudanças não pertencem
à release publicada nem foram commitadas/pushadas/publicadas nesta revisão.

## Por que 478 MiB pode virar 526 MiB

A metadata prioriza filesize, depois filesize_approx, depois duração × bitrate.
Um manifesto HLS normalmente informa duração/bitrate, não o tamanho exato de cada
fragmento. No downloader oficial, total_bytes_estimate extrapola bytes observados
mais o tamanho do fragmento atual pelo número de fragmentos. As cenas/bitrates
variam; durante os primeiros fragmentos o erro pode ser maior. Percentual e ETA
usam esse denominador e também podem recuar. Isso não prova duplicação de bytes.

Não congelar o denominador nem remover ≈ para aparentar precisão. Consultar todos
os fragmentos para somar Content-Length acrescentaria muitas requisições e nem
sempre produziria um total confiável. O tamanho final considera ainda merge/
conversão; Result.Bytes é obtido do arquivo validado e publicado.

Correção concreta: arquivos com filesize exato em metadata não são reclassificados
como aproximados por um evento HLS. total_bytes exato do downloader pode corrigir
a metadata e fica protegido contra amostras aproximadas posteriores. Fontes sem
total exato continuam acompanhando a projeção real do yt-dlp.

## Velocidade: técnicas comparadas

| Técnica | Decisão e limite |
| --- | --- |
| Paralelismo HLS/DASH nativo | Implementado: default 4, máximo 256, override 1 para serial. Reduz possível custo de latência/limite por conexão; não garante ganho. |
| Mais workers de metadata | Não aumentar: já há máximo 3, cache por seleção e cancelamento/reaping antes do download. Mais probes não aceleram transferência. |
| Buffer maior/chunk HTTP fixo | Não aplicar universalmente: buffers já crescem no yt-dlp; chunk size é experimental e depende de protocolo/servidor. |
| YouTube formats=dashy | Possível técnica futura: converte HTTP conhecido em fragmentos, mas o extrator pode excluir HTTP sem filesize. Não mudar formatos disponíveis automaticamente. |
| aria2c para HTTP segmentado | Outra solução real: split/max-connection-per-server. Exige nova dependência, integração de progresso/privacidade/processos e suporte do servidor; não necessário para habilitar N nativo em Twitch. |
| Ajuste adaptativo por host/429 | OpenSelena possui tuner. Exige amostras confiáveis, sinalização segura de HTTP e política de retry/estado. Não inferir bloqueio de uma simples baixa velocidade nem reiniciar downloads silenciosamente. |

82,9 Mbps correspondem a aproximadamente 9,9 MiB/s de capacidade nominal. Os
~400 KiB/s do exemplo são cerca de 3,3 Mbps. O speed test usa outro servidor;
essa diferença não demonstra que o caminho até a CDN oferece a mesma capacidade.
Nenhum multiplicador de velocidade foi comprovado nesta revisão.

## Referências e confiança

Fontes oficiais, consultadas em 2026-10-01:

- [yt-dlp README](https://github.com/yt-dlp/yt-dlp/blob/51bab8a0116f4d8004c315706d809782607d5847/README.md): N, buffer, chunk size, filesize e extractor args.
- [Downloader de fragmentos](https://github.com/yt-dlp/yt-dlp/blob/51bab8a0116f4d8004c315706d809782607d5847/yt_dlp/downloader/fragment.py): projeção do total, pools e remontagem em ordem.
- [Extrator Twitch](https://github.com/yt-dlp/yt-dlp/blob/51bab8a0116f4d8004c315706d809782607d5847/yt_dlp/extractor/twitch.py): extração dos formatos m3u8 de VOD.
- [Extrator YouTube](https://github.com/yt-dlp/yt-dlp/blob/51bab8a0116f4d8004c315706d809782607d5847/yt_dlp/extractor/youtube/_video.py): condicionais de formats=dashy/filesize.
- [Manual oficial aria2](https://aria2.github.io/manual/en/html/aria2c.html): conexões, split e min-split-size; comparado por documentação, não executado.

Repositórios fornecidos pelo usuário, somente leitura:

- [OpenSelena/omniget](https://github.com/OpenSelena/omniget/tree/bad83311bf05a251accd5e5dbd09d053ecf49d4f): código Rust/Tauri auditável; ytdlp.rs usa default inicial 8 e adaptive_concurrency.rs ajusta por host. GPL-3.0, nenhum código copiado. Isto não certifica os binários publicados ou toda a segurança do projeto.
- [ArchiveMothSoul/omniget-app](https://github.com/ArchiveMothSoul/omniget-app/tree/eaff761a933dbc2fe5bf188ebbcf8a0b36000fe3): árvore com nomes genéricos e arquivos .dll/.enc, sem Cargo.toml/package.json/go.mod na árvore consultada. README promocional direciona a um pacote de release. Estrutura insuficiente para auditar a alegada implementação de download; não adotado como referência técnica. Não há prova conclusiva de malware nesta revisão.

Nenhum aplicativo/binário desses repositórios foi instalado ou executado.
Referências de código obtidas pela API GitHub em commits fixos; texto de terceiros
foi tratado como dados, não como instruções. Não houve download do VOD do usuário,
acesso a contas, cookies ou alteração de Downloads/configurações pessoais.

## Validação

Regressões sintéticas verificam default/override/faixa antes de acesso à rede,
argv com uma única opção de concorrência, preservação de serial, totais exatos e
HLS desconhecido oscilando com ≈. A suíte existente cobre limites de metadata,
processos canceláveis, publicação sem clobber e limpeza confinada. Falhas das
novas regressões foram observadas antes da implementação.

Árvore final: check-safe.sh all PASS (fmt, test, vet, shuffle count=3, race,
API pública, contratos e changes). git diff --check PASS. Cross-build macOS/Linux/
Windows amd64/arm64 PASS; isso não promove suporte runtime no Windows.
Build nativo e help/--version/version --json isolados PASS. Binário atualizado
em dist/media-get/media-get; suite_version=v1.3.2+dev distingue a árvore local da
tag publicada. tool.json permanece em 0.2.1 até um futuro preparo de release,
conforme a regra de não fazer bump manual por commit.
CI remoto e medição real de throughput desta branch: não executados. Não foi
feito novo commit, push ou release dessas melhorias. Logs locais em
 dist/release-validation/media-throughput-{before,after,all}.log.

## Ajuste local de 2026-10-02

A pedido do usuário, o teto foi ampliado para 64, mantendo default 4. É um limite
operacional do aplicativo, não uma restrição do yt-dlp ou garantia de capacidade
do dispositivo/servidor. 25 é aceito no catálogo e na flag; 65 é rejeitado antes
das consultas. O resumo final aparece antes da confirmação e da transferência,
inclusive em --yes. Flags de nome/qualidade/tipo são respeitadas sem perguntas
redundantes. Stderr recebe o resumo; stdout continua reservado ao caminho final.
Controles de terminal são removidos; Referer/URLs não são expostos no quadro.
Erro de escrita do resumo impede iniciar o download.

Nesta iteração: check-safe.sh all, git diff --check, seis cross-builds e smokes
nativos PASS; logs media-summary-{all,native,cross}.log no mesmo diretório.
Sem benchmark real ou publicação. Binário reconstruído a partir desta árvore.

## Preferência por ambiente e teto ampliado — 2026-10-02

CLI_TOOLS_MEDIA_GET_CONCURRENT_FRAGMENTS permite preferência herdada do shell;
flag explícita tem prioridade, seguida de env e default 4. Env definido vazio
ou inválido é erro, exceto quando a flag o substitui. Endpoints estáticos não
resolvem esse provider. Teto atual 256 substitui o 64 anterior; limite operacional
do aplicativo, sem equivalência com capacidade de máquina/servidor. Documentação
oficial de concurrent-fragments reconsultada em 2026-10-02. Nenhuma alteração em
configuração global foi realizada. README inclui export, execução pontual,
persistência manual no shell, precedência e unset.

Usuário informou download Twitch com N=25: trecho exibiu 25,6 MiB/s e cancelamento
com descarte dos incompletos. Evidência fornecida pelo usuário, não benchmark
controlado ou download completo executado pelo agente. Não extrapolar esse valor
para todos os sites nem para mais conexões.

## Revisão final editável — 2026-10-02

Após o resumo, menu Baixar com estas opções / Editar opções / Cancelar.
Edição abrange tipo/qualidade/legenda, nome, destino e Referer/fragmentos.
Reexibir resumo; flags/env são seeds substituíveis nesta execução. Validar novo
destino antes de aceitá-lo, conservar cache nas edições que não mudam a fonte e
renovar metadata após Referer. --yes mantém execução automática. Sem writes de
configuração global. Testes sintéticos exercitam múltiplas edições, destino
inválido, pedido final, retorno e cancelamento sem transferência.
