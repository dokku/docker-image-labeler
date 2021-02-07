package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/buildpacks/imgutil/local"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	dockercli "github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	flag "github.com/spf13/pflag"
)

const APIVERSION = "1.25"

// Version contains the docker-image-labeler version
var Version string

func removeImageLabels(img *local.Image, labels []string) (bool, error) {
	modified := false
	for _, label := range labels {
		existingValue, err := img.Label(label)
		if err != nil {
			return modified, fmt.Errorf("Error fetching label %s (%s)\n", label, err.Error())
		}

		if existingValue == "" {
			continue
		}

		modified = true
		if err := img.RemoveLabel(label); err != nil {
			return modified, fmt.Errorf("Error removing label %s (%s)\n", label, err.Error())
		}
	}

	return modified, nil
}

func addImageLabels(img *local.Image, labels map[string]string) (bool, error) {
	modified := false
	for key, value := range labels {
		existingValue, err := img.Label(key)
		if err != nil {
			return modified, fmt.Errorf("Error fetching label %s (%s)\n", key, err.Error())
		}

		if existingValue == value {
			continue
		}

		modified = true
		if err := img.SetLabel(key, value); err != nil {
			return modified, fmt.Errorf("Error setting label %s=%s (%s)\n", key, value, err.Error())
		}
	}

	return modified, nil
}

func parseNewLabels(labels []string) (map[string]string, error) {
	m := map[string]string{}
	for _, label := range labels {
		parts := strings.SplitN(label, "=", 2)
		key := parts[0]
		value := ""
		if len(parts) == 2 {
			value = parts[1]
		}

		if len(key) == 0 {
			return m, errors.New("Invalid label specified")
		}

		m[key] = value
	}

	return m, nil
}

func fetchTags(inspect types.ImageInspect, label string) (string, error) {
	if len(inspect.RepoTags) == 0 {
		return "", nil
	}

	tags := inspect.RepoTags
	s, ok := inspect.Config.Labels[label]
	if ok {
		var existingTags []string
		if err := json.Unmarshal([]byte(s), &existingTags); err != nil {
			return "", err
		}

		tags = append(tags, existingTags...)
	}

	tags = unique(tags)
	originalRepoTags, err := json.Marshal(tags)
	if err != nil {
		return "", fmt.Errorf("Failed to encode image's original tags as JSON (%s)\n", err.Error())
	}
	return string(originalRepoTags), nil
}

func unique(intSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range intSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
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

	if _, _, err = dockerClient.ImageInspectWithRaw(context.Background(), imageName); err != nil {
		if client.IsErrNotFound(err) {
			fmt.Fprintf(os.Stderr, "Failed to fetch image id (%s)\n", err.Error())
			os.Exit(1)
		}
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

	repoTags := inspect.RepoTags

	appendLabels, err := parseNewLabels(*labels)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse new labels: %s\n", err.Error())
		os.Exit(1)
	}

	alternateTagsLabel := "com.dokku.docker-image-labeler/alternate-tags"
	alternateTagValue, err := fetchTags(inspect, alternateTagsLabel)
	if len(alternateTagValue) > 0 {
		appendLabels[alternateTagsLabel] = alternateTagValue
	}

	removed, err := removeImageLabels(img, *removeLabels)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed removing labels: %s\n", err.Error())
		os.Exit(1)
	}

	added, err := addImageLabels(img, appendLabels)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed removing labels: %s\n", err.Error())
		os.Exit(1)
	}

	if !removed && !added {
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
		fmt.Fprintf(os.Stderr, "New and old image have the same identifier (%s)\n", newImageID)
		os.Exit(1)
	}

	if len(repoTags) > 1 {
		return
	}

	if len(repoTags) == 1 && repoTags[0] != imageName {
		return
	}

	options := types.ImageRemoveOptions{
		Force:         false,
		PruneChildren: false,
	}

	if _, err := dockerClient.ImageRemove(context.Background(), originalImageID.String(), options); err != nil {
		if _, ok := err.(errdefs.ErrConflict); ok {
			fmt.Fprintf(os.Stderr, "Warning: Failed to delete old image (%s)\n", err.Error())
			return
		}
		fmt.Fprintf(os.Stderr, "Failed to delete old image (%s)\n", err.Error())
		os.Exit(1)
	}
}
