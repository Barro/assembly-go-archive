#!/usr/bin/env bash
#
# Fetches latest files from Gitlab, verifies them, and replaces
# existing installation under bin/ directory under given ROOT for a
# specific platform.
#
# There needs to be assembly-archive.pub.pem key file to verify
# package signatures.

set -euo pipefail

ROOT=$1
PLATFORM=$2

cd "$ROOT"

GITLAB_BASE=https://gitlab.com/Barro/assembly-archive
ARTIFACTS_URL=$GITLAB_BASE/-/jobs/artifacts/master/download?job=build
SIG_URL=$GITLAB_BASE/-/jobs/artifacts/master/raw/assembly-archive-"$PLATFORM".tar.sig?job=build

# Fetch a new signature. If this fails, there is nothing to download as Gitlab artifacts have expired.
if ! curl --fail-early --fail --silent -o assembly-archive-"$PLATFORM".tar.sig.new --location "$SIG_URL"; then
    #echo >&2 "No signature artifact $SIG_URL exist."
    exit 0
fi

# Nothing to update:
if cmp --quiet assembly-archive-"$PLATFORM".tar.sig.new assembly-archive-"$PLATFORM".tar.sig; then
    #echo >&2 "Nothing to update."
    exit 0
fi
if ! curl --fail-early --fail --silent -o assembly-archive.zip --location "$ARTIFACTS_URL"; then
    echo >&2 "Package download from $ARTIFACTS_URL failed!"
    exit 1
fi
rm -rf extracted
mkdir -p extracted
unzip -qq assembly-archive.zip -d extracted

openssl dgst -verify assembly-archive.pub.pem \
        -signature extracted/assembly-archive-"$PLATFORM".tar.sig \
        extracted/assembly-archive-"$PLATFORM".tar \
    &> /dev/null

rm -rf new
mkdir -p new
tar xf extracted/assembly-archive-"$PLATFORM".tar -C new/
mv bin bin-remove
mv new bin
rm -rf bin-remove

if [[ -f assembly-archive-"$PLATFORM".tar ]]; then
    cp -f assembly-archive-"$PLATFORM".tar assembly-archive-"$PLATFORM".tar.old
fi
cp -f extracted/assembly-archive-"$PLATFORM".tar assembly-archive-"$PLATFORM".tar
# Replace the previous signature on successful replacement:
mv extracted/assembly-archive-"$PLATFORM".tar.sig assembly-archive-"$PLATFORM".tar.sig
