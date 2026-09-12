# Helpers internos compartilhados pelo módulo repo_zip.

export def fail [message: string] {
    error make { msg: $"repo-zip: ($message)" }
}

export def external-detail [result: record] {
    let stderr = ($result.stderr | default "" | str trim)
    let stdout = ($result.stdout | default "" | str trim)

    if ($stderr | is-not-empty) {
        $stderr
    } else {
        $stdout
    }
}
