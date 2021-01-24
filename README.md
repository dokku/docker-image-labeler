# docker-image-labeler [![CircleCI](https://circleci.com/gh/dokku/docker-image-labeler.svg?style=svg)](https://circleci.com/gh/dokku/docker-image-labeler)

Adds and removes labels from docker images

## Requirements

- golang 1.12+

## Background

This package allows for adding and removing image labels without rebuilding images.

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
