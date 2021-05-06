package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"cloud.google.com/go/pubsub"
)

type Worker struct {
	Receiver  Receiver
	Publisher Publisher

	Exit    chan interface{}
	Actions []ActionFunc
}

type ActionFunc func(context.Context, *Event) error

func (w *Worker) Do(ctx context.Context) error {
	jobs, err := w.watchForDeployment(ctx, w.Exit)
	if err != nil {
		return err
	}
	doneJobs, err := w.doDeployment(ctx, w.Exit, jobs)
	if err != nil {
		return err
	}
	return w.sendResults(ctx, w.Exit, doneJobs)
}

func (w *Worker) watchForDeployment(
	ctx context.Context, done <-chan interface{},
) (<-chan Event, error) {
	var err error
	jobs := make(chan Event)

	go func() {
		defer close(jobs)

		cctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// TODO: Handles error accordingly.
		// nolint errcheck
		w.Receiver.Receive(cctx, func(ctx context.Context, msg *pubsub.Message) {
			event := Event{}
			if err = json.Unmarshal(msg.Data, &event); err != nil {
				return
			}
			if event.Status == Pending {
				jobs <- event

				msg.Ack()
			}
		})

		<-done
	}()

	return jobs, err
}

func (w *Worker) doDeployment(
	ctx context.Context, done <-chan interface{}, jobs <-chan Event,
) (<-chan Event, error) {
	doneJobs := make(chan Event)
	go func() {
		defer close(doneJobs)
		for job := range jobs {
			job := job // To avoid possible race.
			select {
			case <-done:
				return
			default:
				for _, fn := range w.Actions {
					if err := fn(ctx, &job); err != nil {
						job.Status = Error
					} else {
						job.Status = Success
					}
				}
				doneJobs <- job
			}
		}
	}()

	return doneJobs, nil
}

func (w *Worker) sendResults(
	ctx context.Context, done <-chan interface{}, doneJobs <-chan Event,
) error {
	for job := range doneJobs {
		job := job // To avoid possible race condition.
		select {
		case <-done:
			return nil
		default:
			data, err := json.Marshal(&job)
			if err != nil {
				return fmt.Errorf("unable to marshal event data: %w", err)
			}
			// NOTE: We assume that all publishing is successful for now.
			w.Publisher.Publish(ctx, &pubsub.Message{
				Data: data,
			})
		}
	}
	return nil
}

func fetchChangesFromRepo(_ context.Context, event *Event) (err error) {
	log.Println("cloning repository", event.Repository, "into", event.BuildID)

	cmd := exec.Command("git", "clone", event.Repository, event.BuildID)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unable to clone repository: %w", err)
	}

	log.Println("changing working directory into", event.BuildID)

	if err := os.Chdir(event.BuildID); err != nil {
		return fmt.Errorf("unable to change to build directory: %w", err)
	}

	log.Println("fetching git branches")

	cmd = exec.Command("git", "fetch")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unable to fetch git branches: %w", err)
	}

	log.Println("checking out", event.Branch, "branch")

	cmd = exec.Command("git", "checkout", event.Branch)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unable to checkout GIT branch: %w", err)
	}

	return nil
}

func deployToGKE(_ context.Context, event *Event) (err error) {
	args := strings.Split(event.Command, " ")
	cmd := exec.Command(args[0], args[1:]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unable to execute deployment: %w", err)
	}

	return nil
}
