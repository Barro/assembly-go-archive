#!/usr/bin/env bash

set -xeuo pipefail

(
    set -xeuo pipefail

    git clone https://github.com/bazelbuild/buildtools.git
    cd buildtools
    bazel build :buildifier
    sudo cp -p bazel-bin/buildifier /usr/local/bin/
)
