# Isolated account profiles for AI CLIs.
#
# Um único comando genérico (lê TOOLS, nenhum código duplicado por CLI):
#
#   ai-profile <tool> list
#   ai-profile <tool> new <nome>
#   ai-profile <tool> rename <antigo> <novo>
#   ai-profile <tool> delete <nome>
#   ai-profile <tool> run <perfil> ...args     -- roda a CLI isolada
#   ai-profile <tool> acp <perfil> ...args     -- lança o agente ACP isolado
#
# Pra adicionar uma CLI nova, só uma entrada em TOOLS (ver abaixo) — nenhum
# comando novo precisa ser escrito. ACP é opcional por tool (campo `acp`).
#
# Todos os perfis vivem dentro de PROFILES_ROOT, cada um numa pasta com ID
# opaco (ex: ~/.ai-profiles/claude-20260621163245-x7k2). O "nome" que você
# usa nos comandos (alias, ex: "perfil-a") é só um rótulo guardado em
# PROFILES_FILE, nunca o nome real da pasta.
#
# Por quê: muitas CLIs (Claude Code incluso) amarram credenciais ao caminho
# absoluto do config dir (ex: Keychain do macOS é indexado por hash do
# path). Se o "nome" do perfil fosse o nome da pasta, renomear o perfil
# exigiria mover a pasta e invalidaria a credencial. Com o ID fixo e nunca
# reaproveitado para nada visível, renomear um alias nunca precisa mover ou
# tocar a pasta — e como o ID nunca aparece em mais nenhum lugar, um
# `ls ~/.ai-profiles` não fica com nomes "errados" depois de um rename.

use ../platform [require-runtime]
use ../utils [safe-remove]

const PROFILE_NAME_PATTERN = '^[A-Za-z0-9_-]+$'
const PROFILES_ROOT = "~/.ai-profiles"
const PROFILES_FILE = "~/.ai-profiles/index.nuon"

# Pasta de templates de statusLine (ver "apply-statusline" mais abaixo).
# Resolvida a partir do próprio mod.nu (path self), não da pasta onde o nu
# foi iniciado — assim o comando funciona de qualquer lugar.
const STATUSLINE_TEMPLATES_DIR = (path self | path dirname | path join "statusline-templates")

# Cada entrada:
#   - name: valor que você digita como <tool> (ex: "claude")
#   - bin: binário a executar
#   - config_env: env var que a CLI usa pra apontar seu diretório de config
#                 (se a CLI não tiver uma var dedicada, "HOME" funciona, mas
#                 isola só o processo filho rodado pelo with-env, não afeta
#                 o resto do shell)
#   - clear_env: env vars de auth/API key a remover antes de rodar, pra não
#                vazar credencial do perfil ativo do shell principal
#   - acp (opcional): {bin, args} do adapter ACP daquele tool. Presença =
#                o tool é oferecido como agente ACP; ausência = não tem ACP.
#                Lançado com o MESMO isolamento do `run`. Ver acp-integration.md.
const TOOLS = [
    {
        name: "claude"
        bin: "claude"
        config_env: "CLAUDE_CONFIG_DIR"
        clear_env: [
            "CLAUDE_CODE_USE_BEDROCK"
            "CLAUDE_CODE_USE_VERTEX"
            "CLAUDE_CODE_USE_FOUNDRY"
            "ANTHROPIC_API_KEY"
            "ANTHROPIC_AUTH_TOKEN"
            "CLAUDE_CODE_OAUTH_TOKEN"
            "ANTHROPIC_BASE_URL"
        ]
        # adapter externo @agentclientprotocol/claude-agent-acp, embrulha o
        # claude code → respeita CLAUDE_CONFIG_DIR, então isola por perfil.
        acp: { bin: "claude-agent-acp", args: [] }
        support: {
            run: {
                macos: { status: "supported", requires: [{ kind: "command", name: "claude" }] }
                linux: { status: "untested", reason: "isolamento de credenciais ainda não validado no Linux" }
                windows: { status: "untested", reason: "isolamento de credenciais ainda não validado no Windows" }
                default: { status: "unsupported", reason: "sistema não reconhecido pelo config" }
            }
            acp: {
                macos: { status: "supported", requires: [{ kind: "command", name: "claude-agent-acp" }] }
                linux: { status: "untested", reason: "stdio/exec do ACP ainda não validado no Linux" }
                windows: { status: "untested", reason: "stdio/exec do ACP ainda não validado no Windows" }
                default: { status: "unsupported", reason: "sistema não reconhecido pelo config" }
            }
        }
    }
    {
        name: "codex"
        bin: "codex"
        config_env: "CODEX_HOME"
        clear_env: ["OPENAI_API_KEY"]
        # codex NÃO tem subcomando `codex acp` — o ACP é um binário
        # separado `codex-acp`, que respeita CODEX_HOME.
        acp: { bin: "codex-acp", args: [] }
        support: {
            run: {
                macos: { status: "supported", requires: [{ kind: "command", name: "codex" }] }
                linux: { status: "untested", reason: "isolamento de credenciais ainda não validado no Linux" }
                windows: { status: "untested", reason: "isolamento de credenciais ainda não validado no Windows" }
                default: { status: "unsupported", reason: "sistema não reconhecido pelo config" }
            }
            acp: {
                macos: { status: "supported", requires: [{ kind: "command", name: "codex-acp" }] }
                linux: { status: "untested", reason: "stdio/exec do ACP ainda não validado no Linux" }
                windows: { status: "untested", reason: "stdio/exec do ACP ainda não validado no Windows" }
                default: { status: "unsupported", reason: "sistema não reconhecido pelo config" }
            }
        }
    }
]
# "agy" (Antigravity/Google) foi deliberadamente deixado fora deste array.
# A única forma de isolá-lo é via $HOME (não tem env var de config-dir
# própria), mas a credencial real do agy é um item ÚNICO e GLOBAL do
# Keychain de login — não por config dir. Sobrescrever $HOME quebra o
# keychain padrão (dialog "Chaves Não Encontradas" no macOS) e, pior, não
# dá isolamento de verdade entre perfis (todos compartilhariam o mesmo
# item de keychain se o dialog fosse "corrigido"). Decisão: usar o agy
# direto, sem isolamento por perfil — uma conta só, sem hack de $HOME/
# keychain. Ver `agy-keychain-issue.md` para o histórico completo.

def tool-spec [
    tool: string
] {
    let spec = ($TOOLS | where name == $tool | get --optional 0)
    if $spec == null {
        let names = ($TOOLS | get name | str join ', ')
        error make {
            msg: $"CLI desconhecida: ($tool). Disponíveis: ($names)"
        }
    }
    $spec
}

def nu-complete-tools [] {
    $TOOLS | get name
}

def nu-complete-actions [] {
    ["list" "new" "rename" "delete" "run" "acp" "apply-statusline"]
}

def nu-complete-statusline-templates [] {
    ls $STATUSLINE_TEMPLATES_DIR
    | get name
    | path parse
    | where extension == "json"
    | get stem
}

# Completer usado pra qualquer argumento que represente um nome de perfil
# (no "run" e no "rename"/"delete" do ai-profile). Acha a CLI já digitada
# olhando os tokens da linha — nesses comandos o tool é sempre um argumento
# literal (não escondido atrás de alias), então não tem ambiguidade.
def nu-complete-profile-arg [context: string] {
    let tools = ($TOOLS | get name)
    let tool = ($context | split row " " | where {|t| $t in $tools} | get --optional 0)
    if $tool == null {
        []
    } else {
        profile-list $tool
    }
}

def profiles-root-path [] {
    $PROFILES_ROOT | path expand
}

def profiles-file-path [] {
    $PROFILES_FILE | path expand
}

def read-profile-map [] {
    let path = (profiles-file-path)
    if ($path | path exists) {
        open $path
    } else {
        []
    }
}

def write-profile-map [entries: list] {
    mkdir (profiles-root-path)
    $entries | save --force (profiles-file-path)
}

# Caminho usado apenas para criar um perfil novo (primeira vez). O ID é
# opaco e definitivo: nunca é regenerado nem reaproveitado para o alias.
# Formato timestamp+sufixo: ordenável por data de criação só olhando o
# nome, e a chance de colisão (mesma CLI, mesmo segundo, mesmo sufixo de 4
# chars) é desprezível sem precisar implementar ULID/UUID.
def new-profile-dir [
    tool: string
] {
    let stamp = (date now | format date "%Y%m%d%H%M%S")
    let suffix = (random chars --length 4 | str lowercase)
    $"(profiles-root-path)/($tool)-($stamp)-($suffix)"
}

def profile-path [
    tool: string
    profile: string
] {
    read-profile-map
    | where {|e| $e.tool == $tool and $e.alias == $profile}
    | get --optional 0
    | get dir
}

def assert-valid-profile-name [
    profile: string
] {
    if not ($profile =~ $PROFILE_NAME_PATTERN) {
        error make {
            msg: $"Nome de perfil inválido: ($profile). Use apenas letras, números, _ ou -"
        }
    }
}

def profile-list [
    tool: string
] {
    read-profile-map | where tool == $tool | get alias | sort
}

# Formato público da ação `list`. Além do alias, mostra a variável que a
# CLI reconhece e o diretório que deve ser usado como valor. `profile-list`
# continua retornando apenas aliases porque também é usado internamente por
# autocomplete e validação.
def profile-list-details [
    tool: string
] {
    let config_env = (tool-spec $tool | get config_env)

    read-profile-map
    | where tool == $tool
    | sort-by alias
    | each {|entry|
        {
            profile: $entry.alias
            env: $config_env
            dir: ($entry.dir | path expand)
        }
    }
}

def profile-list-message [
    profiles: list<string>
] {
    if ($profiles | is-empty) {
        "nenhum perfil criado"
    } else {
        $profiles | str join ', '
    }
}

def existing-profile-dir [
    tool: string
    profile: string
] {
    assert-valid-profile-name $profile

    let profiles = (profile-list $tool)
    if $profile not-in $profiles {
        error make {
            msg: $"Perfil ($tool) inválido: ($profile). Disponíveis: (profile-list-message $profiles)"
        }
    }

    profile-path $tool $profile
}

# CODEX_HOME isola mais que a credencial: ele também muda onde o Codex
# procura as instruções globais. Mantém auth.json separado por perfil, mas
# faz cada perfil herdar os dois arquivos de guidance do home padrão. Links
# simbólicos deixam alterações em ~/.codex valerem na próxima sessão e não
# sobrescrevem um arquivo que o usuário tenha criado especificamente no
# perfil.
def inherit-codex-global-guidance [
    tool: string
    profile_dir: string
] {
    if $tool == "codex" {
        let global_dir = ("~/.codex" | path expand)

        for filename in ["AGENTS.override.md" "AGENTS.md"] {
            let source = ($global_dir | path join $filename)
            let destination = ($profile_dir | path join $filename)

            if ($source | path exists) and not ($destination | path exists --no-symlink) {
                let link_result = (^ln -s $source $destination | complete)

                # Duas sessões podem tentar criar o mesmo link ao mesmo
                # tempo. Se o destino passou a existir, a outra ganhou a
                # corrida e está tudo certo; qualquer outra falha deve
                # impedir o Codex de abrir sem as instruções esperadas.
                if $link_result.exit_code != 0 and not ($destination | path exists --no-symlink) {
                    let detail = ($link_result.stderr | str trim)
                    let detail_suffix = if ($detail | is-empty) { "" } else { $": ($detail)" }
                    error make {
                        msg: $"Não foi possível herdar ($source) em ($destination)($detail_suffix)"
                    }
                }
            }
        }
    }
}

def create-profile [
    tool: string
    profile: string
] {
    assert-valid-profile-name $profile
    tool-spec $tool | ignore

    let existing = (profile-list $tool)
    if $profile in $existing {
        error make {
            msg: $"Perfil ($tool) já existe: ($profile)"
        }
    }

    let dir = (new-profile-dir $tool)
    mkdir $dir

    let entries = (read-profile-map)
    write-profile-map ($entries | append {
        tool: $tool
        alias: $profile
        dir: $dir
        created_at: (date now | format date "%Y-%m-%dT%H:%M:%S")
    })

    $dir
}

# Renomeia apenas o alias no PROFILES_FILE. O diretório físico (e portanto
# qualquer credencial de keychain/cache amarrada a ele) nunca é tocado, então
# o login da CLI continua valendo depois do rename.
def rename-profile [
    tool: string
    old: string
    new: string
] {
    existing-profile-dir $tool $old | ignore
    assert-valid-profile-name $new

    let existing = (profile-list $tool)
    if $new in $existing {
        error make {
            msg: $"Perfil ($tool) já existe: ($new)"
        }
    }

    let entries = (
        read-profile-map
        | each {|e|
            if $e.tool == $tool and $e.alias == $old {
                $e | update alias $new
            } else {
                $e
            }
        }
    )
    write-profile-map $entries

    profile-path $tool $new
}

def delete-profile [
    tool: string
    profile: string
] {
    let dir = (existing-profile-dir $tool $profile)
    print $"Isso vai apagar permanentemente: ($dir)"

    let typed_profile = (input $"Digite o nome do perfil para confirmar: ")
    if $typed_profile != $profile {
        error make {
            msg: $"Confirmação inválida. Esperado: ($profile)"
        }
    }

    let answer = (input "Confirmar exclusão? [y/N]: " | str lowercase)
    if $answer not-in ["y", "yes"] {
        error make {
            msg: "Exclusão cancelada"
        }
    }

    safe-remove $dir (profiles-root-path)

    let entries = (
        read-profile-map
        | where {|e| not ($e.tool == $tool and $e.alias == $profile)}
    )
    write-profile-map $entries

    $dir
}

# Núcleo genérico de execução: monta o env isolado a partir da entrada em
# TOOLS e roda o binário. Toda CLI passa por aqui — só muda o `tool`.
def run-tool-profile [
    tool: string
    profile: string
    args: list<string>
] {
    let spec = (tool-spec $tool)
    require-runtime {
        name: $"ai-profile ($tool) run"
        support: ($spec.support?.run? | default {
            default: "unsupported"
            reason: $"TOOLS não define support.run para ($tool)"
        })
    }

    let dir = (existing-profile-dir $tool $profile)
    inherit-codex-global-guidance $tool $dir

    let overrides = (
        $spec.clear_env
        | reduce --fold {($spec.config_env): $dir} {|env_name, acc|
            $acc | insert $env_name null
        }
    )

    with-env $overrides {
        run-external $spec.bin ...$args
    }
}

# Lança o adapter ACP do tool com o MESMO isolamento do run, mas:
# - usa `exec` (substitui o processo nu) → stdin/stdout intocados, sinais
#   entregues ao adapter, exit code propagado. ESSENCIAL pra ACP, que fala
#   JSON-RPC por stdout: qualquer byte extra corromperia o stream.
# - diagnóstico humano vai pra stderr (print -e), nunca stdout.
# Ver acp-integration.md e a investigação registrada lá.
def acp-tool-profile [
    tool: string
    profile: string
    args: list<string>
] {
    let spec = (tool-spec $tool)
    if ($spec.acp? | is-empty) {
        error make {
            msg: $"($tool) não tem ACP configurado — falta o campo acp em TOOLS"
        }
    }
    require-runtime {
        name: $"ai-profile ($tool) acp"
        support: ($spec.support?.acp? | default {
            default: "unsupported"
            reason: $"TOOLS não define support.acp para ($tool)"
        })
    }

    let dir = (existing-profile-dir $tool $profile)
    inherit-codex-global-guidance $tool $dir

    let overrides = (
        $spec.clear_env
        | reduce --fold {($spec.config_env): $dir} {|env_name, acc|
            $acc | insert $env_name null
        }
    )

    # stderr — stdout é reservado pro stream JSON-RPC do ACP.
    print -e $"acp: ($spec.acp.bin) tool=($tool) profile=($profile)"

    with-env $overrides {
        exec $spec.acp.bin ...$spec.acp.args ...$args
    }
}

# Cada perfil tem seu próprio settings.json (porque CLAUDE_CONFIG_DIR /
# CODEX_HOME apontam pra ele) — é assim que o isolamento funciona, mas
# significa que chaves do settings.json global (ex: statusLine) NUNCA
# chegam a um perfil sozinhas. "apply-statusline" resolve isso sem quebrar
# o isolamento: mescla só a chave "statusLine" (lida de um template em
# statusline-templates/<nome>.json) no settings.json do perfil, mantendo
# todo o resto (model, plugins, theme...) intocado. É manual e opt-in —
# nada disso roda em "new"/"run"/"acp".
#
# Templates diferentes (statusline-templates/<nome>.json) permitem ter
# statuslines diferentes por perfil (ex: um mais simples pra um perfil de
# família, um mais detalhado pra um perfil de trabalho).
def statusline-template-path [
    template: string
] {
    let path = ($STATUSLINE_TEMPLATES_DIR | path join $"($template).json")
    if not ($path | path exists) {
        let names = (nu-complete-statusline-templates | str join ', ')
        error make {
            msg: $"Template de statusLine inexistente: ($template). Disponíveis: ($names)"
        }
    }
    $path
}

def apply-statusline-to-profile [
    tool: string
    profile: string
    template: string
] {
    tool-spec $tool | ignore
    let dir = (existing-profile-dir $tool $profile)
    let statusline = (open (statusline-template-path $template))

    let settings_path = ($dir | path join "settings.json")
    let settings = if ($settings_path | path exists) {
        open $settings_path
    } else {
        {}
    }

    $settings
    | upsert statusLine $statusline
    | save --force $settings_path

    print $"statusLine \(($template)) aplicada em ($tool)/($profile)"
}

# --wrapped é necessário pro "run" poder repassar flags soltas (ex: --print
# "oi") direto pra CLI de verdade, sem o Nushell tentar interpretá-las como
# flags do próprio ai-profile. tool/action continuam posicionais tipados com
# completer normal — --wrapped só afeta como os tokens finais (...rest) são
# tratados, não os primeiros argumentos.
export def --wrapped "ai-profile" [
    tool: string@nu-complete-tools
    action: string@nu-complete-actions = "list"
    ...rest: string@nu-complete-profile-arg
] {
    tool-spec $tool | ignore

    match $action {
        "list" => (profile-list-details $tool)
        "new" => {
            let name = ($rest | get --optional 0)
            if $name == null {
                error make { msg: "uso: ai-profile <tool> new <nome>" }
            }
            create-profile $tool $name
        }
        "rename" => {
            let old = ($rest | get --optional 0)
            let new = ($rest | get --optional 1)
            if $old == null or $new == null {
                error make { msg: "uso: ai-profile <tool> rename <antigo> <novo>" }
            }
            rename-profile $tool $old $new
        }
        "delete" => {
            let name = ($rest | get --optional 0)
            if $name == null {
                error make { msg: "uso: ai-profile <tool> delete <nome>" }
            }
            delete-profile $tool $name
        }
        "run" => {
            let profile = ($rest | get --optional 0)
            if $profile == null {
                error make { msg: "uso: ai-profile <tool> run <perfil> [...args]" }
            }
            run-tool-profile $tool $profile ($rest | skip 1)
        }
        "acp" => {
            let profile = ($rest | get --optional 0)
            if $profile == null {
                error make { msg: "uso: ai-profile <tool> acp <perfil> [...args]" }
            }
            acp-tool-profile $tool $profile ($rest | skip 1)
        }
        "apply-statusline" => {
            let profile = ($rest | get --optional 0)
            let template = ($rest | get --optional 1 | default "default")
            if $profile == null {
                error make { msg: "uso: ai-profile <tool> apply-statusline <perfil> [template]" }
            }
            apply-statusline-to-profile $tool $profile $template
        }
        _ => {
            error make {
                msg: $"Ação desconhecida: ($action). Use list, new, rename, delete, run, acp ou apply-statusline."
            }
        }
    }
}
