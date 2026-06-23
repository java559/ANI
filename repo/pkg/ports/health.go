package ports

import "context"

type HealthChecker interface {
	Health(ctx context.Context) error
}
