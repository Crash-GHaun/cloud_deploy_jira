package example

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
)

type PubSubMessage struct {
	Data []byte `json:"data"`
}

type DeployNotification struct {
	Type    string  `json:"type"`
	Rollout Rollout `json:"rollout"`
	// Add other fields as needed based on the Cloud Deploy notification structure
}

type Rollout struct {
	Name  string `json:"name"`
	State string `json:"state"`
	// Add other fields as needed
}

func init() {
	functions.CloudEvent("cloudDeployPubSub", cloudDeployPubSub)
}

func cloudDeployPubSub(ctx context.Context, e event.Event) error {
	log.Printf("Jira update function invoked")

	// Parse the Pub/Sub message data
	var message PubSubMessage
	if err := json.Unmarshal(e.Data(), &message); err != nil {
		return fmt.Errorf("error parsing Pub/Sub message: %v", err)
	}

	// Unmarshal the Cloud Deploy notification data
	var deployNotification DeployNotification
	if err := json.Unmarshal(message.Data, &deployNotification); err != nil {
		return fmt.Errorf("error parsing Cloud Deploy notification: %v", err)
	}

	// Extract relevant information from the notification
	rolloutState := deployNotification.Rollout.State
	rolloutName := deployNotification.Rollout.Name
	// ... extract other necessary details like the release name or issue key

	log.Printf("Received Cloud Deploy notification for rollout: %s, state: %s", rolloutName, rolloutState)

	// Construct the JIRA API URL
	jiraAPIUrl := fmt.Sprintf("%s/rest/api/2/issue/%s", os.Getenv("JIRA_BASE_URL"), "ISSUE-123") // Replace ISSUE-123 with the actual issue key

	// Construct the request body with the deployment status update
	requestBody, err := json.Marshal(map[string]interface{}{
		"update": map[string]interface{}{
			"comment": []map[string]interface{}{
				{
					"add": map[string]interface{}{
						"body": fmt.Sprintf("Cloud Deploy rollout status updated: %s", rolloutState),
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("error creating request body: %v", err)
	}

	// Create a new HTTP client
	client := &http.Client{}

	// Create a new PUT request
	req, err := http.NewRequest(http.MethodPut, jiraAPIUrl, strings.NewReader(string(requestBody)))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	// Set the request headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", os.Getenv("JIRA_API_TOKEN"))) // Replace with your JIRA API token

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request to JIRA: %v", err)
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("JIRA API request failed with status code: %d", resp.StatusCode)
	}

	log.Printf("JIRA issue updated successfully")
	return nil
}

// Approve the release if require_approval is set to true in the target
// Still need to update this step so it works
//if pipeline.GetSerialPipeline().GetStages()[0].GetTargetId() != "" {
//	_, err = deployClient.ApproveRollout(ctx, &deploypb.ApproveRolloutRequest{
//		Name: fmt.Sprintf("%s/rollouts/%s", pipeline.GetSerialPipeline().GetStages()[0].GetTargetId(), release.Name()),
//	})
//	if err != nil {
//		return fmt.Errorf("error approving rollout: %v", err)
//	}
//	log.Printf("Approved rollout for release: %s", release.Name())
//}
/*
	rollout, err := deployClient.CreateRollout(ctx, &deploypb.CreateRolloutRequest{
		Parent:    release.Name, // Reference the created release
		RolloutId: fmt.Sprintf("rollout-%s", randomID),
		Rollout: &deploypb.Rollout{
			TargetId: "random-date-service", // Replace with your target ID
		},
	})

	if err != nil {
		log.Fatalf("Error creating rollout: %v", err)
	} else {
		log.Printf("Rollout created: %v", rollout)
	}*/
