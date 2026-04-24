package gmail

import (
	"errors"
	"fmt"
	"net"

	"google.golang.org/api/googleapi"
)

var (
	ErrUnauthorized = errors.New("gmail unauthorized")
	ErrForbidden    = errors.New("gmail forbidden")
	ErrNotFound     = errors.New("gmail not found")
	ErrHistoryGone  = errors.New("gmail history not found")
	ErrRateLimited  = errors.New("gmail rate limited")
	ErrUnavailable  = errors.New("gmail unavailable")
)

func normalizeError(operation string, err error) error {
	if err == nil {
		return nil
	}

	var googleErr *googleapi.Error
	if errors.As(err, &googleErr) {
		return fmt.Errorf("%s: %w: %w", operation, classifyGoogleError(googleErr), err)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%s: %w: %w", operation, ErrUnavailable, err)
	}

	return fmt.Errorf("%s: %w", operation, err)
}

func classifyGoogleError(err *googleapi.Error) error {
	switch err.Code {
	case 401:
		return ErrUnauthorized
	case 403:
		for _, item := range err.Errors {
			switch item.Reason {
			case "rateLimitExceeded", "userRateLimitExceeded", "quotaExceeded":
				return ErrRateLimited
			}
		}
		return ErrForbidden
	case 404:
		for _, item := range err.Errors {
			if item.Reason == "historyNotFound" {
				return errors.Join(ErrHistoryGone, ErrNotFound)
			}
		}
		return ErrNotFound
	case 429:
		return ErrRateLimited
	case 408, 500, 502, 503, 504:
		return ErrUnavailable
	default:
		if err.Code >= 500 {
			return ErrUnavailable
		}
		return ErrForbidden
	}
}
