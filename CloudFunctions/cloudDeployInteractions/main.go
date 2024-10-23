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
	Commmand       string                         `json:"command"`
	CreateRelease  deploypb.CreateReleaseRequest  `json:"createReleaseRequest"`
	CreateRollout  deploypb.CreateRolloutRequest  `json:"createRolloutRequest"`
	ApproveRollout deploypb.ApproveRolloutRequest `json:"approveRolloutRequest"`
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

	// Depending on how big this list get's we should probably
	// make a dictionary style object with this mapping. But for now here we are
	switch c.Commmand {
	case "CreateRelease":
		if err := cdCreateRelease(ctx, *deployClient, &c.CreateRelease); err != nil {
			_ = fmt.Errorf("create release failed: %v", err)
			return nil
		}
	case "CreateRollout":
		if err := cdCreateRollout(ctx, *deployClient, &c.CreateRollout); err != nil {
			_ = fmt.Errorf("create rollout failed: %v", err)
			return nil
		}
	case "ApproveRollout":
		if err := cdApproveRollout(ctx, *deployClient, &c.ApproveRollout); err != nil {
			_ = fmt.Errorf("approve rollout failed: %v", err)
			return nil
		}
	}
	return nil
}

func cdCreateRelease(ctx context.Context, d deploy.CloudDeployClient, c *deploypb.CreateReleaseRequest) error {
	releaseOp, err := d.CreateRelease(ctx, c)
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

func cdCreateRollout(ctx context.Context, d deploy.CloudDeployClient, c *deploypb.CreateRolloutRequest) error {
	rollout, err := d.CreateRollout(ctx, c)
	if err != nil {
		return fmt.Errorf("error creating rollout request: %v", err)
	}
	log.Printf("Created Rollout Request: %v", rollout.Name())
	_, err = rollout.Wait(ctx)
	if err != nil {
		return fmt.Errorf("error on rollout operation: %v", err)
	}
	log.Printf("Create Rollout Operation Completed")
	return nil
}

func cdApproveRollout(ctx context.Context, d deploy.CloudDeployClient, c *deploypb.ApproveRolloutRequest) error {
	_, err := d.ApproveRollout(ctx, c)
	if err != nil {
		return fmt.Errorf("error approving rollout request operation: %v", err)
	}
	log.Printf("Approved Rollout")
	return nil
}
