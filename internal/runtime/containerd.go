package runtime

import (
	"context"
	"fmt"
)

// New returns an error for now; containerd support is not implemented.
func New(ctx context.Context, socket string) (interface{}, error) {
	return nil, fmt.Errorf("containerd backend not implemented yet")
}
