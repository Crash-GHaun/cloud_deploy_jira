package example

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
)

type PubSubMessage struct {
	Data []byte `json:"data"`
}

type MessagePublishedData struct {
	Message PubSubMessage
}

type Attributes struct {
	Attributes ApprovalsData `json:"attributes"`
}

type ApprovalsData struct {
	Action        string `json:"Action"`
	Rollout       string `json:"Rollout"`
	ReleaseId     string `json:"ReleaseId"`
	RolloutId     string `json:"RolloutId"`
	TargetId      string `json:"TargetId"`
	Location      string `json:"Location"`
	ProjectNumber string `json:"ProjectNumber"`
}

func init() {
	functions.CloudEvent("cloudDeployApprovals", cloudDeployApprovals)
}

func cloudDeployApprovals(ctx context.Context, e event.Event) error {
	log.Printf("Deploy Operations function invoked")
	// Parse the Pub/Sub message data
	var msg MessagePublishedData
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("event.DataAs: %w", err)
	}

	// Unmarshal the Cloud Deploy Operations data
	log.Printf("Converting Byte to Struct Object")
	var a Attributes
	if err := json.Unmarshal(msg.Message.Data, &a); err != nil {
		return fmt.Errorf("error parsing Pub Sub notification: %v", err)
	}
	// Temp for testing, if you are reading this I meant to delete :P
	log.Printf("Here are the attributes: %v", a)

	// Return nil to ack pubsub message
	return nil
}
