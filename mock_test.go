package main

import (
	"context"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/mock"
)

type publisherMock struct {
	mock.Mock
}

func (p *publisherMock) Publish(
	ctx context.Context,
	msg *pubsub.Message,
) *pubsub.PublishResult {
	args := p.Called(ctx, msg)
	return args.Get(0).(*pubsub.PublishResult)
}

type receiverMock struct {
	mock.Mock
}

func (r *receiverMock) Receive(
	ctx context.Context,
	fn func(ctx context.Context, msg *pubsub.Message),
) error {
	args := r.Called(ctx, fn)
	return args.Error(0)
}
