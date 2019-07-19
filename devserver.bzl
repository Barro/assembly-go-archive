def _devserver_impl(ctx):
    templates_dir = ctx.actions.declare_directory(
        "%s.dir/templates" % ctx.label.name,
    )
    ctx.actions.run(
        outputs = [templates_dir],
        inputs = [ctx.files.templates[0]],
        executable = "tar",
        arguments = [
            "xf",
            ctx.files.templates[0].path,
            "-C",
            templates_dir.path,
            # Bazel's pkg_tar rule always adds ./ as the prefix. This
            # may hopefully break at some point of time and we get
            # less nasty tarballs.
            "--strip-components=2",
            "./templates/",
        ],
    )
    static_dir = ctx.actions.declare_directory(
        "%s.dir/static" % ctx.label.name,
    )
    ctx.actions.run(
        outputs = [static_dir],
        inputs = [ctx.file.static],
        executable = "tar",
        arguments = [
            "xf",
            ctx.file.static.path,
            "-C",
            static_dir.path,
            # Bazel's pkg_tar rule always adds ./ as the prefix. This
            # may hopefully break at some point of time and we get
            # less nasty tarballs.
            "--strip-components=2",
            "./static/",
        ],
    )
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = ctx.outputs.executable,
        substitutions = {
            "{{APP}}": ctx.file.app.short_path,
            "{{STATIC}}": static_dir.short_path,
            "{{TEMPLATES}}": templates_dir.short_path,
        },
        is_executable = True,
    )

    return [DefaultInfo(
        runfiles = ctx.runfiles([ctx.file.app, templates_dir, static_dir]),
    )]

devserver = rule(
    attrs = {
        "app": attr.label(allow_single_file = True),
        "static": attr.label(allow_single_file = True),
        "templates": attr.label(),
        "_template": attr.label(
            default = Label("//:devserver.sh.tmpl"),
            allow_single_file = True,
        ),
    },
    executable = True,
    implementation = _devserver_impl,
)
