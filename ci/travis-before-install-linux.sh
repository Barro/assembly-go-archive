#!/bin/bash

set -xeuo pipefail

BAZEL_VERSION=0.23.0
BAZEL_ARCHIVE=bazel_"$BAZEL_VERSION"-linux-x86_64.deb
BAZEL_SHA256=6d3f5a2eae9021671a967c362513eaaae979b76e9c725359cb75ee149a92817e
echo "$BAZEL_SHA256 *$BAZEL_ARCHIVE" > checksums.sha256

curl --location --retry 5 -o "$BAZEL_ARCHIVE" \
     https://github.com/bazelbuild/bazel/releases/download/"$BAZEL_VERSION"/"$BAZEL_ARCHIVE"
sha256sum -c checksums.sha256
ls

sudo dpkg -i "$BAZEL_ARCHIVE"
