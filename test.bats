#!/usr/bin/env bats

export SYSTEM_NAME="$(uname -s | tr '[:upper:]' '[:lower:]')"
export BIN_FILE="build/$SYSTEM_NAME/docker-image-labeler"

setup() {
  make prebuild $BIN_FILE
}

teardown() {
  true
}

@test "version" {
  run $BIN_FILE version
  [[ "$status" -eq 0 ]]

  run $BIN_FILE -v
  [[ "$status" -eq 0 ]]

  run $BIN_FILE --version
  [[ "$status" -eq 0 ]]
}
