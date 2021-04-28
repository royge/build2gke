package main

import (
	"context"
)

type Worker struct {
	Receiver  Receiver
	Publisher Publisher
}

func (w *Worker) Do(ctx context.Context) error {
	return nil
}
