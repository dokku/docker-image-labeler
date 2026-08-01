package commands

import (
	"reflect"
	"testing"

	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/moby/moby/api/types/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestParseNewLabels(t *testing.T) {
	tests := []struct {
		name    string
		labels  []string
		want    map[string]string
		wantErr bool
	}{
		{
			name:   "empty input",
			labels: []string{},
			want:   map[string]string{},
		},
		{
			name:   "key and value",
			labels: []string{"key=value"},
			want:   map[string]string{"key": "value"},
		},
		{
			name:   "key only",
			labels: []string{"key"},
			want:   map[string]string{"key": ""},
		},
		{
			name:   "value contains equals sign",
			labels: []string{"key=a=b"},
			want:   map[string]string{"key": "a=b"},
		},
		{
			name:   "multiple labels",
			labels: []string{"a=1", "b=2"},
			want:   map[string]string{"a": "1", "b": "2"},
		},
		{
			name:    "empty key",
			labels:  []string{"=value"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseNewLabels(tt.labels)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseNewLabels(%v) expected error, got nil", tt.labels)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseNewLabels(%v) unexpected error: %v", tt.labels, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseNewLabels(%v) = %v, want %v", tt.labels, got, tt.want)
			}
		})
	}
}

func TestUnique(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "empty",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "single element",
			input: []string{"x"},
			want:  []string{"x"},
		},
		{
			name:  "removes duplicates preserving order",
			input: []string{"a", "b", "a", "c", "b"},
			want:  []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unique(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unique(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// inspectWithLabels builds an image.InspectResponse fixture using the moby/moby
// api types that replaced github.com/docker/docker/api/types.
func inspectWithLabels(repoTags []string, labels map[string]string) image.InspectResponse {
	inspect := image.InspectResponse{RepoTags: repoTags}
	if labels != nil {
		inspect.Config = &dockerspec.DockerOCIImageConfig{
			ImageConfig: ocispec.ImageConfig{Labels: labels},
		}
	}
	return inspect
}

const alternateTagsLabel = "com.dokku.docker-image-labeler/alternate-tags"

func TestFetchTags(t *testing.T) {
	tests := []struct {
		name    string
		inspect image.InspectResponse
		want    string
		wantErr bool
	}{
		{
			name:    "no repo tags returns empty",
			inspect: inspectWithLabels(nil, nil),
			want:    "",
		},
		{
			name:    "no config uses repo tags",
			inspect: inspectWithLabels([]string{"hello-world:latest"}, nil),
			want:    `["hello-world:latest"]`,
		},
		{
			name:    "config without label uses repo tags",
			inspect: inspectWithLabels([]string{"hello-world:latest"}, map[string]string{"other": "value"}),
			want:    `["hello-world:latest"]`,
		},
		{
			name:    "merges existing alternate tags",
			inspect: inspectWithLabels([]string{"hello-world:latest"}, map[string]string{alternateTagsLabel: `["old:tag"]`}),
			want:    `["hello-world:latest","old:tag"]`,
		},
		{
			name:    "deduplicates repo and alternate tags",
			inspect: inspectWithLabels([]string{"hello-world:latest"}, map[string]string{alternateTagsLabel: `["hello-world:latest","old:tag"]`}),
			want:    `["hello-world:latest","old:tag"]`,
		},
		{
			name:    "invalid alternate tags json returns error",
			inspect: inspectWithLabels([]string{"hello-world:latest"}, map[string]string{alternateTagsLabel: "not-json"}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fetchTags(tt.inspect, alternateTagsLabel)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("fetchTags() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("fetchTags() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("fetchTags() = %q, want %q", got, tt.want)
			}
		})
	}
}
