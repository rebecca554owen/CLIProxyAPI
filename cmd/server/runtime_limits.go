package main

import (
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	defaultGoMemoryLimit = "3500MiB"
	defaultGoGCPercent   = 60
)

// configureRuntimeMemoryDefaults keeps busy proxy processes from exhausting small VPS hosts.
// Explicit GOMEMLIMIT/GOGC settings always win; these defaults only apply when unset.
func configureRuntimeMemoryDefaults() {
	if _, ok := os.LookupEnv("GOMEMLIMIT"); !ok {
		rawLimit := strings.TrimSpace(os.Getenv("CLIPROXY_DEFAULT_GOMEMLIMIT"))
		if rawLimit == "" {
			rawLimit = defaultGoMemoryLimit
		}
		limit, err := parseMemoryLimitBytes(rawLimit)
		if err != nil {
			log.WithError(err).Warn("failed to apply default Go memory limit")
		} else {
			previous := debug.SetMemoryLimit(limit)
			log.WithFields(log.Fields{
				"limit":    rawLimit,
				"bytes":    limit,
				"previous": previous,
			}).Info("default Go memory limit applied")
		}
	}

	if _, ok := os.LookupEnv("GOGC"); !ok {
		rawPercent := strings.TrimSpace(os.Getenv("CLIPROXY_DEFAULT_GOGC"))
		percent := defaultGoGCPercent
		if rawPercent != "" {
			parsed, err := strconv.Atoi(rawPercent)
			if err != nil || parsed < 0 {
				log.WithField("value", rawPercent).Warn("invalid default Go GC percent, using built-in default")
			} else {
				percent = parsed
			}
		}
		previous := debug.SetGCPercent(percent)
		log.WithFields(log.Fields{
			"percent":  percent,
			"previous": previous,
		}).Info("default Go GC percent applied")
	}
}

func parseMemoryLimitBytes(value string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("empty memory limit")
	}
	lower := strings.ToLower(trimmed)
	multipliers := []struct {
		suffix     string
		multiplier int64
	}{
		{"gib", 1 << 30},
		{"gb", 1000 * 1000 * 1000},
		{"mib", 1 << 20},
		{"mb", 1000 * 1000},
		{"kib", 1 << 10},
		{"kb", 1000},
		{"b", 1},
	}

	multiplier := int64(1)
	numberPart := trimmed
	for _, candidate := range multipliers {
		if strings.HasSuffix(lower, candidate.suffix) {
			multiplier = candidate.multiplier
			numberPart = strings.TrimSpace(trimmed[:len(trimmed)-len(candidate.suffix)])
			break
		}
	}
	if numberPart == "" {
		return 0, fmt.Errorf("missing memory limit value in %q", value)
	}

	parsed, err := strconv.ParseFloat(numberPart, 64)
	if err != nil {
		return 0, fmt.Errorf("parse memory limit %q: %w", value, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("memory limit must be positive: %q", value)
	}
	bytesFloat := parsed * float64(multiplier)
	if bytesFloat > float64(math.MaxInt64) {
		return 0, fmt.Errorf("memory limit overflows int64: %q", value)
	}
	return int64(bytesFloat), nil
}
