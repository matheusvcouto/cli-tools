package notionexport

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"time"
)

// UserAgent identifies the exporter without personal data.
const UserAgent = "notion-export (cli-tools)"

const maxRedirects = 10

// NewHTTPClient returns the client shared by the Notion API adapter and asset
// transfers. It honors the standard proxy environment, bounds connection and
// header waits, and only follows a limited number of http(s) redirects. There
// is no total timeout: large downloads are bounded by stall detection instead.
func NewHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = 30 * time.Second
	transport.ResponseHeaderTimeout = 60 * time.Second
	transport.MaxIdleConnsPerHost = 16
	return &http.Client{Transport: transport, CheckRedirect: checkRedirect}
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("mais de %d redirecionamentos", maxRedirects)
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return fmt.Errorf("redirecionamento para esquema não permitido: %s", req.URL.Scheme)
	}
	return nil
}

// RetryPolicy describes bounded retries of transient HTTP failures. The zero
// value disables retries; Sleep is injectable so tests do not wait.
type RetryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	// RateLimitDelay is the wait after HTTP 429: retrying inside Notion's
	// cooldown window cannot succeed.
	RateLimitDelay time.Duration
	Sleep          func(context.Context, time.Duration) error
}

// DefaultRetryPolicy matches notion-client: 3 retries, full-jitter exponential
// backoff from 1s up to 60s, and a 60s cooldown after 429.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxRetries: 3, BaseDelay: time.Second, MaxDelay: time.Minute, RateLimitDelay: time.Minute, Sleep: SleepContext}
}

// SleepContext waits for d or until ctx is done.
func SleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// RetryableStatus reports transient statuses. 4xx such as 400/403/404 mean a
// private or missing page and fail fast.
func RetryableStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// StatusError is a non-success HTTP response.
type StatusError struct {
	StatusCode int
	RetryAfter time.Duration
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("HTTP %d %s", e.StatusCode, http.StatusText(e.StatusCode))
}

// NewStatusError captures the status and a bounded Retry-After hint.
func NewStatusError(resp *http.Response) *StatusError {
	return &StatusError{StatusCode: resp.StatusCode, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
}

const maxRetryAfter = 2 * time.Minute

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.ParseFloat(v, 64); err == nil && secs >= 0 {
		return min(time.Duration(secs*float64(time.Second)), maxRetryAfter)
	}
	if t, err := http.ParseTime(v); err == nil {
		return min(max(0, time.Until(t)), maxRetryAfter)
	}
	return 0
}

// Do runs op until it succeeds, fails permanently or retries are exhausted.
// op reports whether a failure is transient.
func (p RetryPolicy) Do(ctx context.Context, op func(attempt int) (retry bool, err error)) error {
	for attempt := 0; ; attempt++ {
		retry, err := op(attempt)
		if err == nil {
			return nil
		}
		if !retry || attempt >= p.MaxRetries || ctx.Err() != nil {
			if ctxErr := ctx.Err(); ctxErr != nil && !errors.Is(err, ctxErr) {
				return errors.Join(ctxErr, err)
			}
			return err
		}
		sleep := p.Sleep
		if sleep == nil {
			sleep = SleepContext
		}
		if serr := sleep(ctx, p.delay(attempt, err)); serr != nil {
			return errors.Join(serr, err)
		}
	}
}

func (p RetryPolicy) delay(attempt int, err error) time.Duration {
	var status *StatusError
	suggested := time.Duration(0)
	if errors.As(err, &status) {
		suggested = status.RetryAfter
		if status.StatusCode == http.StatusTooManyRequests {
			return max(suggested, p.RateLimitDelay+time.Duration(rand.Int64N(int64(p.RateLimitDelay/4)+1)))
		}
	}
	if suggested > 0 {
		return suggested
	}
	window := p.BaseDelay << min(attempt, 16)
	if p.MaxDelay > 0 {
		window = min(window, p.MaxDelay)
	}
	if window <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(window) + 1))
}

// transientNetworkError reports failures worth retrying: timeouts, resets and
// other transport errors, but never cancellation of the caller's context.
func transientNetworkError(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return false
	}
	return true
}
