package main

import (
	"fmt"
	"sort"
)

type Reconciler struct {
	ledger *Ledger
}

func NewReconciler(ledger *Ledger) Reconciler {
	return Reconciler{ledger: ledger}
}

func (r Reconciler) Reconcile() ReconciliationReport {
	differences := make([]Difference, 0)
	pools := make(map[string]*GlobalPool)
	exactCount := 0
	microCount := 0
	correctionCount := 0
	blockedCount := 0
	var microTotal Amount
	var correctionTotal Amount
	for _, account := range r.ledger.AccountsInOrder() {
		for _, assetID := range account.AssetIDs() {
			asset := r.ledger.MustAsset(assetID)
			policy := r.ledger.Policy.AssetPolicy(asset)
			expected := getAmount(account.Expected, assetID)
			executed := getAmount(account.Executed, assetID)
			drift := expected - executed
			abs := absAmount(drift)
			difference := Difference{
				Account:  account.ID,
				Owner:    account.Owner,
				Asset:    assetID,
				Expected: expected,
				Executed: executed,
				Drift:    drift,
				AbsDrift: abs,
				Group:    account.AdjustmentGroup,
				Weight:   turnoverWeight(*account, assetID, r.ledger.Policy),
			}
			switch {
			case drift == 0:
				difference.Kind = DifferenceExact
				difference.Reason = "matched"
				exactCount++
			case account.HasFlag("manual-review") || account.HasFlag("frozen"):
				difference.Kind = DifferenceBlocked
				difference.Reason = "manual-review"
				blockedCount++
			case policy.Enabled && abs <= policy.MicroTolerance:
				difference.Kind = DifferenceMicro
				difference.Reason = "global-adjustment"
				microCount++
				microTotal += abs
				r.addPool(pools, assetID, account.AdjustmentGroup, drift)
			default:
				difference.Kind = DifferenceCorrection
				difference.Reason = "direct-correction"
				correctionCount++
				correctionTotal += abs
			}
			differences = append(differences, difference)
		}
	}
	globalPools := mapToPools(pools)
	for _, pool := range globalPools {
		r.ledger.Emit("pool", "", pool.Asset, pool.Amount, "global adjustment pool recorded", map[string]string{
			"group":   pool.Group,
			"entries": fmt.Sprintf("%d", pool.Entries),
		})
	}
	return ReconciliationReport{
		Differences:      differences,
		GlobalPools:      globalPools,
		MicroTotal:       microTotal,
		CorrectionTotal:  correctionTotal,
		ExactCount:       exactCount,
		MicroCount:       microCount,
		CorrectionCount:  correctionCount,
		BlockedCount:     blockedCount,
		BalancedByAsset:  r.ledger.AssetBalances(),
	}
}

func (r Reconciler) addPool(pools map[string]*GlobalPool, asset string, group string, drift Amount) {
	if group == "" {
		group = "primary"
	}
	key := asset + "::" + group
	pool, ok := pools[key]
	if !ok {
		pool = &GlobalPool{
			Asset: asset,
			Group: group,
		}
		pools[key] = pool
	}
	pool.Amount += drift
	pool.AbsAmount += absAmount(drift)
	pool.Entries++
	if drift > 0 {
		pool.PositiveEntries++
	}
	if drift < 0 {
		pool.NegativeEntries++
	}
	pool.MaxSingleDrift = maxAmount(pool.MaxSingleDrift, absAmount(drift))
}

func mapToPools(pools map[string]*GlobalPool) []GlobalPool {
	out := make([]GlobalPool, 0, len(pools))
	for _, pool := range pools {
		out = append(out, *pool)
	}
	sort.SliceStable(out, func(i int, j int) bool {
		if out[i].Asset == out[j].Asset {
			return out[i].Group < out[j].Group
		}
		return out[i].Asset < out[j].Asset
	})
	return out
}

func FilterDifferences(report ReconciliationReport, kind DifferenceKind) []Difference {
	out := make([]Difference, 0)
	for _, difference := range report.Differences {
		if difference.Kind == kind {
			out = append(out, difference)
		}
	}
	return out
}

func DifferencesByAsset(report ReconciliationReport) map[string][]Difference {
	out := make(map[string][]Difference)
	for _, difference := range report.Differences {
		out[difference.Asset] = append(out[difference.Asset], difference)
	}
	for asset := range out {
		sort.SliceStable(out[asset], func(i int, j int) bool {
			return out[asset][i].Account < out[asset][j].Account
		})
	}
	return out
}

func DifferenceVolume(report ReconciliationReport, kind DifferenceKind) Amount {
	var total Amount
	for _, difference := range report.Differences {
		if difference.Kind == kind {
			total += difference.AbsDrift
		}
	}
	return total
}

func PoolByKey(report ReconciliationReport, asset string, group string) (GlobalPool, bool) {
	for _, pool := range report.GlobalPools {
		if pool.Asset == asset && pool.Group == group {
			return pool, true
		}
	}
	return GlobalPool{}, false
}

func ReconciliationBalanced(report ReconciliationReport) bool {
	for _, balance := range report.BalancedByAsset {
		if !balance.Balanced {
			return false
		}
	}
	return true
}

func ReconciliationSummary(report ReconciliationReport) map[string]int {
	return map[string]int{
		"exact":      report.ExactCount,
		"micro":      report.MicroCount,
		"correction": report.CorrectionCount,
		"blocked":    report.BlockedCount,
		"pools":      len(report.GlobalPools),
	}
}
