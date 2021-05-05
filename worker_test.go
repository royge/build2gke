package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/mock"
)

func TestDeployToGKE(t *testing.T) {
	event := Event{
		BuildID:    "er45y76u",
		Repository: "git@github.com:royge/build2gke.git",
		Branch:     "main",
		Command:    "make echo",
	}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		if err := deployToGKE(ctx, &event); err != nil {
			t.Errorf("unable to deploy to GKE: %v", err)
		}
	})

	t.Run("Error", func(t *testing.T) {
		event.Command = "make unknown"
		if err := deployToGKE(ctx, &event); err == nil {
			t.Error("expected error, but got nil")
		}
	})
}

func TestWorker_WatchForDeployment(t *testing.T) {
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
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()

	receiver.On(
		"Receive",
		cctx,
		mock.AnythingOfType("func(context.Context, *pubsub.Message)"),
	).Return(nil)
	defer receiver.AssertExpectations(t)

	exitCh := make(chan interface{})

	worker := Worker{receiver, publisher, exitCh, nil}

	go func(t *testing.T) {
		t.Helper()

		if _, err := worker.watchForDeployment(ctx, exitCh); err != nil {
			t.Fatalf("failure to run the runner: %v", err)
		}
	}(t)
}

func TestWorker_DoDeployment(t *testing.T) {
	var err error

	done := make(chan interface{})
	jobs := make(chan Event)
	doneJobs := make(chan Event)
	ctx := context.Background()

	worker := Worker{
		Actions: []ActionFunc{
			func(ctx context.Context, event *Event) error {
				t.Log("[DEBUG] job: ", event)
				return nil
			},
		},
	}

	go func(t *testing.T) {
		t.Helper()

		doneJobs, err = worker.doDeployment(ctx, done, jobs)
		if err != nil {
			t.Errorf("unable to do deployment: %v", err)
		}

		for job := range doneJobs {
			t.Log("[DEBUG] done job:", job)
			if job.Status != Success {
				t.Errorf(
					"want job status to be %v, got %v",
					Success,
					job.Status,
				)
			}
		}
	}(t)

	go func(t *testing.T) {
		t.Helper()

		jobs <- Event{
			BuildID: "123456",
			Status:  Pending,
		}

		t.Log("[DEBUG] waiting for 250 milliseconds...")
		time.Sleep(250 * time.Millisecond)

		jobs <- Event{
			BuildID: "123457",
			Status:  Pending,
		}

		t.Log("[DEBUG] waiting for 2 seconds...")
		time.Sleep(2 * time.Second)

		jobs <- Event{
			BuildID: "123458",
			Status:  Pending,
		}

		t.Log("[DEBUG] waiting for 3 seconds...")
		time.Sleep(3 * time.Second)

		jobs <- Event{
			BuildID: "123459",
			Status:  Pending,
		}

		done <- true
	}(t)

	<-done
}
