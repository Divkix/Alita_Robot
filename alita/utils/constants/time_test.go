package constants

import (
	"testing"
	"time"
)

func TestCacheDurationValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		got      time.Duration
		expected time.Duration
	}{
		{"AdminCacheTTL", AdminCacheTTL, 30 * time.Minute},
		{"DefaultCacheTTL", DefaultCacheTTL, 5 * time.Minute},
		{"ShortCacheTTL", ShortCacheTTL, 1 * time.Minute},
		{"LongCacheTTL", LongCacheTTL, 1 * time.Hour},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.expected {
				t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.expected)
			}
		})
	}
}

func TestUpdateIntervalValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		got      time.Duration
		expected time.Duration
	}{
		{"UserUpdateInterval", UserUpdateInterval, 5 * time.Minute},
		{"ChatUpdateInterval", ChatUpdateInterval, 5 * time.Minute},
		{"ChannelUpdateInterval", ChannelUpdateInterval, 5 * time.Minute},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.expected {
				t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.expected)
			}
		})
	}
}

func TestTimeoutValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		got      time.Duration
		expected time.Duration
	}{
		{"DefaultTimeout", DefaultTimeout, 10 * time.Second},
		{"ShortTimeout", ShortTimeout, 5 * time.Second},
		{"LongTimeout", LongTimeout, 30 * time.Second},
		{"CaptchaTimeout", CaptchaTimeout, 30 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.expected {
				t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.expected)
			}
		})
	}
}

func TestRetryAndDelayValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		got      time.Duration
		expected time.Duration
	}{
		{"RetryDelay", RetryDelay, 2 * time.Second},
		{"WebhookLatency", WebhookLatency, 10 * time.Millisecond},
		{"MetricsStaleThreshold", MetricsStaleThreshold, 5 * time.Minute},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.expected {
				t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.expected)
			}
		})
	}
}

func TestAllConstantsPositive(t *testing.T) {
	t.Parallel()

	all := []struct {
		name string
		val  time.Duration
	}{
		{"AdminCacheTTL", AdminCacheTTL},
		{"DefaultCacheTTL", DefaultCacheTTL},
		{"ShortCacheTTL", ShortCacheTTL},
		{"LongCacheTTL", LongCacheTTL},
		{"UserUpdateInterval", UserUpdateInterval},
		{"ChatUpdateInterval", ChatUpdateInterval},
		{"ChannelUpdateInterval", ChannelUpdateInterval},
		{"DefaultTimeout", DefaultTimeout},
		{"ShortTimeout", ShortTimeout},
		{"LongTimeout", LongTimeout},
		{"CaptchaTimeout", CaptchaTimeout},
		{"RetryDelay", RetryDelay},
		{"WebhookLatency", WebhookLatency},
		{"MetricsStaleThreshold", MetricsStaleThreshold},
	}

	for _, c := range all {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.val <= 0 {
				t.Errorf("%s = %v, want positive duration", c.name, c.val)
			}
		})
	}
}

func TestCacheTTLOrdering(t *testing.T) {
	t.Parallel()

	if ShortCacheTTL >= DefaultCacheTTL {
		t.Errorf("ShortCacheTTL (%v) must be < DefaultCacheTTL (%v)", ShortCacheTTL, DefaultCacheTTL)
	}
	if DefaultCacheTTL >= LongCacheTTL {
		t.Errorf("DefaultCacheTTL (%v) must be < LongCacheTTL (%v)", DefaultCacheTTL, LongCacheTTL)
	}
	if ShortCacheTTL >= LongCacheTTL {
		t.Errorf("ShortCacheTTL (%v) must be < LongCacheTTL (%v)", ShortCacheTTL, LongCacheTTL)
	}
}

func TestTimeoutOrdering(t *testing.T) {
	t.Parallel()

	if ShortTimeout >= DefaultTimeout {
		t.Errorf("ShortTimeout (%v) must be < DefaultTimeout (%v)", ShortTimeout, DefaultTimeout)
	}
	if DefaultTimeout >= LongTimeout {
		t.Errorf("DefaultTimeout (%v) must be < LongTimeout (%v)", DefaultTimeout, LongTimeout)
	}
	if ShortTimeout >= LongTimeout {
		t.Errorf("ShortTimeout (%v) must be < LongTimeout (%v)", ShortTimeout, LongTimeout)
	}
}
