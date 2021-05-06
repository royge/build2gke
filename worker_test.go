package main

import (
	"context"
	"testing"
	"time"

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
	receiver := new(receiverMock)

	receiver.On(
		"Receive",
		mock.Anything,
		mock.AnythingOfType("func(context.Context, *pubsub.Message)"),
	).Return(nil)

	exit := make(chan interface{})

	worker := Worker{
		Receiver: receiver,
		Exit:     exit,
	}

	go func(t *testing.T) {
		t.Helper()

		_, err := worker.watchForDeployment(ctx, exit)
		if err != nil {
			t.Fatalf("failure to run the runner: %v", err)
		}

		exit <- true
	}(t)

	<-exit
}

func TestWorker_DoDeployment(t *testing.T) {
	var err error

	done := make(chan interface{})
	jobs := make(chan Event)
	doneJobs := make(<-chan Event)
	ctx := context.Background()

	worker := Worker{
		Actions: []ActionFunc{
			func(ctx context.Context, event *Event) error {
				t.Log("[INFO] job: ", event)
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
			t.Log("[INFO] done job:", job)
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

		t.Log("[INFO] waiting for 250 milliseconds...")
		time.Sleep(250 * time.Millisecond)

		jobs <- Event{
			BuildID: "123457",
			Status:  Pending,
		}

		t.Log("[INFO] waiting for 2 seconds...")
		time.Sleep(2 * time.Second)

		jobs <- Event{
			BuildID: "123458",
			Status:  Pending,
		}

		t.Log("[INFO] waiting for 3 seconds...")
		time.Sleep(3 * time.Second)

		jobs <- Event{
			BuildID: "123459",
			Status:  Pending,
		}

		done <- true
	}(t)

	<-done
}
