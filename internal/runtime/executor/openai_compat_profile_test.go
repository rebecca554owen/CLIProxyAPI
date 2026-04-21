package executor

import (
	"net/http"
	"testing"
	"time"
)

func TestNormalizeOpenAICompatStatus_PaymentLikeMessage(t *testing.T) {
	t.Parallel()

	tests := []string{
		"insufficient balance",
		"账户余额不足",
		"余额不足，请充值后重试",
	}

	for _, message := range tests {
		if got := normalizeOpenAICompatStatus(http.StatusBadRequest, message); got != http.StatusPaymentRequired {
			t.Fatalf("normalizeOpenAICompatStatus(%q) = %d, want %d", message, got, http.StatusPaymentRequired)
		}
	}
}

func TestNormalizeOpenAICompatStatus_QuotaLikeMessage(t *testing.T) {
	t.Parallel()

	if got := normalizeOpenAICompatStatus(http.StatusBadRequest, "insufficient_quota"); got != http.StatusTooManyRequests {
		t.Fatalf("normalizeOpenAICompatStatus(quota) = %d, want %d", got, http.StatusTooManyRequests)
	}
}

func TestNewOpenAICompatStatusErr_ParsesRetryAfter(t *testing.T) {
	t.Parallel()

	headers := http.Header{"Retry-After": {"12"}}
	err := newOpenAICompatStatusErr(openAICompatProfileForKind("kimi"), nil, "kimi-k2.5", http.StatusTooManyRequests, headers, "application/json", []byte(`{"error":{"message":"rate limit"}}`))

	if err.StatusCode() != http.StatusTooManyRequests {
		t.Fatalf("StatusCode() = %d, want %d", err.StatusCode(), http.StatusTooManyRequests)
	}
	retryAfter := err.RetryAfter()
	if retryAfter == nil {
		t.Fatal("RetryAfter() = nil, want non-nil")
	}
	if *retryAfter != 12*time.Second {
		t.Fatalf("RetryAfter() = %v, want %v", *retryAfter, 12*time.Second)
	}
}
