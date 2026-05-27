package shapeshifter

import (
	"errors"
	"fmt"
	"sync"
)

type HandlerFunc func(input map[string]any) (map[string]any, error)

type HandlerOptions struct {
	PreviewSafe bool
}

type RegisteredHandler struct {
	Name        string
	Fn          HandlerFunc
	PreviewSafe bool
}

type HandlerSnapshot struct {
	handlers map[string]RegisteredHandler
}

type Registry struct {
	mu       sync.Mutex
	handlers map[string]RegisteredHandler
}

func NewRegistry() *Registry {
	return &Registry{handlers: map[string]RegisteredHandler{}}
}

func (r *Registry) Register(name string, fn HandlerFunc, opts ...HandlerOptions) error {
	if r == nil {
		return errors.New("shapeshifter: nil registry")
	}
	if name == "" {
		return errors.New("shapeshifter: handler name is required")
	}
	if fn == nil {
		return errors.New("shapeshifter: handler function is required")
	}
	if len(opts) > 1 {
		return errors.New("shapeshifter: at most one handler options value is supported")
	}
	var opt HandlerOptions
	if len(opts) == 1 {
		opt = opts[0]
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[name]; exists {
		return fmt.Errorf("shapeshifter: duplicate handler %q", name)
	}
	r.handlers[name] = RegisteredHandler{Name: name, Fn: fn, PreviewSafe: opt.PreviewSafe}
	return nil
}

func (r *Registry) Snapshot() HandlerSnapshot {
	if r == nil {
		return HandlerSnapshot{handlers: map[string]RegisteredHandler{}}
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make(map[string]RegisteredHandler, len(r.handlers))
	for name, handler := range r.handlers {
		out[name] = handler
	}
	return HandlerSnapshot{handlers: out}
}

func (s HandlerSnapshot) lookup(name string) (RegisteredHandler, bool) {
	handler, ok := s.handlers[name]
	return handler, ok
}
