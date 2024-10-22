package example

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	deploy "cloud.google.com/go/deploy/apiv1"
	"cloud.google.com/go/deploy/apiv1/deploypb"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
)

func init() {
	functions.CloudEvent("cloudDeployInteractions", cloudDeployInteractions)
}

type PubSubMessage struct {
	Data []byte `json:"data"`
}

type MessagePublishedData struct {
	Message PubSubMessage
}

type DeployCommand struct {
	Commmand      string                        `json:"command"`
	CreateRelease deploypb.CreateReleaseRequest `json:"createReleaseRequest"`
}

func cloudDeployInteractions(ctx context.Context, e event.Event) error {
	log.Printf("Deploy trigger function invoked")
	// Parse the Pub/Sub message data
	var msg MessagePublishedData
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("event.DataAs: %w", err)
	}
	// Unmarshal the Command Data
	log.Printf("Converting Byte to Struct Object")
	var c DeployCommand
	if err := json.Unmarshal(msg.Message.Data, &c); err != nil {
		log.Printf("Failed to unmarshal to command, assuming bad command")
		return nil
	}

	// Create a new Cloud Deploy client
	deployClient, err := deploy.NewCloudDeployClient(ctx)
	if err != nil {
		return fmt.Errorf("error creating Cloud Deploy client: %v", err)
	}
	defer deployClient.Close()

	switch c.Commmand {
	case "CreateRelease":
		if err := cdCreateRelease(ctx, *deployClient, &c); err != nil {
			return fmt.Errorf("Create Release Failed: %v", err)
		}
	}
	return nil
}

func cdCreateRelease(ctx context.Context, d deploy.CloudDeployClient, c *DeployCommand) error {
	releaseOp, err := d.CreateRelease(ctx, &c.CreateRelease)
	if err != nil {
		return fmt.Errorf("error creating release request: %v", err)
	}
	log.Printf("Created release operation: %s", releaseOp.Name())

	_, err = releaseOp.Wait(ctx)
	if err != nil {
		return fmt.Errorf("error on release operation: %v", err)
	}
	log.Printf("Create Release Operation Completed")
	return nil
}
