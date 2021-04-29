package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"cloud.google.com/go/pubsub"
)

const (
	Pending uint = iota + 1
	Success
	Error
)

// Event defines the properties of cloud build event.
type Event struct {
	BuildID    string `json:"build_id"`
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	Tag        string `json:"tag"`
	Status     uint   `json:"status"`
}

type Publisher interface {
	Publish(context.Context, *pubsub.Message) *pubsub.PublishResult
}

type Receiver interface {
	Receive(
		context.Context,
		func(ctx context.Context, msg *pubsub.Message),
	) error
}

// Runner defines the requirements of cloud build runner.
type Runner struct {
	Publisher Publisher
	Receiver  Receiver
}

func (r *Runner) Run(ctx context.Context, event *Event) error {
	if err := r.sendDeploymentTrigger(ctx, event); err != nil {
		return fmt.Errorf("unable to send deployment event: %w", err)
	}

	if err := r.waitForDeploymentResult(ctx, event); err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}
	return nil
}

func (r *Runner) sendDeploymentTrigger(ctx context.Context, event *Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("unable to marshal event data: %w", err)
	}
	// NOTE: We assume that all publishing is successful for now.
	r.Publisher.Publish(ctx, &pubsub.Message{
		Data: data,
	})
	return nil
}

func (r *Runner) waitForDeploymentResult(ctx context.Context, event *Event) (err error) {
	eventStatusCh := make(chan uint)

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		// TODO: Handles error accordingly.
		// nolint errcheck
		r.Receiver.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			evnt := Event{}
			if err = json.Unmarshal(msg.Data, &evnt); err != nil {
				return
			}
			if evnt.BuildID == event.BuildID {
				eventStatusCh <- evnt.Status

				msg.Ack()
			}
		})
	}()

	status := <-eventStatusCh
	if status == Error {
		err = errors.New("deployment error")
	}

	cancel()
	close(eventStatusCh)

	return err
}
