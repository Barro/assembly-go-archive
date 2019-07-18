def _yui_minified_impl(ctx):
    tools = ctx.attr._yuicompressor.files
    minify_args = ctx.actions.args()
    minify_args.add("-jar")
    minify_args.add_all(ctx.attr._yuicompressor.files)
    minify_args.add("-o")
    outfiles = []
    for infile in ctx.files.srcs:
        outfile = ctx.actions.declare_file(
            "_min/%s" % infile.short_path,
        )
        ctx.actions.run(
            outputs = [outfile],
            inputs = [infile],
            tools = tools,
            executable = "java",
            arguments = [minify_args, outfile.path, infile.path],
        )
        outfiles.append(outfile)

    outfiles_args = ctx.actions.args()
    outfiles_args.add_all(outfiles)
    output = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.run_shell(
        outputs = [output],
        inputs = outfiles,
        arguments = [output.path, outfiles_args],
        command = "out=$1; shift; cat \"$@\" > \"$out\"",
    )
    return [DefaultInfo(files = depset([output]))]

yui_minified = rule(
    attrs = {
        "srcs": attr.label_list(
            allow_files = True,
        ),
        "_yuicompressor": attr.label(
            default = Label("@yuicompressor//file"),
        ),
    },
    implementation = _yui_minified_impl,
)

def _zopflipng_minified_impl(ctx):
    outfiles = []
    for infile in ctx.files.srcs:
        outfile = ctx.actions.declare_file(
            "%s/%s" % (ctx.label.name, infile.short_path),
        )
        ctx.actions.run(
            outputs = [outfile],
            inputs = [infile],
            executable = "zopflipng",
            arguments = ["-m", "-y", infile.path, outfile.path],
        )
        outfiles.append(outfile)
    return [DefaultInfo(files = depset(outfiles))]

zopflipng_minified = rule(
    attrs = {
        "srcs": attr.label_list(
            allow_files = True,
        ),
    },
    implementation = _zopflipng_minified_impl,
)
