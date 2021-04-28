// +build integration

package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"syreclabs.com/go/faker"
)

func integrationTestSetup(t *testing.T) *testConfig {
	t.Helper()

	projectID := os.Getenv("PROJECT")
	if projectID == "" {
		t.Fatalf("PROJECT is not defined")
	}

	// NOTE: We still don't know when to cancel.
	ctx, cancel := context.WithCancel(context.Background())

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("unable to create new pubsub client: %v", err)
	}

	topic1 := createTopic(ctx, t, client)
	trigger := &pubsuber{
		Topic:        topic1,
		Subscription: createSubscription(ctx, t, client, topic1),
	}

	topic2 := createTopic(ctx, t, client)
	result := &pubsuber{
		Topic:        topic2,
		Subscription: createSubscription(ctx, t, client, topic2),
	}

	return &testConfig{
		Trigger:  trigger,
		Result:   result,
		Canceler: cancel,
		Context:  ctx,
	}
}

type pubsuber struct {
	Topic        *pubsub.Topic
	Subscription *pubsub.Subscription
}

// nolint golint
func createTopic(
	ctx context.Context,
	t *testing.T,
	client *pubsub.Client,
) *pubsub.Topic {
	t.Helper()

	topicName := faker.Internet().Slug()
	t.Log("[DEBUG] creating topic: ", topicName)
	topic, err := client.CreateTopic(ctx, topicName)
	if err != nil {
		t.Fatalf("unable to create new pubsub topic: %v", err)
	}

	return topic
}

func createSubscription(
	ctx context.Context, t *testing.T, client *pubsub.Client, topic *pubsub.Topic,
) *pubsub.Subscription {
	t.Helper()

	subName := faker.Lorem().Word() + faker.Lorem().Word() + faker.Lorem().Word()

	t.Log("[DEBUG] creating subscription: ", subName)

	sub, err := client.CreateSubscription(
		ctx,
		subName,
		pubsub.SubscriptionConfig{
			Topic:       topic,
			AckDeadline: 20 * time.Second,
		})
	if err != nil {
		t.Fatalf("unable to create new pubsub subscription: %v", err)
	}

	return sub
}

func integrationTestCleanup(t *testing.T, cfg *testConfig) {
	t.Helper()

	t.Log("[DEBUG]: deleting trigger's subscription")
	if err := cfg.Trigger.Subscription.Delete(cfg.Context); err != nil {
		t.Logf("[WARNING]: unable to delete trigger's subscription, you to delete it manually: %v", err)
	}
	t.Log("[DEBUG]: deleting trigger's topic")
	if err := cfg.Trigger.Topic.Delete(cfg.Context); err != nil {
		t.Logf("[WARNING]: unable to delete trigger's topic, you to delete it manually: %v", err)
	}
	t.Log("[DEBUG]: deleting result's subscription")
	if err := cfg.Result.Subscription.Delete(cfg.Context); err != nil {
		t.Logf("[WARNING]: unable to delete result's subscription, you to delete it manually: %v", err)
	}
	t.Log("[DEBUG]: deleting result's topic")
	if err := cfg.Result.Topic.Delete(cfg.Context); err != nil {
		t.Logf("[WARNING]: unable to delete result's topic, you to delete it manually: %v", err)
	}
}

type testConfig struct {
	Client *pubsub.Client

	Trigger *pubsuber
	Result  *pubsuber

	Canceler context.CancelFunc
	Context  context.Context
}

func TestRunner_Run_Integration(t *testing.T) {
	cfg := integrationTestSetup(t)
	t.Cleanup(func() { integrationTestCleanup(t, cfg) })

	runner := &Runner{
		Publisher: cfg.Trigger.Topic,
		Receiver:  cfg.Result.Subscription,
	}

	trigger := &Event{
		BuildID:    "test1234",
		Repository: "github.com/royge/build2gke.git",
		Branch:     "develop",
		Tag:        "v1.0.0",
		Status:     Pending,
	}

	result := &Event{
		BuildID:    "test1234",
		Repository: "github.com/royge/build2gke.git",
		Branch:     "develop",
		Tag:        "v1.0.0",
		Status:     Success,
	}

	doneCh := make(chan bool, 1)

	go func(t *testing.T) {
		t.Helper()

		t.Log("[DEBUG] waiting for deployment trigger")
		err := cfg.Trigger.Subscription.Receive(cfg.Context, func(ctx context.Context, msg *pubsub.Message) {
			got := Event{}
			if err := json.Unmarshal(msg.Data, &got); err != nil {
				t.Errorf("unable to unmarshal deployment event: %v", err)
				return
			}

			t.Logf("[DEBUG] deployment event received: %v", got)

			if got.BuildID != trigger.BuildID {
				t.Errorf("want build id %v, got %v", trigger.BuildID, got.BuildID)
			}
			msg.Ack()
		})
		if err != nil {
			t.Errorf("error receiving deployment trigger: %v", err)
		}
	}(t)

	go func(t *testing.T) {
		t.Helper()

		if err := runner.Run(cfg.Context, trigger); err != nil {
			t.Errorf("unable to run the runner: %v", err)
			return
		}

		doneCh <- true
	}(t)

	go func(t *testing.T) {
		t.Helper()

		t.Logf("[DEBUG] publishing deployment results: %v", result)

		data, err := json.Marshal(result)
		if err != nil {
			t.Errorf("unable to marshal results data: %v", err)
			return
		}

		cfg.Result.Topic.Publish(cfg.Context, &pubsub.Message{
			Data: data,
		})
	}(t)

	<-doneCh
	close(doneCh)
}
