#!/usr/bin/env bash

set -xeuo pipefail

GITLAB_RELEASES_KEY_PRIVATE_FILE=$1

add-apt-repository -y "deb http://de.archive.ubuntu.com/ubuntu bionic main universe"

apt-get update
apt-get install -y -t bionic zopfli

bazel version
bazel build ...:all
cp bazel-bin/assembly-archive-pkg.tar .

if [[ -f $GITLAB_RELEASES_KEY_PRIVATE_FILE ]]; then
    bazel run :sign-pkg "$GITLAB_RELEASES_KEY_PRIVATE_FILE"
    cp bazel-bin/assembly-archive-pkg.tar.sig .
else
    echo "NO-SIGNATURE-IN-UNPROTECTED-BRANCH" > assembly-archive-pkg.tar.sig
fi
