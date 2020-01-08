#!/usr/bin/env bash

set -xeuo pipefail

GITLAB_RELEASES_KEY_PRIVATE_FILE=$1

add-apt-repository -y "deb http://de.archive.ubuntu.com/ubuntu bionic main universe"

apt-get update
apt-get install -y -t bionic zopfli

bazel version
# Enable more platforms if the need arises to distribute them::
TARGET_PLATFORMS=(
    # darwin_amd64
    linux_amd64
    # linux_arm
    # windows_amd64
)
for platform in "${TARGET_PLATFORMS[@]}"; do
    bazel build ...:all --platforms=@io_bazel_rules_go//go/toolchain:"$platform"
    cp bazel-bin/assembly-archive-pkg.tar assembly-archive-"$platforms".tar

    if [[ -f $GITLAB_RELEASES_KEY_PRIVATE_FILE ]]; then
        bazel run :sign-pkg "$GITLAB_RELEASES_KEY_PRIVATE_FILE"
        cp bazel-bin/assembly-archive-pkg.tar.sig assembly-archive-"$platforms".tar.sig
    else
        echo "NO-SIGNATURE-IN-UNPROTECTED-BRANCH" > assembly-archive-"$platform".tar.sig
    fi
done
