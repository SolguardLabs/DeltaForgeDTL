package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func ensureAmountMap(in map[string]Amount) map[string]Amount {
	if in == nil {
		return make(map[string]Amount)
	}
	return in
}

func cloneAmountMap(in map[string]Amount) map[string]Amount {
	out := make(map[string]Amount, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func amountKeys(maps ...map[string]Amount) []string {
	set := make(map[string]bool)
	for _, values := range maps {
		for key := range values {
			set[key] = true
		}
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func accountIDs(accounts []Account) []string {
	ids := make([]string, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	sort.Strings(ids)
	return ids
}

func absAmount(value Amount) Amount {
	if value < 0 {
		return -value
	}
	return value
}

func minAmount(a Amount, b Amount) Amount {
	if a < b {
		return a
	}
	return b
}

func maxAmount(a Amount, b Amount) Amount {
	if a > b {
		return a
	}
	return b
}

func signAmount(value Amount) Amount {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}

func addAmount(target map[string]Amount, asset string, amount Amount) {
	if target == nil {
		return
	}
	target[asset] = target[asset] + amount
}

func getAmount(values map[string]Amount, asset string) Amount {
	if values == nil {
		return 0
	}
	return values[asset]
}

func sumAmountMap(values map[string]Amount) Amount {
	var total Amount
	for _, value := range values {
		total += value
	}
	return total
}

func nonZeroAmountMap(values map[string]Amount) map[string]Amount {
	out := make(map[string]Amount)
	for key, value := range values {
		if value != 0 {
			out[key] = value
		}
	}
	return out
}

func amountMapEqual(a map[string]Amount, b map[string]Amount) bool {
	keys := amountKeys(a, b)
	for _, key := range keys {
		if a[key] != b[key] {
			return false
		}
	}
	return true
}

func formatAmount(value Amount, scale int64) string {
	if scale <= 0 {
		scale = DefaultScale
	}
	sign := ""
	raw := int64(value)
	if raw < 0 {
		sign = "-"
		raw = -raw
	}
	whole := raw / scale
	fraction := raw % scale
	width := len(strconv.FormatInt(scale-1, 10))
	text := fmt.Sprintf("%s%d.%0*d", sign, whole, width, fraction)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "" || text == "-" {
		return "0"
	}
	return text
}

func parseAmount(text string, scale int64) (Amount, error) {
	if scale <= 0 {
		scale = DefaultScale
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0, fmt.Errorf("empty amount")
	}
	sign := int64(1)
	if strings.HasPrefix(trimmed, "-") {
		sign = -1
		trimmed = strings.TrimPrefix(trimmed, "-")
	}
	parts := strings.Split(trimmed, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid amount %q", text)
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid whole amount %q: %w", text, err)
	}
	fraction := int64(0)
	if len(parts) == 2 {
		width := len(strconv.FormatInt(scale-1, 10))
		value := parts[1]
		if len(value) > width {
			value = value[:width]
		}
		for len(value) < width {
			value += "0"
		}
		fraction, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid fractional amount %q: %w", text, err)
		}
	}
	return Amount(sign * (whole*scale + fraction)), nil
}

func checkedAdd(a Amount, b Amount) (Amount, bool) {
	out := a + b
	if (b > 0 && out < a) || (b < 0 && out > a) {
		return 0, false
	}
	return out, true
}

func checkedSub(a Amount, b Amount) (Amount, bool) {
	out := a - b
	if (b < 0 && out < a) || (b > 0 && out > a) {
		return 0, false
	}
	return out, true
}

func clampAmount(value Amount, low Amount, high Amount) Amount {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func bpsOf(value Amount, bps int64) Amount {
	if bps <= 0 || value == 0 {
		return 0
	}
	sign := signAmount(value)
	abs := absAmount(value)
	return sign * Amount((int64(abs)*bps)/10_000)
}

func ratioBps(numerator Amount, denominator Amount) int64 {
	if denominator == 0 {
		return 0
	}
	sign := int64(1)
	if numerator < 0 {
		sign *= -1
		numerator = -numerator
	}
	if denominator < 0 {
		sign *= -1
		denominator = -denominator
	}
	return sign * int64(numerator) * 10_000 / int64(denominator)
}

func safeWeight(value int64) int64 {
	if value <= 0 {
		return 1
	}
	return value
}

func turnoverWeight(account Account, asset string, policy Policy) int64 {
	base := safeWeight(account.SettlementWeight)
	turnover := absAmount(getAmount(account.Turnover, asset))
	locked := absAmount(getAmount(account.Locked, asset))
	switch strings.ToLower(policy.WeightingMode) {
	case "flat":
		return base
	case "liquidity":
		return base + int64(locked/1000) + account.LiquidityTier*2
	case "risk-adjusted":
		risk := account.RiskTier
		if risk < 1 {
			risk = 1
		}
		return maxInt64(1, base+int64(turnover/1000)+account.LiquidityTier*2-risk)
	default:
		return base + int64(turnover/1000) + account.LiquidityTier
	}
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func splitProRata(total Amount, candidates []weightedAccount) ([]shareLine, Amount) {
	if total == 0 || len(candidates) == 0 {
		return nil, total
	}
	weightTotal := int64(0)
	for _, candidate := range candidates {
		weightTotal += safeWeight(candidate.Weight)
	}
	if weightTotal == 0 {
		return nil, total
	}
	sign := signAmount(total)
	absTotal := absAmount(total)
	lines := make([]shareLine, 0, len(candidates))
	var distributed Amount
	for index, candidate := range candidates {
		weight := safeWeight(candidate.Weight)
		raw := int64(absTotal) * weight
		share := Amount(raw / weightTotal)
		remainder := raw % weightTotal
		lines = append(lines, shareLine{
			ID:        candidate.Account.ID,
			Weight:    weight,
			Share:     share * sign,
			Remainder: remainder,
			Index:     index,
		})
		distributed += share * sign
	}
	left := total - distributed
	sort.SliceStable(lines, func(i int, j int) bool {
		if lines[i].Remainder == lines[j].Remainder {
			return lines[i].ID < lines[j].ID
		}
		return lines[i].Remainder > lines[j].Remainder
	})
	step := signAmount(left)
	for left != 0 && len(lines) > 0 {
		for i := range lines {
			if left == 0 {
				break
			}
			lines[i].Share += step
			left -= step
		}
	}
	sort.SliceStable(lines, func(i int, j int) bool {
		return lines[i].Index < lines[j].Index
	})
	var check Amount
	for _, line := range lines {
		check += line.Share
	}
	return lines, total - check
}

func mergeAmountMaps(base map[string]Amount, overlay map[string]Amount) map[string]Amount {
	out := cloneAmountMap(base)
	for key, value := range overlay {
		out[key] = value
	}
	return out
}

func sortedStringSet(values []string) []string {
	set := make(map[string]bool)
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			set[value] = true
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(value, needle) {
			return true
		}
	}
	return false
}

func stableJoin(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		clean = append(clean, strings.ReplaceAll(part, "|", "/"))
	}
	return strings.Join(clean, "|")
}
