package commands

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
	"github.com/josegonzalez/cli-skeleton/command"
	"github.com/posener/complete"
	flag "github.com/spf13/pflag"
)

const APIVERSION = "1.25"

type RelabelCommand struct {
	command.Meta

	addLabels    []string
	removeLabels []string
}

func (c *RelabelCommand) Name() string {
	return "relabel"
}

func (c *RelabelCommand) Synopsis() string {
	return "Re-labels a docker image"
}

func (c *RelabelCommand) Help() string {
	return command.CommandHelp(c)
}

func (c *RelabelCommand) Examples() map[string]string {
	appName := os.Getenv("CLI_APP_NAME")
	return map[string]string{
		"Add a label":            fmt.Sprintf("%s %s --label=label.key=label.value docker/image:latest", appName, c.Name()),
		"Remove a label":         fmt.Sprintf("%s %s --remove-label=label.key=label.value docker/image:latest", appName, c.Name()),
		"Add and remove a label": fmt.Sprintf("%s %s --label=new=value --remove-label=old=value docker/image:latest", appName, c.Name()),
	}
}

func (c *RelabelCommand) Arguments() []command.Argument {
	args := []command.Argument{}
	args = append(args, command.Argument{
		Name:        "image-name",
		Description: "name of image to manipulate",
		Optional:    false,
		Type:        command.ArgumentString,
	})
	return args
}

func (c *RelabelCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *RelabelCommand) ParsedArguments(args []string) (map[string]command.Argument, error) {
	return command.ParseArguments(args, c.Arguments())
}

func (c *RelabelCommand) FlagSet() *flag.FlagSet {
	f := c.Meta.FlagSet(c.Name(), command.FlagSetClient)
	f.StringSliceVar(&c.addLabels, "label", []string{}, "set of labels to add")
	f.StringSliceVar(&c.removeLabels, "remove-label", []string{}, "set of labels to remove")
	return f
}

func (c *RelabelCommand) AutocompleteFlags() complete.Flags {
	return command.MergeAutocompleteFlags(
		c.Meta.AutocompleteFlags(command.FlagSetClient),
		complete.Flags{
			"--label":        complete.PredictAnything,
			"--remove-label": complete.PredictAnything,
		},
	)
}

func (c *RelabelCommand) Run(args []string) int {
	flags := c.FlagSet()
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		c.Ui.Error(err.Error())
		c.Ui.Error(command.CommandErrorText(c))
		return 1
	}

	arguments, err := c.ParsedArguments(flags.Args())
	if err != nil {
		c.Ui.Error(err.Error())
		c.Ui.Error(command.CommandErrorText(c))
		return 1
	}

	imageName := arguments["image-name"].StringValue()

	if len(c.addLabels) == 0 && len(c.removeLabels) == 0 {
		c.Ui.Error("No labels specified\n")
		return 1
	}

	dockerClient, err := dockercli.NewClientWithOpts(dockercli.FromEnv, dockercli.WithVersion(APIVERSION))
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to create docker client: %s", err.Error()))
		return 1
	}

	if _, _, err = dockerClient.ImageInspectWithRaw(context.Background(), imageName); err != nil {
		if client.IsErrNotFound(err) {
			c.Ui.Error(fmt.Sprintf("Failed to fetch image id: %s", err.Error()))
			return 1
		}
	}

	img, err := local.NewImage(imageName, dockerClient, local.FromBaseImage(imageName))
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to create docker client: %s", err.Error()))
		return 1
	}

	originalImageID, err := img.Identifier()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to fetch image id: %s", err.Error()))
		return 1
	}

	inspect, _, err := dockerClient.ImageInspectWithRaw(context.Background(), originalImageID.String())
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to inspect the source image: %s", err.Error()))
		return 1
	}

	repoTags := inspect.RepoTags

	appendLabels, err := parseNewLabels(c.addLabels)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse new labels: %s", err.Error()))
		return 1
	}

	alternateTagsLabel := "com.dokku.docker-image-labeler/alternate-tags"
	alternateTagValue, err := fetchTags(inspect, alternateTagsLabel)
	if len(alternateTagValue) > 0 {
		appendLabels[alternateTagsLabel] = alternateTagValue
	}

	removed, err := removeImageLabels(img, c.removeLabels)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed removing labels: %s", err.Error()))
		return 1
	}

	added, err := addImageLabels(img, appendLabels)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed removing labels: %s", err.Error()))
		return 1
	}

	if !removed && !added {
		return 0
	}

	if err := img.Save(); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to save image: %s", err.Error()))
		return 1
	}

	newImageID, err := img.Identifier()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to fetch image id: %s", err.Error()))
		return 1
	}

	if newImageID == originalImageID {
		c.Ui.Error(fmt.Sprintf("New and old image have the same identifier: %s", newImageID))
		return 1
	}

	if len(repoTags) > 1 {
		return 0
	}

	if len(repoTags) == 1 && repoTags[0] != imageName {
		return 0
	}

	options := types.ImageRemoveOptions{
		Force:         false,
		PruneChildren: false,
	}

	if _, err := dockerClient.ImageRemove(context.Background(), originalImageID.String(), options); err != nil {
		if _, ok := err.(errdefs.ErrConflict); ok {
			c.Ui.Error(fmt.Sprintf("Warning: Failed to delete old image: %s", err.Error()))
			return 0
		}
		c.Ui.Error(fmt.Sprintf("Failed to delete old image: %s", err.Error()))
		return 1
	}

	return 0
}

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
