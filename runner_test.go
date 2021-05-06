package main

import (
	"context"
	"encoding/json"
	"testing"

	"cloud.google.com/go/pubsub"
)

func TestRunner_SendDeploymentTrigger(t *testing.T) {
	ctx := context.Background()
	event := Event{
		BuildID:    "12345",
		Repository: "royge/build2gke",
		Branch:     "develop",
		Status:     Pending,
	}
	publisher := new(publisherMock)
	data, err := json.Marshal(&event)
	if err != nil {
		t.Fatalf("unable to marshal event data: %v", err)
	}
	publisher.On("Publish", ctx, &pubsub.Message{Data: data}).Return(&pubsub.PublishResult{})
	defer publisher.AssertExpectations(t)

	runner := Runner{publisher, nil, nil}

	if err := runner.sendDeploymentTrigger(ctx, &event); err != nil {
		t.Fatalf("failure to run the runner: %v", err)
	}
}
