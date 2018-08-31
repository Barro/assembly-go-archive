load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
    name = "io_bazel_rules_go",
    urls = ["https://github.com/bazelbuild/rules_go/releases/download/0.15.1/rules_go-0.15.1.tar.gz"],
    sha256 = "5f3b0304cdf0c505ec9e5b3c4fc4a87b5ca21b13d8ecc780c97df3d1809b9ce6",
)
load("@io_bazel_rules_go//go:def.bzl", "go_rules_dependencies", "go_register_toolchains")
go_rules_dependencies()
go_register_toolchains()

http_file(
    name = "yuicompressor",
    urls = [
        "https://github.com/yui/yuicompressor/releases/download/v2.4.8/yuicompressor-2.4.8.jar"
    ],
    sha256 = "30371db57285e490c761f1cca52527e697fe09077a16da46fb892e98a6a25de2",
)
