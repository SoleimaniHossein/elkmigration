package utils

import (
	"context"
	"elkmigration/logger"
	"encoding/json"
	"errors"
	"go.uber.org/zap"
	"strconv"
	"time"
)

// IntToString converts an int to a string
func IntToString(i int) string {
	return strconv.Itoa(i)
}

// StringToInt converts a string to an int, returns 0 and error if conversion fails
func StringToInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// StringToIntOrDefault converts a string to an int with a default fallback value if conversion fails
func StringToIntOrDefault(s string, defaultValue int) int {
	num, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return num
}
func ToStruct[T any](input any) (T, error) {
	var result T

	// Marshal the input to JSON
	jsonData, err := json.Marshal(input)
	if err != nil {
		return result, err
	}

	// Unmarshal JSON into the target struct
	err = json.Unmarshal(jsonData, &result)
	return result, err
}

// Retry retries a function up to a maximum number of attempts with exponential backoff.
// Returns an error if all attempts fail.
func Retry(ctx context.Context, attempts int, backoff time.Duration, operation func() error) error {
	if attempts <= 0 {
		return errors.New("attempts must be greater than zero")
	}

	var err error
	for i := 1; i <= attempts; i++ {
		// Check if the context has been canceled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Execute the operation
		err = operation()
		if err == nil {
			return nil
		}

		// Log the retry attempt
		if i < attempts {
			wait := backoff * (1 << (i - 1)) // Exponential backoff: 2^(i-1) * base backoff
			select {
			case <-time.After(wait): // Wait before retrying
			case <-ctx.Done(): // Exit if context is canceled
				return ctx.Err()
			}
		}
	}

	// Return the error from the last attempt
	return err
}

func RetryWithResult[T any](
	ctx context.Context,
	attempts int,
	backoff time.Duration,
	operation func() (T, error),
) (T, error) {
	var zero T
	if attempts <= 0 {
		logger.Warn("Invalid retry attempts, must be greater than zero")
		return zero, nil
	}

	var result T
	var err error
	for i := 1; i <= attempts; i++ {
		// Check if the context has been canceled
		if ctx.Err() != nil {
			logger.Error("Operation canceled by context", zap.Error(ctx.Err()))
			return zero, ctx.Err()
		}

		// Execute the operation
		result, err = operation()
		if err == nil {
			return result, nil
		}

		// Log the retry attempt
		logger.Warn("Operation failed, retrying...", zap.Int("attempt", i), zap.Error(err))

		// Wait before retrying if more attempts are left
		if i < attempts {
			wait := backoff * (1 << (i - 1)) // Exponential backoff
			logger.Warn("Waiting before next retry", zap.Duration("backoff", wait))
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				logger.Error("Retry halted due to context cancellation", zap.Error(ctx.Err()))
				return zero, ctx.Err()
			}
		}
	}

	// Return the last error after all retries
	logger.Error("Operation failed after all retry attempts", zap.Int("attempts", attempts), zap.Error(err))
	return zero, err
}
