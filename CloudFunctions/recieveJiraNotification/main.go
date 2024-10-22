package example

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"text/template"

	deploy "cloud.google.com/go/deploy/apiv1"
	"cloud.google.com/go/deploy/apiv1/deploypb"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/codingconcepts/env"
)

type config struct {
	ProjectId string `env:"PROJECTID" required:"true"`
	Location  string `env:"LOCATION" required:"true"`
	Pipeline  string `env:"PIPELINE" required:"true"`
	TriggerID string `env:"TRIGGER" required:"true"`
}

var c config

func init() {
	functions.CloudEvent("deployTrigger", deployTrigger)
	//Load env variables using "github.com/codingconcepts/env"
	if err := env.Set(&c); err != nil {
		fmt.Errorf("error getting env: %s", err)
	}
}

type SkaffoldConfig struct {
	Name  string
	Image string
}

type PubSubMessage struct {
	Data []byte `json:"data"`
}

type MessagePublishedData struct {
	Message PubSubMessage
}

// TODO (GHAUN): Get an example message from Jira?
type JiraNotification struct {
	// Add fields here to match the structure of your JIRA notification
	IssueKey   string `json:"issueKey"`
	ChangeType string `json:"changeType"`
	// ... other relevant fields
}

func deployTrigger(ctx context.Context, e event.Event) error {
	log.Printf("Deploy trigger function invoked")

	// Parse the Pub/Sub message data

	var msg MessagePublishedData
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("event.DataAs: %w", err)
	}

	// Only uncomment this if you don't think your message is being recieved/decoded properly.
	//log.Printf("JSON: %v", string(msg.Message.Data))

	// Unmarshal the CloudBuild/JIRA notification data
	log.Printf("Converting Byte to Struct Object")
	var buildNotification BuildMessage
	if err := json.Unmarshal(msg.Message.Data, &buildNotification); err != nil {
		return fmt.Errorf("error parsing JIRA notification: %v", err)
	}
	log.Printf("Checking if proper build")
	if buildNotification.BuildTriggerID != c.TriggerID || buildNotification.Status != "SUCCESS" {
		log.Printf("Build trigger ID or status does not match, returning early")
		// Acknowledge the event, depending on how the event system is expecting it
		return nil // Return nil to indicate successful processing of the event, even if we don't process further
	}
	log.Printf("Pulling relavent image")
	// Extract relevant information from the JIRA notification
	image := buildNotification.Artifacts.Images[0]
	// ... extract other necessary details

	log.Printf("Received Image from Cloud Build: %s", image)

	// Create a new Cloud Deploy client
	deployClient, err := deploy.NewCloudDeployClient(ctx)
	if err != nil {
		return fmt.Errorf("error creating Cloud Deploy client: %v", err)
	}
	defer deployClient.Close()

	// Get the delivery pipeline
	pipelineName := fmt.Sprintf("projects/%s/locations/%s/deliveryPipelines/%s", c.ProjectId, c.Location, c.Pipeline)
	pipeline, err := deployClient.GetDeliveryPipeline(ctx, &deploypb.GetDeliveryPipelineRequest{
		Name: pipelineName,
	})
	log.Printf("Pipeline: %v", pipeline)
	if err != nil {
		return fmt.Errorf("error getting delivery pipeline: %v", err)
	}

	// Update Template
	templatePath := "./serverless_function_source_code/skaffoldTemplate.yaml"
	config := SkaffoldConfig{
		Name:  "random-date-service",
		Image: image,
	}
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}
	outputFile, err := os.Create("skaffold.yaml")
	if err != nil {
		log.Fatalf("Error creating output file: %v", err)
	}
	defer outputFile.Close()

	err = tmpl.Execute(outputFile, config)
	if err != nil {
		log.Fatalf("Error executing tempalte: %v", err)
	}

	randomID, err := generateRandomID(6) // Generate a random ID of 6 bytes (12 hex characters)
	if err != nil {
		log.Fatalf("Error generating random ID: %v", err)
	}

	// Use the random ID as the release ID
	releaseID := fmt.Sprintf("release-%s", randomID)

	// Create a new release
	release, err := deployClient.CreateRelease(ctx, &deploypb.CreateReleaseRequest{
		Parent:    pipeline.Name,
		ReleaseId: releaseID, // Use the JIRA issue key as the release ID
		Release: &deploypb.Release{
			// Configure the release (e.g., Skaffold configuration)
			Name:               fmt.Sprintf(pipelineName + "/releases/areleaseforever"),
			SkaffoldConfigPath: "skaffold.yaml", // Replace with your Skaffold config path
		},
	})
	if err != nil {
		return fmt.Errorf("error creating release: %v", err)
	}

	log.Printf("Created release: %s", release.Name())

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

	rollout, err := deployClient.CreateRollout(ctx, &deploypb.CreateRolloutRequest{
		Parent:    release.Name(), // Reference the created release
		RolloutId: fmt.Sprintf("rollout-%s", randomID),
		Rollout: &deploypb.Rollout{
			TargetId: "random-date-service", // Replace with your target ID
		},
	})

	if err != nil {
		log.Fatalf("Error creating rollout: %v", err)
	} else {
		log.Printf("Rollout created: %v", rollout)
	}

	log.Printf("Deployment triggered successfully")
	return nil
}

func generateRandomID(length int) (string, error) {
	// Create a byte slice of the desired length
	bytes := make([]byte, length)
	// Fill the byte slice with random data
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// Return the hexadecimal string representation of the byte slice
	return hex.EncodeToString(bytes), nil
}
