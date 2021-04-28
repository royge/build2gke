package main

import (
	"context"
	"encoding/json"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/mock"
)

func TestRunner_Run(t *testing.T) {
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

	receiver := new(receiverMock)
	receiver.On(
		"Receive",
		ctx,
		mock.AnythingOfType("func(context.Context, *pubsub.Message)"),
	).Return(nil)
	defer receiver.AssertExpectations(t)

	runner := Runner{publisher, receiver}
	if err := runner.Run(ctx, &event); err != nil {
		t.Fatalf("failure to run the runner: %v", err)
	}
}
