# docker-image-labeler [![CircleCI](https://circleci.com/gh/dokku/docker-image-labeler.svg?style=svg)](https://circleci.com/gh/dokku/docker-image-labeler)

Adds and removes labels from docker images

## Requirements

- golang 1.12+

## Background

This package allows for adding and removing image labels without rebuilding images. Layer history creation time may change.

## Installation

Debian and RPM packages are available via [packagecloud](https://packagecloud.io/dokku/dokku)

For a prebuilt binaries, see the [github releases page](https://github.com/dokku/docker-image-labeler/releases).

## Building from source

A make target is provided for building the package from source.

```shell
make build
```

In addition, builds can be performed in an isolated Docker container:

```shell
make build-docker-image build-in-docker
```

## Usage

```shell
# pull an image
docker image pull mysql:8

# add a label
./docker-image-labeler relabel --label=mysql.version=8 mysql:8

# remove the label
./docker-image-labeler relabel --remove-label=mysql.version mysql:8
```