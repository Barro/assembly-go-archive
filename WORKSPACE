workspace(name = "assembly_archive")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

http_archive(
    name = "io_bazel_rules_go",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/rules_go/releases/download/v0.20.1/rules_go-v0.20.1.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.20.1/rules_go-v0.20.1.tar.gz",
    ],
    sha256 = "842ec0e6b4fbfdd3de6150b61af92901eeb73681fd4d185746644c338f51d4c0",
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

load("//static:static.bzl", "find_zopflipng")

find_zopflipng(name = "zopflipng")
