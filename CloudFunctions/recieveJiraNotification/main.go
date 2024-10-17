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
	"google.golang.org/api/iterator"
)

func init() {
	functions.CloudEvent("deployTrigger", deployTrigger)
}

type PubSubMessage struct {
	Data []byte `json:"data"`
}

type JiraNotification struct {
	// Add fields here to match the structure of your JIRA notification
	IssueKey   string `json:"issueKey"`
	ChangeType string `json:"changeType"`
	// ... other relevant fields
}

func deployTrigger(ctx context.Context, e event.Event) error {
	log.Printf("Deploy trigger function invoked")

	// Parse the Pub/Sub message data
	var message PubSubMessage
	if err := json.Unmarshal(e.Data(), &message); err != nil {
		return fmt.Errorf("error parsing Pub/Sub message: %v", err)
	}

	// Unmarshal the JIRA notification data
	var jiraNotification JiraNotification
	if err := json.Unmarshal(message.Data, &jiraNotification); err != nil {
		return fmt.Errorf("error parsing JIRA notification: %v", err)
	}

	// Extract relevant information from the JIRA notification
	issueKey := jiraNotification.IssueKey
	// ... extract other necessary details

	log.Printf("Received JIRA notification for issue: %s", issueKey)

	// Create a new Cloud Deploy client
	deployClient, err := deploy.NewCloudDeployClient(ctx)
	if err != nil {
		return fmt.Errorf("error creating Cloud Deploy client: %v", err)
	}
	defer deployClient.Close()

	// Get the delivery pipeline
	pipelineName := fmt.Sprintf("projects/%s/locations/%s/deliveryPipelines/%s", "your-project-id", "us-central1", "jira-triggered-pipeline") // Update with your actual pipeline name
	pipeline, err := deployClient.GetDeliveryPipeline(ctx, &deploypb.GetDeliveryPipelineRequest{
		Name: pipelineName,
	})
	if err != nil {
		return fmt.Errorf("error getting delivery pipeline: %v", err)
	}

	// Create a new release
	release, err := deployClient.CreateRelease(ctx, &deploypb.CreateReleaseRequest{
		Parent:    pipelineName,
		ReleaseId: issueKey, // Use the JIRA issue key as the release ID
		Release: &deploypb.Release{
			// Configure the release (e.g., Skaffold configuration)
			SkaffoldConfigPath: "path/to/skaffold.yaml", // Replace with your Skaffold config path
		},
	})
	if err != nil {
		return fmt.Errorf("error creating release: %v", err)
	}

	log.Printf("Created release: %s", release.Name)

	// Approve the release if require_approval is set to true in the target
	// Still need to update this step so it works
	if pipeline.GetSerialPipeline().GetStages()[0].GetTargetId() != "" {
		_, err = deployClient.ApproveRollout(ctx, &deploypb.ApproveRolloutRequest{
			Name: fmt.Sprintf("%s/rollouts/%s", pipeline.GetSerialPipeline().GetStages()[0].GetTargetId(), release.Name()),
		})
		if err != nil {
			return fmt.Errorf("error approving rollout: %v", err)
		}
		log.Printf("Approved rollout for release: %s", release.Name)
	}

	// Get an iterator to list all rollouts for the release
	rolloutIter := deployClient.ListRollouts(ctx, &deploypb.ListRolloutsRequest{
		Parent: pipeline.GetSerialPipeline().GetStages()[0].GetTargetId(),
		Filter: fmt.Sprintf("release.name = %q", release.Name()),
	})

	// Wait for the rollout to start
	for {
		rollout, err := rolloutIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("error listing rollouts: %v", err)
		}

		log.Printf("Rollout status: %s", rollout.GetState())

		if rollout.GetState() != deploypb.Rollout_PENDING {
			// Rollout has started
			break
		}
	}

	log.Printf("Deployment triggered successfully")
	return nil
}
