load("@bazel_tools//tools/build_defs/pkg:pkg.bzl", "pkg_tar")

def _asmarchive_service_create_impl(ctx):
    # Technically these could be determined from the package
    # archive. But thanks to pkg_tar() not supporting transitive
    # dependencies, it this information would be somewhat cumbersome
    # to get out.
    bin_paths = [
        "",
        "static",
        "static/images",
        "templates",
    ]

    substitutions_raw = dict(
        ASMARCHIVE_BIN_DIR = ctx.var.get(
            "ASMARCHIVE_BIN_DIR",
            "/usr/local/share/assembly-archive",
        ),
        ASMARCHIVE_LAUNCHER = ctx.var.get(
            "ASMARCHIVE_LAUNCHER",
            "/usr/local/bin/assembly-archive-launcher",
        ),
        ASMARCHIVE_ENVIRONMENT_FILE = ctx.var.get(
            "ASMARCHIVE_ENVIRONMENT_FILE",
            "/usr/local/etc/assembly-archive.env",
        ),
        ASMARCHIVE_USER = ctx.var.get("ASMARCHIVE_USER", "nobody"),
        ASMARCHIVE_GROUP = ctx.var.get("ASMARCHIVE_GROUP", "nogroup"),
    )
    path_changed_lines = []
    for path in bin_paths:
        path_changed_lines.append(
            "PathChanged=%s/%s" % (
                substitutions_raw["ASMARCHIVE_BIN_DIR"],
                path,
            ),
        )
    substitutions_raw["ASMARCHIVE_SYSTEMD_PATH_CHANGED_LINES"] = "\n".join(
        path_changed_lines,
    )
    substitutions = dict([
        ("{{%s}}" % key, value)
        for key, value in substitutions_raw.items()
    ])

    out_files = {
        "lib/systemd/system/assembly-archive.service": (ctx.file.unit_service, False),
        "lib/systemd/system/assembly-archive-watcher.service": (ctx.file.unit_watcher, False),
        "lib/systemd/system/assembly-archive-watcher.path": (ctx.file.unit_watcher_path, False),
        "bin/assembly-archive-launcher": (ctx.file.launcher, True),
    }
    outfiles = []
    for target, (source, is_executable) in out_files.items():
        outfile = ctx.actions.declare_file(target)
        ctx.actions.expand_template(
            template = source,
            output = outfile,
            substitutions = substitutions,
            is_executable = is_executable,
        )
        outfiles.append(outfile)
    return DefaultInfo(files = depset(outfiles))

_asmarchive_service_create = rule(
    implementation = _asmarchive_service_create_impl,
    attrs = {
        "unit_service": attr.label(
            allow_single_file = True,
            default = "//service:assembly-archive.service.tmpl",
        ),
        "unit_watcher": attr.label(
            allow_single_file = True,
            default = "//service:assembly-archive-watcher.service.tmpl",
        ),
        "unit_watcher_path": attr.label(
            allow_single_file = True,
            default = "//service:assembly-archive-watcher.path.tmpl",
        ),
        "launcher": attr.label(
            allow_single_file = True,
            default = "//service:assembly-archive-launcher",
        ),
    },
)

def asmarchive_service(name, **kw):
    files_target = "%s-files" % name
    _asmarchive_service_create(name = files_target, **kw)
    pkg_tar(
        name = name,
        strip_prefix = ".",
        srcs = [":%s" % files_target],
        **kw
    )
