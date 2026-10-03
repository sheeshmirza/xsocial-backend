package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// TransactionFunc is the callback executed inside an ACID transaction session.
type TransactionFunc func(sessCtx context.Context) (any, error)

// WithTransaction runs the given function inside a MongoDB ACID transaction.
// If the MongoDB deployment does not support transactions (e.g. single standalone
// node without replica set in development), it gracefully falls back to non-transactional
// execution so development and local tests run without error.
// On transient transaction commit errors, it performs automatic retries up to 3 times.
func WithTransaction(ctx context.Context, client *mongo.Client, fn TransactionFunc) (any, error) {
	if client == nil {
		return fn(ctx)
	}

	session, err := client.StartSession()
	if err != nil {
		log.Printf("[DatabaseTx] Could not start session (%v); falling back to direct context", err)
		return fn(ctx)
	}
	defer session.EndSession(ctx)

	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		result, err := session.WithTransaction(ctx, func(sessCtx context.Context) (any, error) {
			return fn(sessCtx)
		})

		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if the deployment simply does not support transactions (standalone instance)
		errMsg := err.Error()
		var cmdErr mongo.CommandError
		if errors.As(err, &cmdErr) ||
			containsAny(errMsg, "replica set", "standalone", "Transaction numbers are only allowed on a replica set member or mongos") {
			log.Printf("[DatabaseTx] MongoDB transactions unsupported on this node; executing non-transactionally")
			return fn(ctx)
		}

		// Check for transient transaction errors to retry
		if containsAny(errMsg, "TransientTransactionError", "UnknownTransactionCommitResult") && attempt < maxRetries {
			log.Printf("[DatabaseTx] Transient transaction error on attempt %d: %v. Retrying...", attempt, err)
			time.Sleep(time.Duration(attempt*50) * time.Millisecond)
			continue
		}

		break
	}

	return nil, fmt.Errorf("transaction failed: %w", lastErr)
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if sub != "" && (s == sub || len(s) >= len(sub) && (contains(s, sub))) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && (len(s) > 0 && indexOf(s, substr) >= 0)))
}

func indexOf(s, substr string) int {
	n := len(substr)
	if n == 0 {
		return 0
	}
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}
