package main

import (
	"fmt"
	"sort"
	"strings"
)

type InvariantCheck struct {
	ID       string `json:"id"`
	Scope    string `json:"scope"`
	Account  string `json:"account,omitempty"`
	Asset    string `json:"asset,omitempty"`
	Passed   bool   `json:"passed"`
	Expected Amount `json:"expected,omitempty"`
	Actual   Amount `json:"actual,omitempty"`
	Message  string `json:"message"`
}

type ExposureLine struct {
	Account          string `json:"account"`
	Owner            string `json:"owner"`
	Asset            string `json:"asset"`
	Expected         Amount `json:"expected"`
	Executed         Amount `json:"executed"`
	Final            Amount `json:"final"`
	ObservedDrift    Amount `json:"observedDrift"`
	SettlementDelta  Amount `json:"settlementDelta"`
	Turnover         Amount `json:"turnover"`
	Weight           int64  `json:"weight"`
	RiskTier         int64  `json:"riskTier"`
	LiquidityTier    int64  `json:"liquidityTier"`
	ConcentrationBps int64  `json:"concentrationBps"`
}

type DriftBucket struct {
	Asset       string   `json:"asset"`
	Group       string   `json:"group"`
	Count       int      `json:"count"`
	Net         Amount   `json:"net"`
	Abs         Amount   `json:"abs"`
	Largest     Amount   `json:"largest"`
	Accounts    []string `json:"accounts"`
	Positive    int      `json:"positive"`
	Negative    int      `json:"negative"`
	Zero        int      `json:"zero"`
	AverageAbs  Amount   `json:"averageAbs"`
}

type AuditProfile struct {
	Name             string           `json:"name"`
	Epoch            int64            `json:"epoch"`
	Checks           []InvariantCheck `json:"checks"`
	Exposures        []ExposureLine   `json:"exposures"`
	Buckets          []DriftBucket    `json:"buckets"`
	Passed           bool             `json:"passed"`
	FailedCount      int              `json:"failedCount"`
	ReviewCount      int              `json:"reviewCount"`
	MaxExposure       Amount           `json:"maxExposure"`
	MaxSettlementMove Amount           `json:"maxSettlementMove"`
}

func BuildAuditProfile(ledger *Ledger, reconciliation ReconciliationReport, settlement SettlementReport) AuditProfile {
	checks := BuildInvariantChecks(ledger, reconciliation, settlement)
	exposures := BuildExposureLines(ledger)
	buckets := BuildDriftBuckets(reconciliation)
	failed := 0
	for _, check := range checks {
		if !check.Passed {
			failed++
		}
	}
	maxExposure := Amount(0)
	maxMove := Amount(0)
	for _, exposure := range exposures {
		maxExposure = maxAmount(maxExposure, absAmount(exposure.Final))
		maxMove = maxAmount(maxMove, absAmount(exposure.SettlementDelta))
	}
	return AuditProfile{
		Name:             ledger.Name,
		Epoch:            ledger.Epoch,
		Checks:           checks,
		Exposures:        exposures,
		Buckets:          buckets,
		Passed:           failed == 0,
		FailedCount:      failed,
		ReviewCount:      len(checks),
		MaxExposure:      maxExposure,
		MaxSettlementMove: maxMove,
	}
}

func BuildInvariantChecks(ledger *Ledger, reconciliation ReconciliationReport, settlement SettlementReport) []InvariantCheck {
	checks := make([]InvariantCheck, 0)
	checks = append(checks, checkAssetTotals(ledger)...)
	checks = append(checks, checkAccountShapes(ledger)...)
	checks = append(checks, checkCorrectionCoverage(ledger, reconciliation)...)
	checks = append(checks, checkSettlementCoverage(reconciliation, settlement)...)
	checks = append(checks, checkReceiverEligibility(ledger, settlement)...)
	sort.SliceStable(checks, func(i int, j int) bool {
		if checks[i].Scope == checks[j].Scope {
			return checks[i].ID < checks[j].ID
		}
		return checks[i].Scope < checks[j].Scope
	})
	return checks
}

func checkAssetTotals(ledger *Ledger) []InvariantCheck {
	checks := make([]InvariantCheck, 0)
	expected := ledger.Totals("expected")
	executed := ledger.Totals("executed")
	final := ledger.Totals("final")
	for _, asset := range ledger.AssetOrder {
		checks = append(checks, InvariantCheck{
			ID:       "asset-final-nonnegative-" + asset,
			Scope:    "asset",
			Asset:    asset,
			Passed:   final[asset] >= 0 || ledger.Policy.AllowNegativeFinal,
			Expected: 0,
			Actual:   final[asset],
			Message:  "final asset balance is inside configured domain",
		})
		checks = append(checks, InvariantCheck{
			ID:       "asset-drift-materialized-" + asset,
			Scope:    "asset",
			Asset:    asset,
			Passed:   (expected[asset]-executed[asset]) == (final[asset]-executed[asset]),
			Expected: expected[asset] - executed[asset],
			Actual:   final[asset] - executed[asset],
			Message:  "asset drift is reflected in final totals",
		})
	}
	return checks
}

func checkAccountShapes(ledger *Ledger) []InvariantCheck {
	checks := make([]InvariantCheck, 0)
	for _, account := range ledger.AccountsInOrder() {
		passedStatus := account.Status != ""
		checks = append(checks, InvariantCheck{
			ID:      "account-status-" + account.ID,
			Scope:   "account",
			Account: account.ID,
			Passed:  passedStatus,
			Message: "account status is present",
		})
		for _, asset := range account.AssetIDs() {
			final := getAmount(account.Final, asset)
			checks = append(checks, InvariantCheck{
				ID:       "account-final-domain-" + account.ID + "-" + asset,
				Scope:    "account",
				Account:  account.ID,
				Asset:    asset,
				Passed:   final >= 0 || ledger.Policy.AllowNegativeFinal,
				Expected: 0,
				Actual:   final,
				Message:  "account final balance is inside configured domain",
			})
		}
	}
	return checks
}

func checkCorrectionCoverage(ledger *Ledger, reconciliation ReconciliationReport) []InvariantCheck {
	checks := make([]InvariantCheck, 0)
	for _, difference := range reconciliation.Differences {
		if difference.Kind != DifferenceCorrection {
			continue
		}
		account, ok := ledger.Account(difference.Account)
		if !ok {
			checks = append(checks, InvariantCheck{
				ID:      "correction-account-" + difference.Account + "-" + difference.Asset,
				Scope:   "correction",
				Account: difference.Account,
				Asset:   difference.Asset,
				Passed:  false,
				Message: "correction references a known account",
			})
			continue
		}
		final := getAmount(account.Final, difference.Asset)
		expected := getAmount(account.Expected, difference.Asset)
		checks = append(checks, InvariantCheck{
			ID:       "correction-final-" + difference.Account + "-" + difference.Asset,
			Scope:    "correction",
			Account:  difference.Account,
			Asset:    difference.Asset,
			Passed:   final == expected || absAmount(final-expected) <= ledger.Policy.MaxCorrection,
			Expected: expected,
			Actual:   final,
			Message:  "direct correction leaves account inside correction envelope",
		})
	}
	return checks
}

func checkSettlementCoverage(reconciliation ReconciliationReport, settlement SettlementReport) []InvariantCheck {
	checks := make([]InvariantCheck, 0)
	poolNet := make(map[string]Amount)
	for _, pool := range reconciliation.GlobalPools {
		poolNet[pool.Asset] += pool.Amount
	}
	allocationNet := AllocationNetByAsset(settlement.Allocations)
	for asset, amount := range poolNet {
		checks = append(checks, InvariantCheck{
			ID:       "settlement-pool-" + asset,
			Scope:    "settlement",
			Asset:    asset,
			Passed:   allocationNet[asset]+settlement.RoundingRemainder == amount || allocationNet[asset] == amount,
			Expected: amount,
			Actual:   allocationNet[asset],
			Message:  "global pool is accounted for by allocations and remainder",
		})
	}
	if len(poolNet) == 0 {
		checks = append(checks, InvariantCheck{
			ID:      "settlement-empty-pools",
			Scope:   "settlement",
			Passed:  len(settlement.Allocations) == 0,
			Message: "empty pool set has no allocations",
		})
	}
	return checks
}

func checkReceiverEligibility(ledger *Ledger, settlement SettlementReport) []InvariantCheck {
	checks := make([]InvariantCheck, 0, len(settlement.Allocations))
	for _, allocation := range settlement.Allocations {
		account, ok := ledger.Account(allocation.Account)
		passed := ok && account.CanReceiveGlobal()
		checks = append(checks, InvariantCheck{
			ID:      "receiver-eligible-" + allocation.ID,
			Scope:   "receiver",
			Account: allocation.Account,
			Asset:   allocation.Asset,
			Passed:  passed,
			Message: "allocation receiver is eligible under account controls",
		})
	}
	return checks
}

func BuildExposureLines(ledger *Ledger) []ExposureLine {
	lines := make([]ExposureLine, 0)
	finalTotals := ledger.Totals("final")
	for _, account := range ledger.AccountsInOrder() {
		for _, asset := range account.AssetIDs() {
			final := getAmount(account.Final, asset)
			concentration := int64(0)
			if finalTotals[asset] != 0 {
				concentration = int64(absAmount(final)) * 10_000 / int64(absAmount(finalTotals[asset]))
			}
			lines = append(lines, ExposureLine{
				Account:          account.ID,
				Owner:            account.Owner,
				Asset:            asset,
				Expected:         getAmount(account.Expected, asset),
				Executed:         getAmount(account.Executed, asset),
				Final:            final,
				ObservedDrift:    getAmount(account.Expected, asset) - getAmount(account.Executed, asset),
				SettlementDelta:  final - getAmount(account.Executed, asset),
				Turnover:         getAmount(account.Turnover, asset),
				Weight:           turnoverWeight(*account, asset, ledger.Policy),
				RiskTier:         account.RiskTier,
				LiquidityTier:    account.LiquidityTier,
				ConcentrationBps: concentration,
			})
		}
	}
	sort.SliceStable(lines, func(i int, j int) bool {
		if lines[i].Asset == lines[j].Asset {
			return lines[i].Account < lines[j].Account
		}
		return lines[i].Asset < lines[j].Asset
	})
	return lines
}

func BuildDriftBuckets(reconciliation ReconciliationReport) []DriftBucket {
	buckets := make(map[string]*DriftBucket)
	accountSets := make(map[string]map[string]bool)
	for _, difference := range reconciliation.Differences {
		group := difference.Group
		if group == "" {
			group = "primary"
		}
		key := difference.Asset + "::" + group
		bucket, ok := buckets[key]
		if !ok {
			bucket = &DriftBucket{Asset: difference.Asset, Group: group}
			buckets[key] = bucket
			accountSets[key] = make(map[string]bool)
		}
		bucket.Count++
		bucket.Net += difference.Drift
		bucket.Abs += difference.AbsDrift
		bucket.Largest = maxAmount(bucket.Largest, difference.AbsDrift)
		if difference.Drift > 0 {
			bucket.Positive++
		} else if difference.Drift < 0 {
			bucket.Negative++
		} else {
			bucket.Zero++
		}
		accountSets[key][difference.Account] = true
	}
	out := make([]DriftBucket, 0, len(buckets))
	for key, bucket := range buckets {
		if bucket.Count > 0 {
			bucket.AverageAbs = bucket.Abs / Amount(bucket.Count)
		}
		for account := range accountSets[key] {
			bucket.Accounts = append(bucket.Accounts, account)
		}
		sort.Strings(bucket.Accounts)
		out = append(out, *bucket)
	}
	sort.SliceStable(out, func(i int, j int) bool {
		if out[i].Asset == out[j].Asset {
			return out[i].Group < out[j].Group
		}
		return out[i].Asset < out[j].Asset
	})
	return out
}

func AuditProfileText(profile AuditProfile) string {
	lines := make([]string, 0)
	status := "passed"
	if !profile.Passed {
		status = "review"
	}
	lines = append(lines, fmt.Sprintf("audit:%s:%d:%s", profile.Name, profile.Epoch, status))
	lines = append(lines, fmt.Sprintf("checks:%d failed:%d", profile.ReviewCount, profile.FailedCount))
	for _, bucket := range profile.Buckets {
		lines = append(lines, fmt.Sprintf(
			"bucket:%s:%s count=%d net=%d abs=%d",
			bucket.Asset,
			bucket.Group,
			bucket.Count,
			bucket.Net,
			bucket.Abs,
		))
	}
	for _, check := range profile.Checks {
		if check.Passed {
			continue
		}
		lines = append(lines, fmt.Sprintf("failed:%s:%s", check.Scope, check.ID))
	}
	return strings.Join(lines, "\n")
}

func ExposureByAccount(lines []ExposureLine) map[string][]ExposureLine {
	out := make(map[string][]ExposureLine)
	for _, line := range lines {
		out[line.Account] = append(out[line.Account], line)
	}
	for account := range out {
		sort.SliceStable(out[account], func(i int, j int) bool {
			return out[account][i].Asset < out[account][j].Asset
		})
	}
	return out
}

func ExposureByAsset(lines []ExposureLine) map[string][]ExposureLine {
	out := make(map[string][]ExposureLine)
	for _, line := range lines {
		out[line.Asset] = append(out[line.Asset], line)
	}
	for asset := range out {
		sort.SliceStable(out[asset], func(i int, j int) bool {
			if out[asset][i].ConcentrationBps == out[asset][j].ConcentrationBps {
				return out[asset][i].Account < out[asset][j].Account
			}
			return out[asset][i].ConcentrationBps > out[asset][j].ConcentrationBps
		})
	}
	return out
}

func TopExposure(lines []ExposureLine, limit int) []ExposureLine {
	copyLines := append([]ExposureLine{}, lines...)
	sort.SliceStable(copyLines, func(i int, j int) bool {
		if absAmount(copyLines[i].Final) == absAmount(copyLines[j].Final) {
			return copyLines[i].Account < copyLines[j].Account
		}
		return absAmount(copyLines[i].Final) > absAmount(copyLines[j].Final)
	})
	if limit <= 0 || limit > len(copyLines) {
		limit = len(copyLines)
	}
	return copyLines[:limit]
}
