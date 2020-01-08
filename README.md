This is the second iteration of [Assembly Archive](https://archive.assembly.org/).

## Pre-requisites

* Likely a modern GNU/Linux system to handle all the build time bash magic.
* [Bazel](https://bazel.build/) and its dependencies, like JRE.
* [zopflipng](https://github.com/google/zopfli)

Also a recommended requirement is to use
[iBazel](https://github.com/bazelbuild/bazel-watcher) to get immediate
updates into use when source code changes.

## Building

To create a build for distribution, you can use `bazel build ...:all`
command to build everything. This will then create a distribution
tarball at `bazel-bin/assembly-archive.tar.gz`. This also supports
cross compilation against different platforms, so you can create
binaries that work on different systems without actually having to
compile on such systems:

```bash
# Compile for the current host platform:
$ bazel build //:assembly-archive-pkg
# Compile for Raspberry Pi:
$ bazel build //:assembly-archive-pkg --platforms=@io_bazel_rules_go//go/toolchain:linux_arm
# Compile for macOS:
$ bazel build //:assembly-archive-pkg --platforms=@io_bazel_rules_go//go/toolchain:darwin_amd64
# Compile for Linux:
$ bazel build //:assembly-archive-pkg --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64
# Compile for Windows:
$ bazel build //:assembly-archive-pkg --platforms=@io_bazel_rules_go//go/toolchain:windows_amd64
```

### Creating SystemD unit

SystemD unit file for Assembly Archive can be created by building
`//service:assembly-archive-service` target with appropriate defines:

```bash
bazel build //service:assembly-archive-service \
    --define ASMARCHIVE_USER=asmarchive \
    --define ASMARCHIVE_GROUP=asmarchive \
    --define ASMARCHIVE_ENVIRONMENT_FILE=/assembly-archive/assembly-archive.env \
    --define ASMARCHIVE_BIN_DIR=/assembly-archive/bin
```

This will then create `bazel-bin/service/assembly-archive-service.tar`
that holds following SystemD units and a launcher binary that should
be extracted under `/usr/local`:

* `assembly-archive.service` launches the server executable.
* `assembly-archive-watcher.service` re-launches the service. Used for
  service updates.
* `assembly-archive-watcher.path` monitors paths under
  `ASMARCHIVE_BIN_DIR` for updates and restarts the service when such
  things is detected.

## Running

Assembly archive expects a reverse proxy that only exposes the
`/site/` namespace to the public when in production mode as the root
path.

The extracted distribution package provides `assembly-archive`
executable that includes all the application logic. This listens to
`localhost` at port `8080` and you may want to change that for
production:

```bash
$ ./assembly-archive -host 0.0.0.0 -port 1234
2019/07/20 12:52:54 Listening to 0.0.0.0:1234
```

Also to enable `/api/` usage, you need to create a plain text file
that defines the API credentials for updates. By default this reads
`auth.txt` but it can be configured with `-authfile` parameter. The
format of this file is to have `USERNAME:PASSWORD` combination on each
line. This format intentionally does not hash passwords. Here are
example commands to create `/api/` namespace access::

```bash
$ touch auth.txt
$ chmod 600 auth.txt
$ echo username:password > auth.txt
$ ./assembly-archive -authfile auth.txt
```

## Development

In development you can run locally in `-dev` mode. This basically
entails the standard hack, build, test cycle where the build and test
phases can be done by using Bazel. It is recommended to use
[iBazel](https://github.com/bazelbuild/bazel-watcher) to get make
rebuild, restart, and retest operations done automatically:

Run the development server:

```bash
# You can run the latest version with Bazel. You need to manually
# press ctrl-c to stop the process and start it again:
$ bazel run //:devserver -- -dev
# ibazel command does restarts for you whenever something changes:
$ ibazel run //:devserver -- -dev
```

Run tests:

```bash
$ bazel test ...:all
# ibazel will re-run tests immediately when something changes:
$ ibazel test ...:all
```

## Deploying

The
[GitLab instance of this repository](https://gitlab.com/Barro/assembly-archive)
provides pre-built binaries that have a relatively short
lifetime. When a newer version of Assembly Archive binaries is
published, it can be verified with following type of command:

```bash
openssl dgst \
    -verify assembly-archive.pub.pem \
    -signature assembly-archive-linux_amd64.tar.sig \
    assembly-archive-linux_amd64.tar
```

The contents of `assembly-archive.pub.pem` file currently is:

```
-----BEGIN PUBLIC KEY-----
MFYwEAYHKoZIzj0CAQYFK4EEAAoDQgAEOd3EAKmlJPzppm3WTSlr6RjhSHrxishA
PGXuelBjmnIOOKOtL+REtkc1o4btrvVvgtHY+xV75hqi+CEYAQoBsw==
-----END PUBLIC KEY-----
```

This signature verification and replacing old binaries with a newer
ones is done automatically with
[`ci/update-binaries.sh`](ci/update-binaries.sh) script.
