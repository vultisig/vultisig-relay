package contexthelper

import "context"

// CheckCancellation checks if the context is cancelled.
// If the context is cancelled, it returns ErrContextCancelled.
func CheckCancellation(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
