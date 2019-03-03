#!/bin/bash

set -xeuo pipefail

BAZEL_VERSION=0.23.0
BAZEL_ARCHIVE=bazel_"$BAZEL_VERSION"-linux-x86_64.deb

curl --retry -o "$BAZEL_ARCHIVE" \
     https://github.com/bazelbuild/bazel/releases/download/"$BAZEL_VERSION"/"$BAZEL_ARCHIVE"

sudo dpkg -i "$BAZEL_ARCHIVE"
