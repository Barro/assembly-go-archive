def _yui_minified_impl(ctx):
    tools = ctx.attr._yuicompressor.files
    minify_args = ctx.actions.args()
    minify_args.add("-jar")
    minify_args.add_all(ctx.attr._yuicompressor.files)
    minify_args.add("-o")
    outfiles = depset()
    for infile in ctx.files.srcs:
        outfile = ctx.actions.declare_file(
            "_min/%s" % infile.short_path)
        ctx.actions.run(
            outputs = [outfile],
            inputs = [infile],
            tools = tools,
            executable = "java",
            arguments = [minify_args, outfile.path, infile.path])
        outfiles += [outfile]

    outfiles_args = ctx.actions.args()
    outfiles_args.add_all(outfiles)
    output = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.run_shell(
        outputs = [output],
        inputs = outfiles,
        arguments = [output.path, outfiles_args],
        command = "out=$1; shift; cat \"$@\" > \"$out\"",
    )
    return [DefaultInfo(files=depset([output]))]


yui_minified = rule(
    implementation = _yui_minified_impl,
    attrs = {
        "srcs": attr.label_list(
            allow_files = True,
        ),
        "_yuicompressor": attr.label(
            default = Label("@yuicompressor//file")
        ),
    },
)

def _zopflipng_minified_impl(ctx):
    outfiles = depset()
    for infile in ctx.files.srcs:
        outfile = ctx.actions.declare_file(
            "%s/%s" % (ctx.label.name, infile.short_path))
        ctx.actions.run(
            outputs = [outfile],
            inputs = [infile],
            executable = "zopflipng",
            arguments = ["-m", "-y", infile.path, outfile.path])
        outfiles += [outfile]
    return [DefaultInfo(files=outfiles)]


zopflipng_minified = rule(
    implementation = _zopflipng_minified_impl,
    attrs = {
        "srcs": attr.label_list(
            allow_files = True
        ),
    },
)
