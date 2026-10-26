package service

import "context"

type TransactionManager interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}
