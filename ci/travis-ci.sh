#!/bin/bash

set -xeuo pipefail

DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")"; pwd)

bazel build ...:all
