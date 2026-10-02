# media-get

Downloader de vídeo, áudio e legendas com yt-dlp instalado no sistema. Produto
experimental. O launcher possui testes nativos sintéticos no macOS e Linux
(x64 e ARM64); funcionamento com sites/FFmpeg reais continua não verificado.
Downloads no Windows são recusados explicitamente. Help, schema, contract e
completion continuam disponíveis sem yt-dlp, FFmpeg ou acesso ao HOME.

## Dependências

Não há instalação automática. A inspeção requer `yt-dlp`. Vídeo com áudio e
conversão de legenda para SRT requerem `ffmpeg`; extração de áudio também requer
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
- **Fragmentos paralelos:** 1 a 8, padrão 1. Traduz para `--concurrent-fragments`
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
A confirmação aceita s/sim/y/yes; Enter cancela. Ctrl+C encerra os processos e
retorna 130; o estado do terminal e a visibilidade do cursor são restaurados.

A estimativa corresponde à transferência antes de merge/conversão. Soma todos
os streams selecionados e usa tamanho informado, aproximado ou duração ×
bitrate. Uma parte desconhecida torna o total desconhecido, com aviso sem
bloquear o download. Legendas têm estimativa indisponível.

- **Vídeo:** vídeo com áudio, melhor qualidade ou limite de altura. Streams
  separados são mesclados em MP4 quando possível, com MKV como alternativa.
  O arquivo mantém a extensão realmente produzida. Não há garantia de H.264,
  reprodução no QuickTime ou conversão forçada do vídeo.
- **Áudio:** extração em M4A; qualidade de conversão 256 kbps quando há recodificação.
- **Legenda:** faixa manual ou automática em SRT. A faixa manual é preferida
  quando há ambas no mesmo idioma. TXT e traduções especiais ficam para depois.

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
licença BSD, versões/checksums fixados e fontes em `vendor/` para testes offline.
Os notices acompanham os arquivos de release. O CLI Core público não foi alterado.

Nos campos de texto em terminal nativo (URL, Referer, nome e confirmações),
setas esquerda/direita movem o cursor; Home/End (Ctrl+A/Ctrl+E) vão ao início/fim;
Backspace/Delete removem antes/depois do cursor. Texto longo usa janela horizontal
para manter a edição na mesma linha. Enter vazio conserva o valor padrão.
Ctrl+C/Esc cancela e restaura o terminal. Cancelamento mostra “Cancelado.”,
com o resultado de descarte/retenção, e continua retornando código 130.
