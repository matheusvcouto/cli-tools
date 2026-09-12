# Validação e composição de nomes/caminhos usados pelo repo-zip.

use ./common.nu [fail]


def validate-component [value: string label: string] {
    if ($value | is-empty) {
        fail $"($label) não pode ser vazio"
    }

    if $value != ($value | str trim) {
        fail $"($label) não pode começar ou terminar com espaços"
    }

    if (($value | str contains "/") or ($value | str contains "\\")) {
        fail $"($label) deve ser apenas um nome, sem diretórios"
    }

    if $value in ["." ".."] {
        fail $"($label) inválido: ($value)"
    }

    $value
}


export def normalize-base-name [value: string] {
    let checked = (validate-component $value "--name")
    let parsed = ($checked | path parse)
    let extension = ($parsed.extension? | default "" | str lowercase)
    let base = if $extension == "zip" {
        $parsed.stem
    } else {
        $checked
    }

    validate-component $base "--name"
}


export def validate-suffix [value: string label: string] {
    let checked = (validate-component $value $label)

    # Mantém nomes automáticos portáveis e previsíveis.
    if not ($checked =~ '^[A-Za-z0-9][A-Za-z0-9._-]*$') {
        fail $"($label) aceita apenas letras, números, ponto, _ e -"
    }

    $checked
}


export def relative-if-inside [path: string root: string] {
    try {
        $path | path relative-to $root
    } catch {
        null
    }
}
