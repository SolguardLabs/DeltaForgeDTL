package main

import (
	"fmt"
	"sort"
)

type CorrectionPlanner struct {
	ledger *Ledger
}

func NewCorrectionPlanner(ledger *Ledger) CorrectionPlanner {
	return CorrectionPlanner{ledger: ledger}
}

func (p CorrectionPlanner) Plan(report ReconciliationReport) []Correction {
	corrections := make([]Correction, 0)
	sequence := 1
	for _, difference := range report.Differences {
		if difference.Kind != DifferenceCorrection {
			continue
		}
		amount := difference.Drift
		status := CorrectionPlanned
		if absAmount(amount) > p.ledger.Policy.MaxCorrection {
			amount = signAmount(amount) * p.ledger.Policy.MaxCorrection
			status = CorrectionCapped
		}
		correction := Correction{
			ID:      fmt.Sprintf("corr-%06d", sequence),
			Account: difference.Account,
			Asset:   difference.Asset,
			Amount:  amount,
			Before:  p.ledger.Final(difference.Account, difference.Asset),
			Reason:  difference.Reason,
			Status:  status,
			Epoch:   p.ledger.Epoch,
			Trace:   stableJoin(difference.Account, difference.Asset, fmt.Sprintf("%d", difference.Drift)),
		}
		correction.After = correction.Before + amount
		corrections = append(corrections, correction)
		sequence++
	}
	sort.SliceStable(corrections, func(i int, j int) bool {
		if corrections[i].Asset == corrections[j].Asset {
			return corrections[i].Account < corrections[j].Account
		}
		return corrections[i].Asset < corrections[j].Asset
	})
	for index := range corrections {
		corrections[index].ID = fmt.Sprintf("corr-%06d", index+1)
	}
	return corrections
}

func (p CorrectionPlanner) Apply(corrections []Correction) ([]Correction, error) {
	applied := make([]Correction, 0, len(corrections))
	for _, correction := range corrections {
		if correction.Status == CorrectionSkipped {
			applied = append(applied, correction)
			continue
		}
		before, after, err := p.ledger.ApplyFinal(correction.Account, correction.Asset, correction.Amount)
		if err != nil {
			correction.Status = CorrectionSkipped
			correction.Reason = err.Error()
			applied = append(applied, correction)
			continue
		}
		correction.Before = before
		correction.After = after
		if correction.Status != CorrectionCapped {
			correction.Status = CorrectionApplied
		}
		p.ledger.Emit("correction", correction.Account, correction.Asset, correction.Amount, "direct correction applied", map[string]string{
			"id":     correction.ID,
			"status": string(correction.Status),
		})
		applied = append(applied, correction)
	}
	return applied, nil
}

func CorrectionVolume(corrections []Correction) Amount {
	var total Amount
	for _, correction := range corrections {
		total += absAmount(correction.Amount)
	}
	return total
}

func CorrectionNetByAsset(corrections []Correction) map[string]Amount {
	out := make(map[string]Amount)
	for _, correction := range corrections {
		if correction.Status == CorrectionApplied || correction.Status == CorrectionCapped {
			out[correction.Asset] += correction.Amount
		}
	}
	return out
}

func CorrectionsByAccount(corrections []Correction) map[string][]Correction {
	out := make(map[string][]Correction)
	for _, correction := range corrections {
		out[correction.Account] = append(out[correction.Account], correction)
	}
	for account := range out {
		sort.SliceStable(out[account], func(i int, j int) bool {
			if out[account][i].Asset == out[account][j].Asset {
				return out[account][i].ID < out[account][j].ID
			}
			return out[account][i].Asset < out[account][j].Asset
		})
	}
	return out
}

func CorrectionStatusCounts(corrections []Correction) map[CorrectionStatus]int {
	out := make(map[CorrectionStatus]int)
	for _, correction := range corrections {
		out[correction.Status]++
	}
	return out
}

func AnySkippedCorrection(corrections []Correction) bool {
	for _, correction := range corrections {
		if correction.Status == CorrectionSkipped {
			return true
		}
	}
	return false
}

func LargestCorrection(corrections []Correction) Correction {
	var largest Correction
	for _, correction := range corrections {
		if absAmount(correction.Amount) > absAmount(largest.Amount) {
			largest = correction
		}
	}
	return largest
}
