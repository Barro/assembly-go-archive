workspace(name = "assembly_archive")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "f5127a8f911468cd0b2d7a141f17253db81177523e4429796e14d429f5444f5f",
    urls = ["https://github.com/bazelbuild/rules_go/releases/download/0.16.1/rules_go-0.16.1.tar.gz"],
)

load("@io_bazel_rules_go//go:def.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains()

http_file(
    name = "yuicompressor",
    sha256 = "30371db57285e490c761f1cca52527e697fe09077a16da46fb892e98a6a25de2",
    urls = [
        "https://github.com/yui/yuicompressor/releases/download/v2.4.8/yuicompressor-2.4.8.jar",
    ],
)
