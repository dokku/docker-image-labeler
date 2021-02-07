package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/buildpacks/imgutil/local"
	"github.com/docker/docker/api/types"
	dockercli "github.com/docker/docker/client"
	flag "github.com/spf13/pflag"
)

const APIVERSION = "1.25"

// Version contains the docker-image-labeler version
var Version string

func removeImageLabels(img *local.Image, labels []string) bool {
	modified := false
	for _, label := range labels {
		existingValue, err := img.Label(label)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching label %s (%s)\n", label, err.Error())
			os.Exit(1)
		}

		if existingValue == "" {
			continue
		}

		modified = true
		if err := img.RemoveLabel(label); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing label %s (%s)\n", label, err.Error())
			os.Exit(1)
		}
	}

	return modified
}

func main() {
	args := flag.NewFlagSet("docker-image-labeler", flag.ExitOnError)
	var labels *[]string = args.StringSlice("label", []string{}, "set of labels to add")
	var removeLabels *[]string = args.StringSlice("remove-label", []string{}, "set of labels to remove")
	var versionFlag *bool = args.BoolP("version", "v", false, "show the version")
	args.Parse(os.Args[1:])
	imageName := args.Arg(0)

	if *versionFlag {
		fmt.Fprintf(os.Stdout, "docker-image-labeler %s\n", Version)
		os.Exit(0)
	}

	if imageName == "version" {
		fmt.Fprintf(os.Stdout, "docker-image-labeler %s\n", Version)
		os.Exit(0)
	}

	if imageName == "" {
		fmt.Fprintf(os.Stderr, "No image specified\n")
		os.Exit(1)
	}

	if len(*labels) == 0 && len(*removeLabels) == 0 {
		fmt.Fprintf(os.Stderr, "No labels specified\n")
		os.Exit(1)
	}

	dockerClient, err := dockercli.NewClientWithOpts(dockercli.FromEnv, dockercli.WithVersion(APIVERSION))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create docker client (%s)\n", err.Error())
		os.Exit(1)
	}

	img, err := local.NewImage(imageName, dockerClient, local.FromBaseImage(imageName))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create docker client (%s)\n", err.Error())
		os.Exit(1)
	}

	originalImageID, err := img.Identifier()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch image id (%s)\n", err.Error())
		os.Exit(1)
	}

	inspect, _, err := dockerClient.ImageInspectWithRaw(context.Background(), originalImageID.String())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to inspect the source image (%s)\n", err.Error())
		os.Exit(1)
	}

	appendLabels := *labels
	originalTagsLabel := "com.dokku.docker-image-labeler/original-tags"
	if _, ok := inspect.Config.Labels[originalTagsLabel]; !ok {
		originalRepoTags, err := json.Marshal(inspect.RepoTags)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to encode image's original tags as JSON (%s)\n", err.Error())
			os.Exit(1)
		}

		appendLabels = append(appendLabels, fmt.Sprintf("com.dokku.docker-image-labeler/original-tags=%s", string(originalRepoTags)))
	}

	modified := removeImageLabels(img, *removeLabels)

	for _, label := range appendLabels {
		parts := strings.SplitN(label, "=", 2)
		key := parts[0]
		newValue := ""
		if len(parts) == 2 {
			newValue = parts[1]
		}

		existingValue, err := img.Label(key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching label %s (%s)\n", key, err.Error())
			os.Exit(1)
		}

		if existingValue == newValue {
			continue
		}

		modified = true
		if err := img.SetLabel(key, newValue); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting label %s=%s (%s)\n", key, newValue, err.Error())
			os.Exit(1)
		}
	}

	if !modified {
		return
	}

	if err := img.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save image (%s)\n", err.Error())
		os.Exit(1)
	}

	newImageID, err := img.Identifier()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch image id (%s)\n", err.Error())
		os.Exit(1)
	}

	if newImageID == originalImageID {
		os.Exit(1)
	}

	if len(inspect.RepoTags) > 0 {
		return
	}

	options := types.ImageRemoveOptions{
		Force:         false,
		PruneChildren: false,
	}

	if _, err := dockerClient.ImageRemove(context.Background(), originalImageID.String(), options); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to delete old image (%s)\n", err.Error())
		os.Exit(1)
	}
}
