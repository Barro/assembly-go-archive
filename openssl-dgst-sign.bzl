def _openssl_dgst_sign_impl(ctx):
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = ctx.outputs.executable,
        substitutions = {
            "{{SRC}}": ctx.file.src.short_path,
        },
        is_executable = True,
    )

    return [DefaultInfo(
        runfiles = ctx.runfiles([ctx.file.src]),
    )]

openssl_dgst_sign = rule(
    implementation = _openssl_dgst_sign_impl,
    attrs = {
        "src": attr.label(allow_single_file = True),
        "_template": attr.label(
            allow_single_file = True,
            default = Label("//:openssl-dgst-sign.sh.tmpl"),
        ),
    },
    executable = True,
)
