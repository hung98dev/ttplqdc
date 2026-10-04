package queue

import "errors"

// Typed queue errors (errors.Is compatible). Wire codes map at the edge.
var (
	// ErrBackpressure means the durable admission gate could not reserve a
	// slot — the request never reached the database and nothing enqueued.
	ErrBackpressure = errors.New("queue: durable admission gate saturated")
	// ErrQueueFull means the bounded queue capacity is exhausted.
	ErrQueueFull = errors.New("queue: capacity exhausted")
	// ErrUnknownProducer means the record's producer/family/owner is not in
	// the closed save_rules.md registry — admission is refused.
	ErrUnknownProducer = errors.New("queue: unknown producer or family/owner mismatch")
	// ErrNoClientReceipt means a CLIENT command lacks durable receipt
	// admission — nothing is enqueued without it.
	ErrNoClientReceipt = errors.New("queue: client command lacks receipt admission")
	// ErrErasureFenced means the owner is under an erasure admission fence;
	// new subject work is refused and uncommitted commands are cancelled.
	ErrErasureFenced = errors.New("queue: owner erasure-fenced")
	// ErrAdmissionFailed means durable receipt admission failed (DB outage
	// or conflict); nothing was enqueued.
	ErrAdmissionFailed = errors.New("queue: receipt admission failed")
	// ErrShutdown means the queue is stopped and refuses new submissions.
	ErrShutdown = errors.New("queue: shut down")
)
