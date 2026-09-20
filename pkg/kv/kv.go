package kv

import (
	"context"
	"io"

	"github.com/go-faster/errors"

	"github.com/iyear/tdl/core/storage"
)

//go:generate go-enum --values --names --flag --nocase

// Driver
// ENUM(legacy, bolt, file)
type Driver string

const DriverTypeKey = "type"

type Meta map[string]map[string][]byte // namespace, key, value

type Storage interface {
	Name() string
	MigrateTo() (Meta, error)
	MigrateFrom(Meta) error
	Namespaces() ([]string, error)
	RemoveNamespace(ns string) error
	Open(ns string) (storage.Storage, error)
	io.Closer
}

var drivers = map[Driver]func(map[string]any) (Storage, error){}

func register(name Driver, fn func(map[string]any) (Storage, error)) {
	drivers[name] = fn
}

func New(driver Driver, opts map[string]any) (Storage, error) {
	if fn, ok := drivers[driver]; ok {
		return fn(opts)
	}

	return nil, errors.Errorf("unsupported driver: %s", driver)
}

func NewWithMap(o map[string]string) (Storage, error) {
	driver, err := ParseDriver(o[DriverTypeKey])
	if err != nil {
		return nil, errors.Wrap(err, "parse driver")
	}

	opts := make(map[string]any)
	for k, v := range o {
		if k == DriverTypeKey {
			continue
		}

		opts[k] = v
	}

	return New(driver, opts)
}

type ctxKey struct{}
type ownedKey struct{}

func With(ctx context.Context, kv Storage) context.Context {
	return WithOwned(ctx, kv)
}

func WithOwned(ctx context.Context, stg Storage) context.Context {
	ctx = context.WithValue(ctx, ctxKey{}, stg)
	return context.WithValue(ctx, ownedKey{}, true)
}

func WithBorrowed(ctx context.Context, stg Storage) context.Context {
	ctx = context.WithValue(ctx, ctxKey{}, stg)
	return context.WithValue(ctx, ownedKey{}, false)
}

func Owns(ctx context.Context) bool {
	owned, _ := ctx.Value(ownedKey{}).(bool)
	return owned
}

func From(ctx context.Context) Storage {
	return ctx.Value(ctxKey{}).(Storage)
}

func TryFrom(ctx context.Context) (Storage, bool) {
	stg, ok := ctx.Value(ctxKey{}).(Storage)
	return stg, ok
}
