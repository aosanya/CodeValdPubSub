package codevaldpubsub

import "errors"

// ── Topic errors ──────────────────────────────────────────────────────────────

// ErrTopicNotFound is returned by [PubSubManager.GetTopic],
// [PubSubManager.GetTopicByPattern], and [PubSubManager.DeleteTopic] when no
// Topic entity with the given ID or pattern exists.
var ErrTopicNotFound = errors.New("topic not found")

// ErrTopicAlreadyRegistered is returned by [PubSubManager.RegisterTopic] when
// a Topic entity with the same pattern already exists.
var ErrTopicAlreadyRegistered = errors.New("topic pattern already registered")

// ── Event errors ──────────────────────────────────────────────────────────────

// ErrEventNotFound is returned by [PubSubManager.GetEvent] when no Event
// entity with the given ID exists.
var ErrEventNotFound = errors.New("event not found")

// ── Subscription errors ───────────────────────────────────────────────────────

// ErrSubscriptionNotFound is returned by [PubSubManager.GetSubscription],
// [PubSubManager.UpdateSubscription], and [PubSubManager.Unsubscribe] when no
// Subscription entity with the given ID exists.
var ErrSubscriptionNotFound = errors.New("subscription not found")

// ErrSubscriptionNotCancellable is returned by [PubSubManager.Unsubscribe]
// when the subscription is already in the "cancelled" state.
var ErrSubscriptionNotCancellable = errors.New("subscription is already cancelled")
