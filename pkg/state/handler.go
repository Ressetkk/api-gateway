package state

import (
	"context"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Handler is an interface that describes a object that can be used as a part
// of state pipeline. This handler should implement independent logic that
// handles specific part of a resource, as part of a Handler pipeline.
type Handler interface {
	// Handle is a function that executes independent logic describe in the
	// sequential pipeline of handlers. This function has gets passed current
	// context and also a State struct it is working with.
	Handle(ctx context.Context, s *State) error
}

// HandlerFunc is a simple type of handler that implements a Handler interface
// as single function.
type HandlerFunc func(ctx context.Context, s *State) error

// Handle is an implementation of Handler interface. It calls underlying
// HandlerFunc function.
func (h HandlerFunc) Handle(ctx context.Context, s *State) error {
	return h(ctx, s)
}

// State defines the actual context of the underlying state. It serves as an
// interface for handlers to alter the kubernetes resources and eventually stop
// running pipeline without returning an error.
type State struct {
	stop   context.CancelFunc
	client client.Client
	log    logr.Logger
}

// Log returns logr.Logger to interact with logging interface.
func (s *State) Log() logr.Logger {
	return s.log
}

// Client returns client.Client interface for interfacing with kubernetes API.
func (s *State) Client() client.Client {
	return s.client
}

// Stop stops the execution of the state. Subsequent calls to this function
// do nothing.
func (s *State) Stop() {
	s.stop()
}

// Run executes chain of Handlers against context. It wraps provided context
// with cancel function and passes this as a stop function to the underlying State.
func (r *Runner) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	r.state.stop = cancel
	for _, h := range r.handlers {
		select {
		case <-ctx.Done():
			break
		default:
			if err := h.Handle(ctx, &r.state); err != nil {
				return err
			}
		}
	}
	return nil
}

// New is a construction function that returns Runner struct. It respects the
// Option functional parameters as modifiers.
func New(client client.Client, opts ...Option) *Runner {
	s := State{
		client: client,
	}
	for _, opt := range opts {
		opt(&s)
	}
	return &Runner{state: s}
}

// Runner describes a struct for running defined chain of handlers with a state.
type Runner struct {
	handlers []Handler
	state    State
}

// AddHandlers adds structs that implement Handler interface into chain that is
// executed in order.
func (r *Runner) AddHandlers(handler ...Handler) {
	r.handlers = append(r.handlers, handler...)
}
