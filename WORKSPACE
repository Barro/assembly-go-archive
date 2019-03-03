workspace(name = "assembly_archive")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "6776d68ebb897625dead17ae510eac3d5f6342367327875210df44dbe2aeeb19",
    url = "https://github.com/bazelbuild/rules_go/releases/download/0.17.1/rules_go-0.17.1.tar.gz",
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
