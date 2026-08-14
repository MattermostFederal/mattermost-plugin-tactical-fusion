package decorators

import "fmt"

// Registry holds the decorators in registration order.
//
// Order matters only as a tiebreak: when two matches overlap, the longest wins
// and registration order decides equal-length ties. That keeps adding a
// decorator from silently changing an existing one's behavior.
type Registry struct {
	ordered []Decorator
	byType  map[string]Decorator
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byType: map[string]Decorator{}}
}

// Register adds a decorator. It returns an error rather than panicking on a
// duplicate type, so OnActivate can surface the problem instead of taking the
// whole plugin down.
func (r *Registry) Register(d Decorator) error {
	t := d.Type()
	if t == "" {
		return fmt.Errorf("decorator type must not be empty")
	}
	if _, dup := r.byType[t]; dup {
		return fmt.Errorf("decorator type %q is already registered", t)
	}
	r.byType[t] = d
	r.ordered = append(r.ordered, d)
	return nil
}

// All returns the decorators in registration order.
func (r *Registry) All() []Decorator {
	return r.ordered
}

// Get returns the decorator for a type, or nil if none is registered.
func (r *Registry) Get(t string) Decorator {
	return r.byType[t]
}

// NewDefaultRegistry builds a registry from the decorators this plugin ships.
// They are passed in by OnActivate, so adding one is one argument there plus one
// directory, and nothing in this package changes.
//
// This deliberately returns a fresh registry rather than mutating a package
// level one. A global would be written by Register while MessageWillBePosted
// and ServeHTTP read it concurrently, which is a data race, and it would also
// make activation non-idempotent: a second OnActivate in the same process would
// fail permanently on "already registered".
func NewDefaultRegistry(ds ...Decorator) (*Registry, error) {
	r := NewRegistry()
	for _, d := range ds {
		if err := r.Register(d); err != nil {
			return nil, err
		}
	}
	return r, nil
}
