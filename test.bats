#!/usr/bin/env bats

export SYSTEM_NAME="$(uname -s | tr '[:upper:]' '[:lower:]')"
export BIN_FILE="build/$SYSTEM_NAME/docker-image-labeler"

setup() {
  make prebuild $BIN_FILE
  docker rmi hello-world:latest 2>/dev/null || true
  docker rmi alpine:latest 2>/dev/null || true
  docker rmi alpine/git:v2.30.0 2>/dev/null || true
}

teardown() {
  true
}

@test "version" {
  run $BIN_FILE version
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]

  run $BIN_FILE -v
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]

  run $BIN_FILE --version
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
}

@test "failing arguments" {
  run $BIN_FILE
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 1 ]]
  [[ "$output" == "No image specified" ]]

  run $BIN_FILE hello-world
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 1 ]]
  [[ "$output" == "No labels specified" ]]

  run $BIN_FILE hello-world --label
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 2 ]]
  [[ "$output" == *"flag needs an argument: --label"* ]]

  run $BIN_FILE hello-world --remove-label
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 2 ]]
  [[ "$output" == *"flag needs an argument: --remove-label"* ]]

  run $BIN_FILE hello-world --label key=value
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 1 ]]
  [[ "$output" == "Failed to fetch image id (Error: No such image: hello-world)" ]]

  docker image pull hello-world:latest
  run $BIN_FILE hello-world --label =value
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 1 ]]
  [[ "$output" == "Failed to parse new labels: Invalid label specified" ]]
}

@test "layer length does not change" {
  docker image pull hello-world:latest
  run /bin/bash -c "docker image inspect hello-world:latest --format '{{ .RootFS.Layers }}' | grep -o sha256 | wc -l"
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" -eq 1 ]]

  run $BIN_FILE hello-world:latest --label key=value
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]

  run /bin/bash -c "docker image inspect hello-world:latest --format '{{ .RootFS.Layers }}' | grep -o sha256 | wc -l"
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" -eq 1 ]]

  docker image pull alpine/git:v2.30.0
  run /bin/bash -c "docker image inspect alpine/git:v2.30.0 --format '{{ .RootFS.Layers }}' | grep -o sha256 | wc -l"
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" -eq 3 ]]

  run $BIN_FILE alpine/git:v2.30.0 --label key=value
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]

  run /bin/bash -c "docker image inspect alpine/git:v2.30.0 --format '{{ .RootFS.Layers }}' | grep -o sha256 | wc -l"
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" -eq 3 ]]

  run $BIN_FILE alpine/git:v2.30.0 --label key2=value2
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]

  run /bin/bash -c "docker image inspect alpine/git:v2.30.0 --format '{{ .RootFS.Layers }}' | grep -o sha256 | wc -l"
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" -eq 3 ]]
}

@test "retag with same label" {
  docker image pull hello-world:latest
  run $BIN_FILE hello-world:latest --label key=value
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]

  run docker image inspect hello-world:latest --format '{{ index .Config.Labels "key" }}'
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" == "value" ]]

  run docker image inspect hello-world:latest --format '{{ index .Config.Labels "com.dokku.docker-image-labeler/alternate-tags" }}'
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" == '["hello-world:latest"]' ]]

  run docker image inspect hello-world:latest --format '{{ .Metadata.LastTagTime }}'
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]

  # retag
  last_tag_time="$output"
  run $BIN_FILE hello-world:latest --label key=value
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]

  run docker image inspect hello-world:latest --format '{{ index .Config.Labels "key" }}'
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" == "value" ]]

  run docker image inspect hello-world:latest --format '{{ index .Config.Labels "com.dokku.docker-image-labeler/alternate-tags" }}'
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" == '["hello-world:latest"]' ]]

  run docker image inspect hello-world:latest --format '{{ .Metadata.LastTagTime }}'
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" == "$last_tag_time" ]]
}


@test "retag with new labels" {
  docker image pull hello-world:latest
  run $BIN_FILE hello-world:latest --label key=value
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]

  run docker image inspect hello-world:latest --format '{{ index .Config.Labels "key" }}'
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" == "value" ]]

  run docker image inspect hello-world:latest --format '{{ index .Config.Labels "com.dokku.docker-image-labeler/alternate-tags" }}'
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" == '["hello-world:latest"]' ]]

  run docker image inspect hello-world:latest --format '{{ .Metadata.LastTagTime }}'
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]

  # retag
  last_tag_time="$output"
  run $BIN_FILE hello-world:latest --label key=value --label key2=value2
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]

  run docker image inspect hello-world:latest --format '{{ index .Config.Labels "key" }}'
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" == "value" ]]

  run docker image inspect hello-world:latest --format '{{ index .Config.Labels "key2" }}'
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" == "value2" ]]

  run docker image inspect hello-world:latest --format '{{ index .Config.Labels "com.dokku.docker-image-labeler/alternate-tags" }}'
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" == '["hello-world:latest"]' ]]

  run docker image inspect hello-world:latest --format '{{ .Metadata.LastTagTime }}'
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" != "$last_tag_time" ]]
}

@test "does not delete images with running containers" {
  docker image pull hello-world:latest
  run /bin/bash -c "docker image inspect hello-world:latest --format '{{ .ID }}' | cut -d ':' -f2"
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]

  originalImageID="$output"
  run docker container run hello-world:latest
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]

  run $BIN_FILE hello-world:latest --label key=value
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]

  run /bin/bash -c "docker image inspect hello-world:latest --format '{{ .ID }}' | cut -d ':' -f2"
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" != "$originalImageID" ]]

  run /bin/bash -c "docker image inspect $originalImageID"
  echo "status: $status"
  echo "output: $output"
  [[ "$status" -eq 0 ]]
}
