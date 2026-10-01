# media-get

Downloader de vídeo, áudio e legendas com yt-dlp instalado no sistema. Produto
experimental. O launcher possui testes nativos sintéticos no macOS e Linux
(x64 e ARM64); funcionamento com sites/FFmpeg reais continua não verificado.
Downloads no Windows são recusados explicitamente. Help, schema, contract e
completion continuam disponíveis sem yt-dlp, FFmpeg ou acesso ao HOME.

## Dependências

Não há instalação automática. A inspeção requer `yt-dlp`. Vídeo com áudio e
conversão de legenda para SRT requerem `ffmpeg`; extração de áudio também requer
`ffprobe` (distribuído com FFmpeg). Ausências produzem erro indicando o comando:

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

Perguntas: URL → Referer opcional (Enter: nenhum) → tipo → qualidade ou faixa de
legenda → estimativa → continuar/ajustar/cancelar → nome → confirmação. A seleção
usa números e Enter. Os menus de tipo e qualidade mostram o tamanho estimado
ao lado de cada opção, antes da seleção. As consultas usam até três goroutines
em paralelo, com limite de 20 segundos por lote e cache durante o fluxo.
Enquanto carregam, a CLI mostra um indicador animado no terminal e o contador
de estimativas concluídas (por exemplo, `3/5`). Em saída redirecionada, mostra
linhas de status sem animação. Falhas aparecem como `tamanho indisponível` e
não bloqueiam a escolha. `0` volta nas escolhas de qualidade/legenda; `q` cancela
nos menus. A confirmação aceita `s`, `sim`, `y` e `yes`; Enter cancela. Ctrl+C
encerra os processos do download e retorna 130.

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

O trabalho nasce em `.media-get-<aleatório>` (permissão 0700) no destino.
Sucesso limpa somente essa área gerada. Falha ou cancelamento preserva arquivos
incompletos e informa o caminho. Não há retomada automática nem comando de
limpeza nesta versão. Falha na limpeza após sucesso gera aviso e mantém o
arquivo entregue.

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
O progresso de transferência mostra percentual, bytes, velocidade e tempo
restante quando disponíveis. O percentual é de cada stream: vídeo e áudio
podem ter transferências separadas. Conversão/mesclagem e verificação/publicação
mostram suas etapas sem inventar um percentual de processamento.
Downloads não têm timeout total: o cancelamento mata o grupo de processos no
Unix, incluindo FFmpeg. Saída bruta do filho é descartada porque pode conter
URLs assinadas. Só campos numéricos de progresso e marcadores fixos de etapa chegam à UI. Erro de processo
informa etapa, código de saída e sugestões gerais; não reproduz seu stderr.

Testes offline usam apenas backend/executáveis sintéticos. Não comprovam
compatibilidade com sites reais, FFmpeg real ou reprodução do arquivo.
