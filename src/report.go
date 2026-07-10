package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

type Engine struct{}

func NewEngine() Engine {
	return Engine{}
}

func (e Engine) Run(fixture Fixture, options RunOptions) (Report, error) {
	normalized, err := NormalizeFixture(fixture)
	if err != nil {
		return Report{}, err
	}
	ledger, err := NewLedger(normalized)
	if err != nil {
		return Report{}, err
	}
	if err := ValidateLedgerReady(ledger); err != nil {
		return Report{}, err
	}
	ledger.Emit("policy", "", "", 0, "policy resolved", PolicySummary(ledger.Policy))
	reconciler := NewReconciler(ledger)
	reconciliation := reconciler.Reconcile()
	planner := NewCorrectionPlanner(ledger)
	planned := planner.Plan(reconciliation)
	corrections, err := planner.Apply(planned)
	if err != nil {
		return Report{}, err
	}
	settlementEngine := NewSettlementEngine(ledger)
	settlement, err := settlementEngine.Settle(reconciliation)
	if err != nil {
		return Report{}, err
	}
	reconciliation.BalancedByAsset = ledger.AssetBalances()
	snapshot := NewSnapshotBuilder(ledger).Build()
	assets := BuildAssetReports(ledger, reconciliation, corrections, settlement)
	metrics := BuildMetrics(ledger, reconciliation, corrections, settlement)
	report := Report{
		Name:           ledger.Name,
		Description:    ledger.Description,
		Epoch:          ledger.Epoch,
		Book:           ledger.Book,
		Source:         options.Source,
		Assets:         assets,
		Accounts:       ledger.ReportAccounts(),
		Snapshot:       snapshot,
		Reconciliation: reconciliation,
		Corrections:    corrections,
		Settlement:     settlement,
		Metrics:        metrics,
	}
	if options.IncludeEvents {
		report.Events = ledger.Events
	}
	return report, nil
}

func BuildAssetReports(ledger *Ledger, reconciliation ReconciliationReport, corrections []Correction, settlement SettlementReport) []AssetReport {
	expected := ledger.Totals("expected")
	executed := ledger.Totals("executed")
	final := ledger.Totals("final")
	correctionNet := CorrectionNetByAsset(corrections)
	allocationNet := AllocationNetByAsset(settlement.Allocations)
	liquidationNet := LiquidationNetByAsset(settlement.Liquidations)
	microCount := make(map[string]int)
	correctionCount := make(map[string]int)
	for _, difference := range reconciliation.Differences {
		switch difference.Kind {
		case DifferenceMicro:
			microCount[difference.Asset]++
		case DifferenceCorrection:
			correctionCount[difference.Asset]++
		}
	}
	out := make([]AssetReport, 0, len(ledger.AssetOrder))
	for _, assetID := range ledger.AssetOrder {
		asset := ledger.MustAsset(assetID)
		out = append(out, AssetReport{
			ID:              asset.ID,
			Symbol:          asset.Symbol,
			ExpectedTotal:   expected[assetID],
			ExecutedTotal:   executed[assetID],
			FinalTotal:      final[assetID],
			GlobalNet:       allocationNet[assetID],
			CorrectionNet:   correctionNet[assetID],
			LiquidationNet:  liquidationNet[assetID],
			Drift:           expected[assetID] - executed[assetID],
			AccountCount:    ledger.AccountCountForAsset(assetID),
			MicroCount:      microCount[assetID],
			CorrectionCount: correctionCount[assetID],
		})
	}
	return out
}

func BuildMetrics(ledger *Ledger, reconciliation ReconciliationReport, corrections []Correction, settlement SettlementReport) Metrics {
	maxDrift := Amount(0)
	riskWeighted := Amount(0)
	touched := make(map[string]bool)
	for _, difference := range reconciliation.Differences {
		maxDrift = maxAmount(maxDrift, difference.AbsDrift)
		account, _ := ledger.Account(difference.Account)
		risk := int64(1)
		if account != nil && account.RiskTier > 0 {
			risk = account.RiskTier
		}
		riskWeighted += difference.AbsDrift * Amount(risk)
		if difference.Kind != DifferenceExact {
			touched[difference.Account] = true
		}
	}
	for _, allocation := range settlement.Allocations {
		touched[allocation.Account] = true
	}
	for _, correction := range corrections {
		touched[correction.Account] = true
	}
	return Metrics{
		TotalExpected:      ledger.Totals("expected"),
		TotalExecuted:      ledger.Totals("executed"),
		TotalFinal:         ledger.Totals("final"),
		NetDrift:           netDriftByAsset(ledger),
		CorrectionVolume:   CorrectionVolume(corrections),
		GlobalVolume:       absAmount(settlement.AllocationTotal),
		LiquidationVolume:  settlement.LiquidationTotal,
		MaxAccountDrift:    maxDrift,
		RiskWeightedDrift:  riskWeighted,
		ReceiverHerfindahl: receiverHerfindahl(settlement.Allocations),
		AccountsTouched:    len(touched),
	}
}

func netDriftByAsset(ledger *Ledger) map[string]Amount {
	out := make(map[string]Amount)
	expected := ledger.Totals("expected")
	executed := ledger.Totals("executed")
	for _, asset := range ledger.AssetOrder {
		out[asset] = expected[asset] - executed[asset]
	}
	return out
}

func receiverHerfindahl(allocations []Allocation) int64 {
	if len(allocations) == 0 {
		return 0
	}
	byAccount := AllocationNetByAccount(allocations)
	var total Amount
	for _, value := range byAccount {
		total += absAmount(value)
	}
	if total == 0 {
		return 0
	}
	var index int64
	for _, value := range byAccount {
		share := int64(absAmount(value)) * 10_000 / int64(total)
		index += share * share
	}
	return index
}

func WriteReportJSON(writer io.Writer, report Report) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func WriteCompactJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	return encoder.Encode(value)
}

func DecodeFixture(reader io.Reader) (Fixture, error) {
	var fixture Fixture
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		return Fixture{}, err
	}
	return fixture, nil
}

func EncodeFixture(writer io.Writer, fixture Fixture) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(fixture)
}

func ReportDigest(report Report) string {
	return fmt.Sprintf("%s:%d:%s:%s", report.Name, report.Epoch, report.Book, report.Snapshot.Hash)
}

func ReportAsset(report Report, id string) (AssetReport, bool) {
	for _, asset := range report.Assets {
		if asset.ID == id {
			return asset, true
		}
	}
	return AssetReport{}, false
}

func ReportAccount(report Report, id string) (AccountReport, bool) {
	for _, account := range report.Accounts {
		if account.ID == id {
			return account, true
		}
	}
	return AccountReport{}, false
}

func ReportBalance(account AccountReport, asset string) (BalanceLine, bool) {
	for _, balance := range account.Balances {
		if balance.Asset == asset {
			return balance, true
		}
	}
	return BalanceLine{}, false
}

func SortReport(report *Report) {
	sort.SliceStable(report.Assets, func(i int, j int) bool {
		return report.Assets[i].ID < report.Assets[j].ID
	})
	sort.SliceStable(report.Accounts, func(i int, j int) bool {
		return report.Accounts[i].ID < report.Accounts[j].ID
	})
	sort.SliceStable(report.Corrections, func(i int, j int) bool {
		return report.Corrections[i].ID < report.Corrections[j].ID
	})
	SortAllocations(report.Settlement.Allocations)
}

func ValidateReport(report Report) error {
	if report.Name == "" {
		return fmt.Errorf("report name is required")
	}
	if err := VerifySnapshotShape(report.Snapshot); err != nil {
		return err
	}
	for _, asset := range report.Assets {
		if asset.ID == "" {
			return fmt.Errorf("asset report missing id")
		}
	}
	for _, account := range report.Accounts {
		if account.ID == "" {
			return fmt.Errorf("account report missing id")
		}
	}
	return nil
}
