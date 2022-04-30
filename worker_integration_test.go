//go:build integration
// +build integration

package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
)

func TestIntegration_FetchChangesFromRepo(t *testing.T) {
	event := Event{
		BuildID:    "er45y76u",
		Repository: "git@github.com:royge/build2gke.git",
		Branch:     "main",
	}
	ctx := context.Background()

	if err := fetchChangesFromRepo(ctx, &event); err != nil {
		t.Fatalf("unable to fetch changes from repository: %v", err)
	}

	if err := os.Chdir("../"); err != nil {
		t.Errorf("unable to change to build directory: %v", err)
	}

	if err := os.RemoveAll(event.BuildID); err != nil {
		t.Log("error:", err)
	}
}

func TestIntegration_Worker_WatchForDeployment(t *testing.T) {
	cfg := integrationTestSetup(t)
	t.Cleanup(func() { integrationTestCleanup(t, cfg) })

	trigger := &Event{
		BuildID:    "test1234",
		Repository: "github.com/royge/build2gke.git",
		Branch:     "develop",
		Tag:        "v1.0.0",
		Status:     Pending,
		Command:    "make echo",
	}

	worker := &Worker{
		Publisher: cfg.Result.Topic,
		Receiver:  cfg.Trigger.Subscription,
	}

	done := make(chan interface{})
	defer close(done)

	start := make(chan interface{})

	go func(t *testing.T) {
		t.Helper()

		jobs, err := worker.watchForDeployment(cfg.Context, done)
		if err != nil {
			t.Errorf("error watching for deployment: %v", err)
		}

		start <- true

		got := <-jobs
		if got.BuildID != trigger.BuildID {
			t.Errorf("want event build ID %v, got %v", trigger.BuildID, got.BuildID)
		}

		done <- true
	}(t)

	go func(t *testing.T) {
		t.Helper()

		<-start

		t.Logf("[INFO] publishing deployment trigger: %v", trigger)

		data, err := json.Marshal(trigger)
		if err != nil {
			t.Errorf("unable to marshal trigger data: %v", err)
			return
		}

		cfg.Trigger.Topic.Publish(cfg.Context, &pubsub.Message{
			Data: data,
		})
	}(t)

	<-done
}

func TestIntegration_Worker_SendResults(t *testing.T) {
	cfg := integrationTestSetup(t)
	t.Cleanup(func() { integrationTestCleanup(t, cfg) })

	result := Event{
		BuildID:    "test1234",
		Repository: "github.com/royge/build2gke.git",
		Branch:     "develop",
		Tag:        "v1.0.0",
		Status:     Success,
		Command:    "make echo",
	}

	doneJobs := make(chan Event)
	defer close(doneJobs)

	worker := &Worker{
		Publisher: cfg.Result.Topic,
		Receiver:  cfg.Trigger.Subscription,
		Exit:      make(chan interface{}),
	}
	defer close(worker.Exit)

	go func(t *testing.T) {
		t.Helper()

		t.Log("[INFO] waiting for deployment results")
		err := cfg.Result.Subscription.Receive(cfg.Context, func(ctx context.Context, msg *pubsub.Message) {
			got := Event{}
			if err := json.Unmarshal(msg.Data, &got); err != nil {
				t.Errorf("unable to unmarshal deployment result: %v", err)
				return
			}

			t.Logf("[INFO] deployment result received: %v", got)

			if got.BuildID != result.BuildID {
				t.Errorf("want build id %v, got %v", result.BuildID, got.BuildID)
			}
			msg.Ack()
		})
		if err != nil && err != context.Canceled {
			t.Errorf("error receiving deployment result: %v", err)
		}
	}(t)

	go func(t *testing.T) {
		t.Helper()

		err := worker.sendResults(cfg.Context, worker.Exit, doneJobs)
		if err != nil {
			t.Errorf("error watching for deployment: %v", err)
		}
	}(t)

	go func(t *testing.T) {
		doneJobs <- result
		worker.Exit <- true
	}(t)

	// NOTE: Waiting for the results to be completely published.
	// This time is dependent on network latency. You may adjust this
	// accordingly.
	time.Sleep(3 * time.Second)
	cfg.Cancel()
	<-worker.Exit
}
