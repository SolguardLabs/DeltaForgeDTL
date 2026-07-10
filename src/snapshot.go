package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

type SnapshotBuilder struct {
	ledger *Ledger
}

func NewSnapshotBuilder(ledger *Ledger) SnapshotBuilder {
	return SnapshotBuilder{ledger: ledger}
}

func (b SnapshotBuilder) Build() SnapshotReport {
	expected := b.ledger.Totals("expected")
	executed := b.ledger.Totals("executed")
	final := b.ledger.Totals("final")
	locked := b.ledger.Totals("locked")
	lines := b.CanonicalLines(expected, executed, final, locked)
	hash := hashLines(lines)
	return SnapshotReport{
		Epoch:              b.ledger.Epoch,
		Book:               b.ledger.Book,
		AccountCount:       len(b.ledger.Accounts),
		AssetCount:         len(b.ledger.Assets),
		ExpectedTotals:     expected,
		ExecutedTotals:     executed,
		FinalTotals:        final,
		LockedTotals:       locked,
		Hash:               hash,
		CanonicalLineCount: len(lines),
	}
}

func (b SnapshotBuilder) CanonicalLines(expected map[string]Amount, executed map[string]Amount, final map[string]Amount, locked map[string]Amount) []string {
	lines := make([]string, 0)
	lines = append(lines, stableJoin("header", b.ledger.Name, b.ledger.Book, fmt.Sprintf("%d", b.ledger.Epoch)))
	for _, asset := range b.ledger.AssetOrder {
		record := b.ledger.MustAsset(asset)
		lines = append(lines, stableJoin(
			"asset",
			record.ID,
			record.Symbol,
			fmt.Sprintf("%d", record.Scale),
			fmt.Sprintf("%d", record.Precision),
			fmt.Sprintf("%d", expected[asset]),
			fmt.Sprintf("%d", executed[asset]),
			fmt.Sprintf("%d", final[asset]),
			fmt.Sprintf("%d", locked[asset]),
		))
	}
	for _, account := range b.ledger.AccountsInOrder() {
		lines = append(lines, stableJoin(
			"account",
			account.ID,
			account.Owner,
			account.Kind,
			account.Status,
			account.AdjustmentGroup,
			fmt.Sprintf("%d", account.SettlementWeight),
		))
		for _, asset := range amountKeys(account.Expected, account.Executed, account.Final, account.Locked, account.Turnover) {
			lines = append(lines, stableJoin(
				"balance",
				account.ID,
				asset,
				fmt.Sprintf("%d", getAmount(account.Expected, asset)),
				fmt.Sprintf("%d", getAmount(account.Executed, asset)),
				fmt.Sprintf("%d", getAmount(account.Final, asset)),
				fmt.Sprintf("%d", getAmount(account.Locked, asset)),
				fmt.Sprintf("%d", getAmount(account.Turnover, asset)),
			))
		}
	}
	sort.Strings(lines[1:])
	return lines
}

func hashLines(lines []string) string {
	hasher := sha256.New()
	for _, line := range lines {
		hasher.Write([]byte(line))
		hasher.Write([]byte{'\n'})
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func SnapshotDiff(before SnapshotReport, after SnapshotReport) []AssetBalance {
	assets := amountKeys(before.ExpectedTotals, before.ExecutedTotals, before.FinalTotals, after.FinalTotals)
	out := make([]AssetBalance, 0, len(assets))
	for _, asset := range assets {
		expected := before.ExpectedTotals[asset]
		executed := before.ExecutedTotals[asset]
		final := after.FinalTotals[asset]
		out = append(out, AssetBalance{
			Asset:         asset,
			ExpectedTotal: expected,
			ExecutedTotal: executed,
			FinalTotal:    final,
			ObservedDrift: expected - executed,
			SettledDrift:  final - executed,
			Balanced:      expected-executed == final-executed,
		})
	}
	return out
}

func SnapshotDigest(report SnapshotReport) string {
	parts := make([]string, 0)
	parts = append(parts, fmt.Sprintf("epoch:%d", report.Epoch))
	parts = append(parts, fmt.Sprintf("book:%s", report.Book))
	parts = append(parts, fmt.Sprintf("accounts:%d", report.AccountCount))
	parts = append(parts, fmt.Sprintf("assets:%d", report.AssetCount))
	for _, asset := range amountKeys(report.ExpectedTotals, report.ExecutedTotals, report.FinalTotals, report.LockedTotals) {
		parts = append(parts, stableJoin(
			"asset",
			asset,
			fmt.Sprintf("%d", report.ExpectedTotals[asset]),
			fmt.Sprintf("%d", report.ExecutedTotals[asset]),
			fmt.Sprintf("%d", report.FinalTotals[asset]),
			fmt.Sprintf("%d", report.LockedTotals[asset]),
		))
	}
	parts = append(parts, fmt.Sprintf("hash:%s", report.Hash))
	return strings.Join(parts, "\n")
}

func VerifySnapshotShape(report SnapshotReport) error {
	if report.Epoch <= 0 {
		return fmt.Errorf("snapshot epoch must be positive")
	}
	if report.Book == "" {
		return fmt.Errorf("snapshot book is required")
	}
	if report.AccountCount <= 0 {
		return fmt.Errorf("snapshot account count must be positive")
	}
	if report.AssetCount <= 0 {
		return fmt.Errorf("snapshot asset count must be positive")
	}
	if len(report.Hash) != 64 {
		return fmt.Errorf("snapshot hash must be sha256 hex")
	}
	if report.CanonicalLineCount < report.AssetCount+report.AccountCount {
		return fmt.Errorf("snapshot canonical line count is too small")
	}
	return nil
}

func SnapshotTotalsStable(a SnapshotReport, b SnapshotReport) bool {
	return amountMapEqual(a.ExpectedTotals, b.ExpectedTotals) &&
		amountMapEqual(a.ExecutedTotals, b.ExecutedTotals) &&
		amountMapEqual(a.FinalTotals, b.FinalTotals) &&
		amountMapEqual(a.LockedTotals, b.LockedTotals)
}
