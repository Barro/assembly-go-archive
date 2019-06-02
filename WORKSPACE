workspace(name = "assembly_archive")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "a82a352bffae6bee4e95f68a8d80a70e87f42c4741e6a448bec11998fcc82329",
    url = "https://github.com/bazelbuild/rules_go/releases/download/0.18.5/rules_go-0.18.5.tar.gz",
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains()

http_file(
    name = "yuicompressor",
    sha256 = "30371db57285e490c761f1cca52527e697fe09077a16da46fb892e98a6a25de2",
    urls = [
        "https://github.com/yui/yuicompressor/releases/download/v2.4.8/yuicompressor-2.4.8.jar",
    ],
)
