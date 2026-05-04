package codevaldpubsub

import "errors"

var (
	ErrTopicNotFound          = errors.New("topic not found")
	ErrTopicAlreadyRegistered = errors.New("topic already registered")
	ErrEventNotFound          = errors.New("event not found")
	ErrSubscriptionNotFound   = errors.New("subscription not found")
	ErrDeliveryNotFound       = errors.New("delivery not found")
)
