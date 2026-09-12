# Guards reutilizáveis para runtime/plataforma. Não chame estes guards no
# topo de config.nu/env.nu: startup deve continuar abrindo em qualquer OS.

export def command-exists [name: string]: nothing -> bool {
    let matches = (which $name)
    $matches | any {|cmd|
        let kind = ($cmd.type? | default "")
        let path = ($cmd.path? | default "")
        # `which` pode devolver um "external" com path == nome bare (não
        # resolvido) quando $env.PATH está vazio — nesse caso ele cai de
        # volta pro diretório do próprio binário nu em execução. O
        # `path exists` extra filtra esse falso positivo.
        $kind == "external" and ($path | is-not-empty) and (($path | path expand) | path exists)
    }
}

export def missing-commands [names: list<string>]: nothing -> list<string> {
    $names | where {|name| not (command-exists $name)}
}

def record-like [value] {
    $value | describe | str starts-with "record"
}

def list-like [value] {
    let shape = ($value | describe)
    ($shape | str starts-with "list") or ($shape | str starts-with "table")
}

def normalize-support-rule [rule] {
    if (record-like $rule) {
        $rule
    } else {
        { status: $rule }
    }
}

def runtime-support-rule [spec: record] {
    let support = ($spec.support? | default { default: "unsupported" })
    let raw = (
        $support
        | get --optional $nu.os-info.name
        | default ($support | get --optional default | default "unsupported")
    )

    normalize-support-rule $raw
}

def dependency-target [dep: record] {
    let kind = ($dep.kind? | default "command")
    if $kind == "path" {
        $dep.path? | default ""
    } else {
        $dep.name? | default ""
    }
}

def check-dependency [dep] {
    if not (record-like $dep) {
        return {
            ok: false
            kind: "spec"
            target: "requires"
            reason: "dependência deve ser record"
            hint: "use { kind: command, name: ... }, { kind: path, path: ... } ou { kind: env, name: ... }"
        }
    }

    let kind = ($dep.kind? | default "command")
    let target = (dependency-target $dep)
    let hint = ($dep.hint? | default null)

    if ($target | is-empty) {
        return {
            ok: false
            kind: $kind
            target: "sem nome"
            reason: $"dependência do tipo ($kind) sem campo obrigatório"
            hint: $hint
        }
    }

    match $kind {
        "command" => {
            if (command-exists $target) {
                { ok: true }
            } else {
                {
                    ok: false
                    kind: $kind
                    target: $target
                    reason: "comando ausente no PATH"
                    hint: $hint
                }
            }
        }
        "path" => {
            let expanded = ($target | path expand)
            if ($expanded | path exists) {
                { ok: true }
            } else {
                {
                    ok: false
                    kind: $kind
                    target: $target
                    reason: $"caminho não existe: ($expanded)"
                    hint: $hint
                }
            }
        }
        "env" => {
            let value = ($env | get --optional $target)
            let present = if $value == null {
                false
            } else if (($value | describe) == "string") {
                not ($value | is-empty)
            } else {
                true
            }

            if $present {
                { ok: true }
            } else {
                {
                    ok: false
                    kind: $kind
                    target: $target
                    reason: "variável de ambiente ausente ou vazia"
                    hint: $hint
                }
            }
        }
        _ => {
            {
                ok: false
                kind: $kind
                target: $target
                reason: "tipo de dependência desconhecido"
                hint: "use command, path ou env"
            }
        }
    }
}

# Retorna diagnóstico estruturado. Não joga erro.
export def check-runtime [spec: record] {
    let feature = ($spec.name? | default "feature")
    let os = $nu.os-info.name
    let raw_support = ($spec.support? | default null)

    if $raw_support != null and not (record-like $raw_support) {
        return {
            ok: false
            feature: $feature
            os: $os
            status: "invalid"
            failures: [
                {
                    kind: "spec"
                    target: "support"
                    reason: "support deve ser record"
                    hint: "use { default: supported } ou um mapa por OS"
                }
            ]
        }
    }

    let support = (runtime-support-rule $spec)
    let status = ($support.status? | default "unsupported")
    mut failures = []

    if $status == "unsupported" {
        let reason = ($support.reason? | default "não suportado neste sistema")
        $failures = ($failures | append {
            kind: "os"
            os: $os
            status: $status
            reason: $reason
            hint: ($support.hint? | default null)
        })
    } else if $status == "untested" {
        let reason = ($support.reason? | default "ainda não testado neste sistema")
        $failures = ($failures | append {
            kind: "os"
            os: $os
            status: $status
            reason: $reason
            hint: ($support.hint? | default null)
        })
    } else if not ($status in ["supported" "partial"]) {
        $failures = ($failures | append {
            kind: "os"
            os: $os
            status: $status
            reason: "status de suporte inválido"
            hint: "use supported, partial, untested ou unsupported"
        })
    }

    if $status in ["supported" "partial"] {
        # requires vem de dois lugares e os dois se aplicam juntos: spec.requires
        # é a dependência universal do feature (ex: o binário principal), e
        # support.<os>.requires é específica daquele OS (ex: um wrapper só no
        # Windows). Nenhum dos dois substitui o outro.
        let spec_requires = ($spec.requires? | default [])
        let support_requires = ($support.requires? | default [])
        mut requires = []

        if (list-like $spec_requires) {
            $requires = ($requires | append $spec_requires)
        } else {
            $failures = ($failures | append {
                kind: "spec"
                target: "requires"
                reason: "requires deve ser list"
                hint: "use uma lista de records de dependência"
            })
        }

        if (list-like $support_requires) {
            $requires = ($requires | append $support_requires)
        } else {
            $failures = ($failures | append {
                kind: "spec"
                target: "support.requires"
                reason: "requires deve ser list"
                hint: "use uma lista de records de dependência"
            })
        }

        let dep_failures = (
            $requires
            | each {|dep| check-dependency $dep}
            | where ok == false
            | each {|failure| $failure | reject ok}
        )
        $failures = ($failures | append $dep_failures)
    }

    {
        ok: ($failures | is-empty)
        feature: $feature
        os: $os
        status: $status
        failures: $failures
    }
}

export def runtime-ok [spec: record]: nothing -> bool {
    let result = (check-runtime $spec)
    $result.ok
}

def format-runtime-failure [failure: record] {
    let kind = ($failure.kind? | default "runtime")
    let target = ($failure.target? | default "")
    let reason = ($failure.reason? | default "falha sem detalhe")
    let hint = ($failure.hint? | default null)
    let hint_text = if $hint == null { "" } else { $" — ($hint)" }

    if $kind == "os" {
        let os = ($failure.os? | default $nu.os-info.name)
        let status = ($failure.status? | default "unsupported")
        $"OS ($os) está ($status): ($reason)($hint_text)"
    } else {
        $"($kind) ($target): ($reason)($hint_text)"
    }
}

# Aborta com erro claro se o runtime não satisfaz a política declarada.
# Em caso de sucesso, não imprime nada. Isso é essencial para comandos ACP.
export def require-runtime [spec: record] {
    let result = (check-runtime $spec)
    if not $result.ok {
        let details = (
            $result.failures
            | each {|failure| format-runtime-failure $failure}
            | str join "\n- "
        )
        error make {
            msg: $"($result.feature) não pode executar em ($result.os)\n- ($details)"
        }
    }
}

export def require-command [name: string, hint?: string] {
    require-runtime {
        name: $"comando ($name)"
        support: {
            default: "supported"
        }
        requires: [
            {
                kind: "command"
                name: $name
                hint: $hint
            }
        ]
    }
}

def clipboard-spec [os: string] {
    if $os == "windows" {
        {
            copy: { cmd: "clip.exe", args: [] }
            paste: { cmd: "powershell", args: ["-NoProfile" "-Command" "Get-Clipboard"] }
        }
    } else if $os == "macos" {
        {
            copy: { cmd: "pbcopy", args: [] }
            paste: { cmd: "pbpaste", args: [] }
        }
    } else {
        # Linux/BSD: xclip na seleção clipboard.
        {
            copy: { cmd: "xclip", args: ["-selection" "clipboard"] }
            paste: { cmd: "xclip", args: ["-selection" "clipboard" "-o"] }
        }
    }
}

export def clip-copy [] {
    let spec = (clipboard-spec $nu.os-info.name).copy
    require-runtime {
        name: "clipboard copy"
        support: {
            default: "supported"
        }
        requires: [
            {
                kind: "command"
                name: $spec.cmd
            }
        ]
    }
    $in | ^$spec.cmd ...$spec.args
}

export def clip-paste [] {
    let spec = (clipboard-spec $nu.os-info.name).paste
    require-runtime {
        name: "clipboard paste"
        support: {
            default: "supported"
        }
        requires: [
            {
                kind: "command"
                name: $spec.cmd
            }
        ]
    }
    ^$spec.cmd ...$spec.args
}
