# media-get

Downloader de vídeo, áudio e legendas com yt-dlp instalado no sistema. Produto
experimental. O launcher possui testes nativos sintéticos no macOS e Linux
(x64 e ARM64); funcionamento com sites/FFmpeg reais continua não verificado.
Downloads no Windows são recusados explicitamente. Help, schema, contract e
completion continuam disponíveis sem yt-dlp, FFmpeg ou acesso ao HOME.

## Dependências

Não há instalação automática. A inspeção requer `yt-dlp`. Vídeo com áudio e
conversão de legenda para SRT requerem `ffmpeg`; MP4 compatível e extração de áudio também requerem
`ffprobe` do mesmo diretório utilizado por FFmpeg. Ausências produzem erro indicando o comando:

```sh
# macOS
brew install yt-dlp ffmpeg
# Debian/Ubuntu
sudo apt install yt-dlp ffmpeg
```

Use uma versão atual do yt-dlp que ofereça `--no-plugin-dirs`, `--progress-delta`
e templates de progresso. Pacotes antigos de distribuições podem precisar de
atualização. Certos extratores, incluindo YouTube, podem precisar de runtime JS
ou componentes adicionais mantidos pelo próprio yt-dlp; a CLI não os instala
nem baixa automaticamente. Não há promessa de funcionamento em qualquer site.

## Uso interativo

```sh
media-get
media-get 'https://example.invalid/video'
media-get --referer '' 'https://example.invalid/video'
```

Fluxo padrão: URL → tipo → qualidade ou legenda → revisão das opções → nome →
confirmação. Referer não é perguntado no início. No terminal macOS/Linux, os menus
usam ↑/↓ e Enter; digite para buscar e use Esc para cancelar. Após a seleção,
o menu é substituído por uma linha com a resposta escolhida. Sem terminal de
saída, ou com `TERM=dumb`, permanecem menus numerados (0 volta; q cancela).

Na revisão, **Adicionar configuração** abre um catálogo pesquisável com:

- **Referer:** URL da página de origem; Enter remove. Alterar o Referer renova os
  metadados, as opções disponíveis e as estimativas. Se a primeira consulta falhar,
  também é possível configurar o Referer antes de tentar novamente.
- **Fragmentos paralelos:** 1 a 256, padrão 4. Traduz para `--concurrent-fragments`
  do yt-dlp e pode acelerar HLS/DASH; não garante ganho em qualquer protocolo.

As configurações valem somente para este download. Não há argumentos livres,
cookies ou configurações persistidas. O resumo indica Referer definido sem
repetir seu valor. `--referer` continua disponível para automação.

Vídeo e áudio começam a ser estimados em segundo plano junto com a consulta
inicial da URL. Assim que ela identifica os formatos, as qualidades também
entram na fila, mesmo antes de você escolher vídeo. Menus abrem imediatamente
após identificar a mídia e mostram um indicador animado “calculando…”; os
valores aparecem no próprio menu conforme ficam prontos. Você pode selecionar
ou continuar sem esperar. Consultas de tamanho têm até três workers e orçamento
de 20 segundos por fonte; falhas/timeouts viram tamanho indisponível.

Cache sincronizado somente em memória, mantido durante escolhas, configuração,
nome e confirmação final. Voltar não repete consultas. Trocar Referer encerra a
sessão anterior e renova metadata/cache; configurar fragmentos preserva o cache.
Antes de baixar, encerrar/recolher workers e conservar somente a previsão
selecionada para a barra. Sem arquivos de cache ou consultas durante download.
Terminais numerados recebem uma prévia estática sem bloquear pela estimativa;
menus nativos atualizam as linhas enquanto você navega ou pesquisa.

Limites iguais ou superiores à resolução máxima conhecida são cobertos pela
opção de melhor qualidade, evitando consultas redundantes; quando há altura
desconhecida, os limites ficam disponíveis. Flags continuam aceitando todos os
limites documentados. Legendas são identificadas na consulta inicial, com tamanho
indisponível quando não há dados para calcular.
Após o resumo, escolha **Baixar com estas opções**, **Editar opções** ou
**Cancelar**. Enter confirma a opção selecionada. Ctrl+C encerra os processos e
retorna 130; o estado do terminal e a visibilidade do cursor são restaurados.

A estimativa corresponde à transferência antes de merge/conversão. Soma todos
os streams selecionados e usa tamanho informado, aproximado ou duração ×
bitrate. Uma parte desconhecida torna o total desconhecido, com aviso sem
bloquear o download. Legendas têm estimativa indisponível.

- **Vídeo:** vídeo com áudio, melhor qualidade ou limite de altura. Streams
  separados são mesclados em MP4 quando possível, com MKV como alternativa.
  O arquivo mantém a extensão realmente produzida. No modo automático não há garantia de H.264,
  reprodução no QuickTime ou conversão forçada do vídeo.
- **Áudio:** extração em M4A; qualidade de conversão 256 kbps quando há recodificação.
- **Legenda:** faixa manual ou automática em SRT. A faixa manual é preferida
  quando há ambas no mesmo idioma. O formato TXT está disponível; traduções especiais ficam para depois.

## Destino e arquivos

Precedência: `--output-dir` > `CLI_TOOLS_MEDIA_GET_DOWNLOAD_DIR` > `~/Downloads`.
A variável precisa conter um caminho não vazio. O destino deve existir e ser
um diretório real; a raiz selecionada não pode ser um symlink. Paths relativos
são resolvidos contra o diretório atual; `~` deve ser expandido pelo shell.
No Linux, a resolução de `user-dirs.dirs`/XDG customizado ainda não existe;
configure a variável ou a flag para essa pasta.

```sh
export CLI_TOOLS_MEDIA_GET_DOWNLOAD_DIR="$HOME/Downloads"
media-get --output-dir "$HOME/Downloads" 'https://example.invalid/video'
```

Título saneado é o nome sugerido. Colisões viram `Título (1).mp4`, `(2)`, etc.
A publicação por hard link não substitui arquivos ou symlinks e acontece após
verificar que há um único arquivo regular não vazio. Um filesystem sem suporte
à publicação segura por hard link retorna erro; não há fallback menos seguro.

O trabalho nasce em `Media Get — Incompletos-<aleatório>` (permissão 0700)
no destino, visível no Finder. Sucesso limpa somente essa área gerada.
Falha ou cancelamento interativo oferece **Descartar download incompleto**
(padrão) ou **Manter arquivos**, informando caminho e tamanho. Enter confirma
o descarte; Esc, novo Ctrl+C, EOF ou 30 segundos sem resposta também descartam.
Com `--yes`, falha/cancelamento descarta automaticamente sem pergunta.
Somente a pasta criada pela execução atual é elegível: diretórios antigos não
são varridos nem removidos. Mudança de identidade bloqueia a limpeza; falha na
remoção informa o caminho restante. Não há retomada automática.
Falha na limpeza após sucesso gera aviso e mantém o arquivo entregue.

## Uso sem terminal

```sh
media-get 'https://example.invalid/video' --kind video --quality 1080 --yes
media-get 'https://example.invalid/video' --kind audio --yes
media-get 'https://example.invalid/video' --kind subtitle --subtitle-lang pt --yes
media-get 'https://example.invalid/video' --kind subtitle --subtitle-lang en-orig --auto-subs --yes
media-get 'https://example.invalid/stream.m3u8' --referer 'https://origin.invalid/page' --kind video --yes
```

Sem terminal, URL, `--kind` e `--yes` são obrigatórios. Vídeo sem `--quality`
usará a melhor qualidade disponível. Legenda exige idioma exato; para conhecer
faixas disponíveis, use o fluxo interativo. Stdout contém somente o caminho
final; perguntas, progresso e avisos ficam em stderr. `--name` define o nome
base, sem extensão. `--quality` exige `--kind video`; flags de legenda exigem
`--kind subtitle`.

## Isolamento e limites

Argv não passa por shell. Configurações, plugins, netrc e cache do yt-dlp são
desativados; somente PATH e locale entram no ambiente filho. Não há leitura de
cookies do navegador ou credenciais do usuário, persistência de URLs/Referer,
remoção de guardrails do sistema nem bypass de DRM/autenticação. A URL e o
Referer entram em argv do processo e podem ser visíveis às ferramentas do SO.

Metadados têm timeout de 45 s, limite de 8 MiB e timeout de socket de 20 s.
A barra aparece desde o início com a estimativa escolhida e fica visível também
quando o download é interrompido. O progresso ocupa a mesma linha com barra,
percentual, bytes atuais/total, velocidade e tempo restante. Atualiza até oito vezes por segundo sem acumular
linhas. Totais aproximados usam `≈`; total desconhecido mantém bytes e animação.
Conversão e publicação têm indicador animado, sem percentual fictício.
Saída redirecionada contém logs de etapas e progresso limitado a uma atualização
periódica, sem códigos ANSI. O terminal estreito corta a linha para evitar wraps.
Com estimativa e identidade dos streams, o progresso soma vídeo e áudio uma única
vez e atualiza seus totais com os dados da transferência. A barra usa previsão
aproximada enquanto alguma parcela depender de metadata. Sem previsão, o total
é do stream atual; sem total, a barra fica indeterminada. Totais e ETA decimais
HLS são aceitos. Estimativa não representa o tamanho final após conversão.
Downloads não têm timeout total: o cancelamento mata o grupo de processos no
Unix, incluindo FFmpeg. Saída bruta do filho é descartada porque pode conter
URLs assinadas. Fragmentos indisponíveis abortam o download em vez de entregar mídia com partes
faltantes. Só números validados e marcadores fixos chegam à UI; IDs de formato
são transformados em hashes opacos usados somente para contabilizar streams. Erro de processo
informa etapa, código de saída e sugestões gerais; não reproduz seu stderr.

Testes offline usam apenas backend/executáveis sintéticos. Não comprovam
compatibilidade com sites reais, FFmpeg real ou reprodução do arquivo.

## Bibliotecas de terminal

`golang.org/x/term v0.46.0` detecta TTY/tamanho e controla/restaura modo raw;
`golang.org/x/sys v0.48.0` fornece polling cancelável Unix. Ambas são oficiais Go,
licença BSD e versões/checksums fixados em go.mod/go.sum. A preparação preenche
o cache dedicado; testes offline reutilizam esse cache sem vendor.
Os notices acompanham os arquivos de release. O CLI Core público não foi alterado.

Nos campos de texto em terminal nativo (URL, Referer, nome e confirmações),
setas esquerda/direita movem o cursor; Home/End (Ctrl+A/Ctrl+E) vão ao início/fim;
Backspace/Delete removem antes/depois do cursor. Texto longo usa janela horizontal
para manter a edição na mesma linha. Enter vazio conserva o valor padrão.
Ctrl+C/Esc cancela e restaura o terminal. Cancelamento mostra “Cancelado.”,
com o resultado de descarte/retenção, e continua retornando código 130.

## Velocidade e totais HLS/DASH

O download usa quatro fragmentos paralelos por padrão, sem limite de velocidade.
Esse paralelismo só vale para formatos processados pelo downloader HLS/DASH
nativo do yt-dlp; HTTP progressivo não ganha conexões adicionais por essa opção.
Pode reduzir o efeito de latência/limites por conexão, mas não garante saturar
sua rede nem superar o limite do servidor. Se houver bloqueio ou instabilidade,
use um fragmento. A faixa de 1 a 256 permite experimentar valores como 25:

```sh
media-get --concurrent-fragments 25 'https://example.invalid/video'
media-get --concurrent-fragments 1 --kind video --yes 'https://example.invalid/video'
```

A flag e a configuração interativa compartilham a validação de 1 a 256. Zero,
negativos e valores fora da faixa falham antes de consultar a mídia. Alterar a
concorrência não repete as consultas de estimativa. Nenhuma configuração é salva.

Em HLS (como VODs do Twitch), os tamanhos dos fragmentos variam. A estimativa
inicial usa filesize/filesize_approx ou duração × bitrate; durante a transferência,
o yt-dlp extrapola os fragmentos observados. Por isso o total marcado com ≈,
o percentual e o ETA podem subir ou descer. Um total exato de metadata ou do
downloader tem prioridade sobre amostras aproximadas posteriores. O tamanho do
arquivo final só é conhecido depois de mesclagem/conversão e publicação.
Não há promessa de bytes exatos antecipados para HLS sem consultar todos os
fragmentos, o que adicionaria requisições e atraso.

Revisão, fontes e limites de validação: [auditoria](../../docs/media-get-throughput.md).

## Resumo antes do download

Flags respondem às opções antecipadamente, sem repetir a pergunta de nome:

```sh
media-get 'https://example.invalid/video' --kind video --quality 360 \
  --concurrent-fragments 25 --name meu-video --output-dir /caminho/downloads
```

Antes da confirmação final, o resumo reúne mídia, tipo, qualidade, transferência
estimada, paralelismo, nome base e destino. Em terminal, aparece em um quadro
adaptado à largura; em saída redirecionada, usa texto simples. Referer aparece
somente como definido; seu valor e a URL de origem ficam fora do resumo.
`--yes` elimina perguntas e confirmação, mas mantém o resumo antes da transferência.
O nome é uma base: extensão/container dependem da mídia e do processamento.

256 é um teto de configuração do media-get, não uma medida da capacidade do Mac.
Mais conexões podem ajudar em HLS/DASH, mas também aumentar contenção ou provocar
limitação do servidor. Compare 4, 8, 16 e 25 na mesma fonte para escolher um valor;
o padrão continua 4. HTTP progressivo não usa esse paralelismo.

## Fragmentos por variável de ambiente

`CLI_TOOLS_MEDIA_GET_CONCURRENT_FRAGMENTS` define o paralelismo padrão de vídeo e
áudio quando `--concurrent-fragments` não foi informado. Aceita inteiros de 1 a
256. A prioridade é **flag > variável de ambiente > padrão 4**. Durante o fluxo
interativo, você ainda pode editar esse valor para o download atual; o resumo
mostra o valor efetivo. O programa não grava configuração global.

Para todos os comandos iniciados a partir da sessão atual (zsh/bash):

```sh
export CLI_TOOLS_MEDIA_GET_CONCURRENT_FRAGMENTS=25
media-get
```

Para persistir entre sessões, adicione essa linha `export` ao seu `~/.zshrc`
(ou `~/.bashrc` se usar bash) e abra um novo terminal. Isso alcança os processos
que herdam esse ambiente; aplicativos iniciados fora desse shell podem não
herdá-lo. O media-get não modifica esses arquivos automaticamente.

Para somente uma execução, inclusive acima do teto anterior de 64:

```sh
CLI_TOOLS_MEDIA_GET_CONCURRENT_FRAGMENTS=128 media-get
```

Para substituir a variável em uma execução, use a flag:

```sh
media-get --concurrent-fragments 8
```

Para voltar ao padrão 4:

```sh
unset CLI_TOOLS_MEDIA_GET_CONCURRENT_FRAGMENTS
```

Variável definida mas vazia, zero, números negativos, texto e valores acima de
256 geram erro antes das consultas/download. Use `unset`, não valor vazio, para
remover a preferência. Uma flag válida tem prioridade mesmo se a variável for
inválida. Help/version/schema/contract/completion estática não validam essa
variável nem inicializam o downloader.

O teto de 256 limita o uso de threads/conexões; não é uma recomendação de usar o
máximo. O paralelismo atua nos downloads HLS/DASH nativos, conforme a
[documentação do yt-dlp](https://github.com/yt-dlp/yt-dlp#download-options).
Mais fragmentos podem aumentar memória, arquivos abertos e contenção, ou levar
o servidor a limitar as requisições. Compare os resultados na mesma fonte.

## Editar após o resumo

No menu final, **Editar opções** permite alterar tipo/qualidade/legenda, nome,
diretório de download, Referer e fragmentos paralelos. Valores vindos de flags
ou env são apenas os valores iniciais: a edição vale para este download e não
altera a variável de ambiente nem grava preferências globais.

Após editar, o resumo é exibido novamente antes de escolher **Baixar com estas
opções**. **Voltar** no menu de edição mantém os valores; **Cancelar** encerra sem
iniciar a transferência. O novo diretório deve existir e é validado antes de ser
aceito. Trocar somente nome, destino ou fragmentos reaproveita metadata; mudar
Referer renova a consulta e as escolhas. `--yes` mantém o fluxo automático com
resumo e dispensa o menu final.

## Legendas em SRT ou TXT

Ao escolher somente legenda, selecione a faixa/idioma e depois o formato:
SRT preserva tempos e numeração; TXT contém apenas o texto em UTF-8, sem
marcações de tempo, números de blocos ou tags de formatação. Números falados
são preservados. Trechos repetidos em legendas automáticas sobrepostas são
unidos; repetições em momentos distintos permanecem. Não resume nem traduz.

```sh
media-get 'https://example.com/video' --kind subtitle --subtitle-lang en-orig --auto-subs --subtitle-format txt --name transcricao --yes
```

`--subtitle-format srt` é o padrão para automação. Um nome terminado em `.txt`
ou `.srt` também seleciona o formato, tem precedência sobre a escolha anterior
e não duplica a extensão. O resumo final mostra a opção efetiva.

## Vídeo MP4 compatível

Depois da qualidade, o menu oferece Automático ou MP4 compatível. Automático
mantém os codecs da fonte e pode resultar em MKV, MP4 ou outro contêiner.
MP4 prioriza H.264/AAC na seleção do yt-dlp e verifica os codecs com ffprobe.
Quando possível copia as faixas sem perda; recodifica somente as faixas
incompatíveis para H.264 em yuv420p/AAC. Recodificação pode demorar, consumir CPU,
alterar a qualidade e mudar o tamanho final. É explicitada no menu/resumo.
Não há garantia de reprodução em todo dispositivo/player.

```sh
media-get 'https://example.com/video' --kind video --video-format mp4 --name video --yes
media-get 'https://example.com/video' --kind video --name video.mp4 --yes
```

Um nome terminado em `.mp4` solicita MP4 compatível, inclusive quando editado na
revisão final, e a extensão aparece uma única vez. Tem precedência sobre
`--video-format auto`. MP4 requer ffmpeg e ffprobe do mesmo pacote, com encoders
libx264/AAC disponíveis se for preciso recodificar. Falhas não publicam resultado
parcial nem substituem arquivos existentes; o fluxo de incompletos é mantido.
A conversão usa a área privada do download e é cancelável junto aos subprocessos.
