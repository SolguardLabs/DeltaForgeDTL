package main

import (
	"fmt"
	"sort"
)

type SettlementEngine struct {
	ledger *Ledger
}

func NewSettlementEngine(ledger *Ledger) SettlementEngine {
	return SettlementEngine{ledger: ledger}
}

func (s SettlementEngine) Settle(report ReconciliationReport) (SettlementReport, error) {
	allocations := make([]Allocation, 0)
	var allocationTotal Amount
	var roundingRemainder Amount
	sequence := 1
	corrected := correctedGlobalReceivers(report)
	for _, pool := range report.GlobalPools {
		if pool.Amount == 0 {
			continue
		}
		asset := s.ledger.MustAsset(pool.Asset)
		assetPolicy := s.ledger.Policy.AssetPolicy(asset)
		if !assetPolicy.Enabled {
			roundingRemainder += pool.Amount
			continue
		}
		candidates := filterCorrectedReceivers(s.ledger.ReceiverCandidates(pool.Asset, pool.Group), pool, corrected)
		shares, remainder := splitProRata(pool.Amount, candidates)
		roundingRemainder += remainder
		for index, share := range shares {
			if share.Share == 0 {
				continue
			}
			candidate := candidates[index]
			before, after, err := s.ledger.ApplyFinal(candidate.Account.ID, pool.Asset, share.Share)
			if err != nil {
				roundingRemainder += share.Share
				s.ledger.Emit("allocation_skipped", candidate.Account.ID, pool.Asset, share.Share, err.Error(), map[string]string{
					"pool": pool.Key(),
				})
				continue
			}
			allocation := Allocation{
				ID:       fmt.Sprintf("alloc-%06d", sequence),
				Account:  candidate.Account.ID,
				Asset:    pool.Asset,
				Group:    pool.Group,
				Amount:   share.Share,
				Weight:   share.Weight,
				Before:   before,
				After:    after,
				Pool:     pool.Key(),
				Sequence: sequence,
			}
			allocations = append(allocations, allocation)
			allocationTotal += share.Share
			sequence++
			s.ledger.Emit("allocation", allocation.Account, allocation.Asset, allocation.Amount, "global allocation applied", map[string]string{
				"id":    allocation.ID,
				"group": allocation.Group,
			})
		}
	}
	liquidations := s.LiquidateFloors()
	var liquidationTotal Amount
	for _, liquidation := range liquidations {
		liquidationTotal += liquidation.Settled
	}
	return SettlementReport{
		Allocations:       allocations,
		Liquidations:      liquidations,
		AllocationTotal:   allocationTotal,
		LiquidationTotal:  liquidationTotal,
		ReceiverCount:     countAllocationReceivers(allocations),
		PoolCount:         len(report.GlobalPools),
		RoundingRemainder: roundingRemainder,
		Completed:         roundingRemainder == 0,
	}, nil
}

func correctedGlobalReceivers(report ReconciliationReport) map[string]bool {
	out := make(map[string]bool)
	for _, difference := range report.Differences {
		if difference.Kind != DifferenceCorrection {
			continue
		}
		out[receiverKey(difference.Account, difference.Asset, difference.Group)] = true
	}
	return out
}

func filterCorrectedReceivers(candidates []weightedAccount, pool GlobalPool, corrected map[string]bool) []weightedAccount {
	if len(corrected) == 0 {
		return candidates
	}
	filtered := make([]weightedAccount, 0, len(candidates))
	for _, candidate := range candidates {
		if corrected[receiverKey(candidate.Account.ID, pool.Asset, pool.Group)] {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func receiverKey(account string, asset string, group string) string {
	if group == "" {
		group = "primary"
	}
	return account + "::" + asset + "::" + group
}

func (s SettlementEngine) LiquidateFloors() []Liquidation {
	liquidations := make([]Liquidation, 0)
	sequence := 1
	for _, account := range s.ledger.AccountsInOrder() {
		if account.HasFlag("no-liquidation") {
			continue
		}
		for _, assetID := range s.ledger.AssetOrder {
			asset := s.ledger.MustAsset(assetID)
			policy := s.ledger.Policy.AssetPolicy(asset)
			floor := policy.SettlementFloor
			current := getAmount(account.Final, assetID)
			if current >= floor {
				continue
			}
			shortfall := floor - current
			haircut := bpsOf(shortfall, policy.LiquidationHaircutBps)
			settled := shortfall - haircut
			before, after, err := s.ledger.ApplyFinal(account.ID, assetID, settled)
			if err != nil {
				continue
			}
			liquidation := Liquidation{
				ID:         fmt.Sprintf("liq-%06d", sequence),
				Account:    account.ID,
				Asset:      assetID,
				Starting:   before,
				Settled:    settled,
				Haircut:    haircut,
				Final:      after,
				Reason:     "settlement-floor",
				HaircutBps: policy.LiquidationHaircutBps,
				Sequence:   sequence,
			}
			liquidations = append(liquidations, liquidation)
			sequence++
			s.ledger.Emit("liquidation", account.ID, assetID, settled, "settlement floor liquidated", map[string]string{
				"id": liquidation.ID,
			})
		}
	}
	return liquidations
}

func countAllocationReceivers(allocations []Allocation) int {
	set := make(map[string]bool)
	for _, allocation := range allocations {
		set[allocation.Account] = true
	}
	return len(set)
}

func AllocationNetByAsset(allocations []Allocation) map[string]Amount {
	out := make(map[string]Amount)
	for _, allocation := range allocations {
		out[allocation.Asset] += allocation.Amount
	}
	return out
}

func AllocationNetByAccount(allocations []Allocation) map[string]Amount {
	out := make(map[string]Amount)
	for _, allocation := range allocations {
		out[allocation.Account] += allocation.Amount
	}
	return out
}

func LiquidationNetByAsset(liquidations []Liquidation) map[string]Amount {
	out := make(map[string]Amount)
	for _, liquidation := range liquidations {
		out[liquidation.Asset] += liquidation.Settled
	}
	return out
}

func SortAllocations(allocations []Allocation) {
	sort.SliceStable(allocations, func(i int, j int) bool {
		if allocations[i].Asset == allocations[j].Asset {
			if allocations[i].Group == allocations[j].Group {
				return allocations[i].Account < allocations[j].Account
			}
			return allocations[i].Group < allocations[j].Group
		}
		return allocations[i].Asset < allocations[j].Asset
	})
}

func LargestAllocation(allocations []Allocation) Allocation {
	var largest Allocation
	for _, allocation := range allocations {
		if absAmount(allocation.Amount) > absAmount(largest.Amount) {
			largest = allocation
		}
	}
	return largest
}

func SettlementComplete(report SettlementReport) bool {
	return report.Completed && report.RoundingRemainder == 0
}

func SettlementSummary(report SettlementReport) map[string]int {
	return map[string]int{
		"allocations":  len(report.Allocations),
		"liquidations": len(report.Liquidations),
		"receivers":    report.ReceiverCount,
		"pools":        report.PoolCount,
	}
}
