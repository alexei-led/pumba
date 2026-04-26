package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	imagetypes "github.com/docker/docker/api/types/image"
	log "github.com/sirupsen/logrus"
)

// imagePullResponse is one line of the JSON progress stream returned by
// the Docker daemon's image pull endpoint.
type imagePullResponse struct {
	Status         string `json:"status"`
	Error          string `json:"error"`
	Progress       string `json:"progress"`
	ProgressDetail struct {
		Current int `json:"current"`
		Total   int `json:"total"`
	} `json:"progressDetail"`
}

// pullImage pulls a Docker image and drains the progress stream.
func (client dockerClient) pullImage(ctx context.Context, img string) error {
	log.WithField("img", img).Debug("pulling image")
	events, err := client.imageAPI.ImagePull(ctx, img, imagetypes.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull stress-ng img: %w", err)
	}
	defer events.Close()
	d := json.NewDecoder(events)
	var pullResponse *imagePullResponse
	for {
		if err = d.Decode(&pullResponse); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("failed to decode docker pull result: %w", err)
		}
		log.Debug(pullResponse)
	}
}
