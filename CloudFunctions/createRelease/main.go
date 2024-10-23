package example

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"

	deploy "cloud.google.com/go/deploy/apiv1"
	"cloud.google.com/go/deploy/apiv1/deploypb"
	"cloud.google.com/go/pubsub"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/codingconcepts/env"
)

type config struct {
	// Truthfully project ID and location might be able to be gathered by the function instead of env
	ProjectId   string `env:"PROJECTID" required:"true"`
	Location    string `env:"LOCATION" required:"true"`
	Pipeline    string `env:"PIPELINE" required:"true"`
	TriggerID   string `env:"TRIGGER" required:"true"`
	SendTopicID string `env:"SENDTOPICID" required:"true"`
}

var c config

func init() {
	functions.CloudEvent("deployTrigger", deployTrigger)
	//Load env variables using "github.com/codingconcepts/env"
	if err := env.Set(&c); err != nil {
		_ = fmt.Errorf("error getting env: %s", err)
	}
}

type PubSubMessage struct {
	Data []byte `json:"data"`
}

type MessagePublishedData struct {
	Message PubSubMessage
}

type CommandMessage struct {
	Commmand      string                        `json:"command"`
	CreateRelease deploypb.CreateReleaseRequest `json:"createReleaseRequest"`
}

func deployTrigger(ctx context.Context, e event.Event) error {
	log.Printf("Deploy trigger function invoked")

	// Parse the Pub/Sub message data
	var msg MessagePublishedData
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("event.DataAs: %w", err)
	}

	// Unmarshal the CloudBuild data
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
	if err != nil {
		return fmt.Errorf("error getting delivery pipeline: %v", err)
	}

	randomID, err := generateRandomID(6) // Generate a random ID of 6 bytes (12 hex characters)
	if err != nil {
		log.Fatalf("Error generating random ID: %v", err)
	}

	// Use the random ID as the release ID
	releaseID := fmt.Sprintf("release-%s", randomID)

	// Create a new release request
	var command = CommandMessage{
		Commmand: "CreateRelease",
		CreateRelease: deploypb.CreateReleaseRequest{
			Parent:    pipeline.Name,
			ReleaseId: releaseID, // Use the JIRA issue key as the release ID
			Release: &deploypb.Release{
				// Configure the release (e.g., Skaffold configuration)
				BuildArtifacts: []*deploypb.BuildArtifact{
					{
						// Tag == Container Image
						Tag: image,
						// Image == The template substitution variable in run.yaml
						Image: "pizza",
					},
				},
				SkaffoldConfigUri: fmt.Sprintf("%s/%s.tar.gz",
					buildNotification.Substitutions.DeployGCS,
					buildNotification.Substitutions.CommitSha,
				), // This is needed as we upload to GCS from Cloud Build
				SkaffoldConfigPath: "skaffold.yaml", // Replace with your Skaffold config path
			},
		},
	}
	err = sendCommandPubSub(ctx, &command)
	if err != nil {
		return fmt.Errorf("failed to send pubsub command: %v", err)
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

// This should be in a shared code folder
func sendCommandPubSub(ctx context.Context, m *CommandMessage) error {
	client, err := pubsub.NewClient(ctx, c.ProjectId)
	if err != nil {
		return fmt.Errorf("pubsub.NewClient: %v", err)
	}
	defer client.Close()
	t := client.Topic(c.SendTopicID)
	// Marshal the CommandMessage into a JSON byte slice
	jsonData, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("json.Marshal: %v", err)
	}
	log.Printf("Sending message to PubSub")
	result := t.Publish(ctx, &pubsub.Message{
		Data: jsonData, // Use the JSON byte slice here
	})
	// Block until the result is returned and a server-generated
	// ID is returned for the published message.
	id, err := result.Get(ctx)
	log.Printf("ID: %s, err: %v", id, err)
	if err != nil {
		fmt.Printf("Get: %v", err)
		return nil

	}
	log.Printf("Published a message; msg ID: %v\n", id)
	return nil
}
