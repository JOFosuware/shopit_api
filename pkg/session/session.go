package session

import (
	"context"
)

type AppSession interface {
	Put(ctx context.Context, key string, val interface{})
	Get(ctx context.Context, key string) interface{}
	Destroy(ctx context.Context) error
	RenewToken(ctx context.Context) error
}
