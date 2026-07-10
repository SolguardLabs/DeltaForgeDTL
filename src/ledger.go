package main

import (
	"fmt"
	"sort"
	"strings"
)

type Ledger struct {
	Name         string
	Description  string
	Epoch        int64
	Book         string
	Labels       map[string]string
	Assets       map[string]Asset
	AssetOrder   []string
	Accounts     map[string]*Account
	AccountOrder []string
	Policy       Policy
	Events       []Event
}

func NewLedger(fixture Fixture) (*Ledger, error) {
	fixture.Policy = fixture.Policy.WithDefaults()
	if fixture.Name == "" {
		fixture.Name = "cycle"
	}
	if fixture.Book == "" {
		fixture.Book = "default"
	}
	if fixture.Epoch == 0 {
		fixture.Epoch = 1
	}
	ledger := &Ledger{
		Name:        fixture.Name,
		Description: fixture.Description,
		Epoch:       fixture.Epoch,
		Book:        fixture.Book,
		Labels:      copyStringMap(fixture.Labels),
		Assets:      make(map[string]Asset),
		Accounts:    make(map[string]*Account),
		Policy:      fixture.Policy,
	}
	for _, rawAsset := range fixture.Assets {
		asset := rawAsset.WithDefaults()
		if strings.TrimSpace(asset.ID) == "" {
			return nil, fmt.Errorf("asset id is required")
		}
		if _, exists := ledger.Assets[asset.ID]; exists {
			return nil, fmt.Errorf("duplicate asset %s", asset.ID)
		}
		ledger.Assets[asset.ID] = asset
		ledger.AssetOrder = append(ledger.AssetOrder, asset.ID)
	}
	if len(ledger.Assets) == 0 {
		defaultAsset := Asset{ID: "USD", Symbol: "USD", Name: "Settlement Dollar"}.WithDefaults()
		ledger.Assets[defaultAsset.ID] = defaultAsset
		ledger.AssetOrder = append(ledger.AssetOrder, defaultAsset.ID)
	}
	sort.Strings(ledger.AssetOrder)
	for _, rawAccount := range fixture.Accounts {
		account := rawAccount.WithDefaults()
		if strings.TrimSpace(account.ID) == "" {
			return nil, fmt.Errorf("account id is required")
		}
		if _, exists := ledger.Accounts[account.ID]; exists {
			return nil, fmt.Errorf("duplicate account %s", account.ID)
		}
		if ledger.Policy.IsFrozenKind(account.Kind) {
			account.Flags = sortedStringSet(append(account.Flags, "frozen"))
		}
		ledger.ensureKnownAssets(account)
		copy := account
		ledger.Accounts[account.ID] = &copy
		ledger.AccountOrder = append(ledger.AccountOrder, account.ID)
	}
	sort.Strings(ledger.AccountOrder)
	if len(ledger.Accounts) == 0 {
		return nil, fmt.Errorf("at least one account is required")
	}
	ledger.Emit("init", "", "", 0, "ledger initialized", map[string]string{
		"name":     ledger.Name,
		"accounts": fmt.Sprintf("%d", len(ledger.Accounts)),
		"assets":   fmt.Sprintf("%d", len(ledger.Assets)),
	})
	return ledger, nil
}

func (l *Ledger) ensureKnownAssets(account Account) {
	for _, assetID := range account.AssetIDs() {
		if _, exists := l.Assets[assetID]; exists {
			continue
		}
		asset := Asset{ID: assetID, Symbol: strings.ToUpper(assetID), Name: assetID}.WithDefaults()
		l.Assets[assetID] = asset
		l.AssetOrder = append(l.AssetOrder, assetID)
		sort.Strings(l.AssetOrder)
	}
}

func (l *Ledger) Asset(id string) (Asset, bool) {
	asset, ok := l.Assets[id]
	return asset, ok
}

func (l *Ledger) MustAsset(id string) Asset {
	if asset, ok := l.Asset(id); ok {
		return asset
	}
	return Asset{ID: id, Symbol: strings.ToUpper(id), Name: id}.WithDefaults()
}

func (l *Ledger) Account(id string) (*Account, bool) {
	account, ok := l.Accounts[id]
	return account, ok
}

func (l *Ledger) AccountsInOrder() []*Account {
	out := make([]*Account, 0, len(l.AccountOrder))
	for _, id := range l.AccountOrder {
		if account, ok := l.Accounts[id]; ok {
			out = append(out, account)
		}
	}
	return out
}

func (l *Ledger) AssetsInOrder() []Asset {
	out := make([]Asset, 0, len(l.AssetOrder))
	for _, id := range l.AssetOrder {
		out = append(out, l.Assets[id])
	}
	return out
}

func (l *Ledger) Emit(kind string, account string, asset string, amount Amount, message string, metadata map[string]string) {
	l.Events = append(l.Events, Event{
		Type:     kind,
		Epoch:    l.Epoch,
		Account:  account,
		Asset:    asset,
		Amount:   amount,
		Message:  message,
		Metadata: copyStringMap(metadata),
	})
}

func (l *Ledger) Expected(account string, asset string) Amount {
	if record, ok := l.Account(account); ok {
		return getAmount(record.Expected, asset)
	}
	return 0
}

func (l *Ledger) Executed(account string, asset string) Amount {
	if record, ok := l.Account(account); ok {
		return getAmount(record.Executed, asset)
	}
	return 0
}

func (l *Ledger) Final(account string, asset string) Amount {
	if record, ok := l.Account(account); ok {
		return getAmount(record.Final, asset)
	}
	return 0
}

func (l *Ledger) SetFinal(account string, asset string, value Amount) error {
	record, ok := l.Account(account)
	if !ok {
		return fmt.Errorf("unknown account %s", account)
	}
	if _, ok := l.Assets[asset]; !ok {
		return fmt.Errorf("unknown asset %s", asset)
	}
	record.Final[asset] = value
	return nil
}

func (l *Ledger) ApplyFinal(account string, asset string, delta Amount) (Amount, Amount, error) {
	record, ok := l.Account(account)
	if !ok {
		return 0, 0, fmt.Errorf("unknown account %s", account)
	}
	if _, ok := l.Assets[asset]; !ok {
		return 0, 0, fmt.Errorf("unknown asset %s", asset)
	}
	before := getAmount(record.Final, asset)
	after, ok := checkedAdd(before, delta)
	if !ok {
		return before, before, fmt.Errorf("amount overflow for %s/%s", account, asset)
	}
	if after < 0 && !l.Policy.AllowNegativeFinal {
		return before, before, fmt.Errorf("negative final balance for %s/%s", account, asset)
	}
	record.Final[asset] = after
	return before, after, nil
}

func (l *Ledger) ApplyExpected(account string, asset string, delta Amount) (Amount, Amount, error) {
	record, ok := l.Account(account)
	if !ok {
		return 0, 0, fmt.Errorf("unknown account %s", account)
	}
	before := getAmount(record.Expected, asset)
	after, ok := checkedAdd(before, delta)
	if !ok {
		return before, before, fmt.Errorf("amount overflow for expected %s/%s", account, asset)
	}
	record.Expected[asset] = after
	return before, after, nil
}

func (l *Ledger) ApplyExecuted(account string, asset string, delta Amount) (Amount, Amount, error) {
	record, ok := l.Account(account)
	if !ok {
		return 0, 0, fmt.Errorf("unknown account %s", account)
	}
	before := getAmount(record.Executed, asset)
	after, ok := checkedAdd(before, delta)
	if !ok {
		return before, before, fmt.Errorf("amount overflow for executed %s/%s", account, asset)
	}
	record.Executed[asset] = after
	record.Final[asset] = after
	return before, after, nil
}

func (l *Ledger) Totals(selector string) map[string]Amount {
	totals := make(map[string]Amount)
	for _, account := range l.AccountsInOrder() {
		var source map[string]Amount
		switch selector {
		case "expected":
			source = account.Expected
		case "executed":
			source = account.Executed
		case "locked":
			source = account.Locked
		default:
			source = account.Final
		}
		for _, asset := range l.AssetOrder {
			totals[asset] += getAmount(source, asset)
		}
	}
	for _, asset := range l.AssetOrder {
		if _, ok := totals[asset]; !ok {
			totals[asset] = 0
		}
	}
	return totals
}

func (l *Ledger) TotalFor(selector string, asset string) Amount {
	return l.Totals(selector)[asset]
}

func (l *Ledger) AccountCountForAsset(asset string) int {
	count := 0
	for _, account := range l.AccountsInOrder() {
		if _, ok := account.Expected[asset]; ok {
			count++
			continue
		}
		if _, ok := account.Executed[asset]; ok {
			count++
			continue
		}
		if _, ok := account.Final[asset]; ok {
			count++
		}
	}
	return count
}

func (l *Ledger) ReceiverCandidates(asset string, group string) []weightedAccount {
	candidates := make([]weightedAccount, 0)
	for _, account := range l.AccountsInOrder() {
		if !l.Policy.IncludeInactive && !account.IsActive() {
			continue
		}
		if !account.CanReceiveGlobal() {
			continue
		}
		if group != "" && account.AdjustmentGroup != "" && account.AdjustmentGroup != group {
			continue
		}
		weight := turnoverWeight(*account, asset, l.Policy)
		if weight < l.Policy.MinSettlementWeight {
			continue
		}
		candidates = append(candidates, weightedAccount{
			Account: *account,
			Asset:   asset,
			Group:   group,
			Weight:  weight,
		})
	}
	sort.SliceStable(candidates, func(i int, j int) bool {
		if candidates[i].Weight == candidates[j].Weight {
			return candidates[i].Account.ID < candidates[j].Account.ID
		}
		return candidates[i].Weight > candidates[j].Weight
	})
	if l.Policy.MaxGlobalReceivers > 0 && len(candidates) > l.Policy.MaxGlobalReceivers {
		candidates = candidates[:l.Policy.MaxGlobalReceivers]
	}
	return candidates
}

func (l *Ledger) ReportAccounts() []AccountReport {
	reports := make([]AccountReport, 0, len(l.AccountOrder))
	for _, account := range l.AccountsInOrder() {
		lines := make([]BalanceLine, 0)
		for _, asset := range amountKeys(account.Expected, account.Executed, account.Final, account.Locked) {
			expected := getAmount(account.Expected, asset)
			executed := getAmount(account.Executed, asset)
			final := getAmount(account.Final, asset)
			lines = append(lines, BalanceLine{
				Asset:    asset,
				Expected: expected,
				Executed: executed,
				Final:    final,
				Drift:    expected - executed,
				Delta:    final - executed,
				Locked:   getAmount(account.Locked, asset),
			})
		}
		reports = append(reports, AccountReport{
			ID:               account.ID,
			Owner:            account.Owner,
			Kind:             account.Kind,
			Status:           account.Status,
			AdjustmentGroup:  account.AdjustmentGroup,
			SettlementWeight: account.SettlementWeight,
			RiskTier:         account.RiskTier,
			LiquidityTier:    account.LiquidityTier,
			Balances:         lines,
			Flags:            append([]string{}, account.Flags...),
		})
	}
	return reports
}

func (l *Ledger) AssetBalances() []AssetBalance {
	out := make([]AssetBalance, 0, len(l.AssetOrder))
	expected := l.Totals("expected")
	executed := l.Totals("executed")
	final := l.Totals("final")
	for _, asset := range l.AssetOrder {
		observed := expected[asset] - executed[asset]
		settled := final[asset] - executed[asset]
		out = append(out, AssetBalance{
			Asset:         asset,
			ExpectedTotal: expected[asset],
			ExecutedTotal: executed[asset],
			FinalTotal:    final[asset],
			ObservedDrift: observed,
			SettledDrift:  settled,
			Balanced:      observed == settled,
		})
	}
	return out
}

func (l *Ledger) CloneFixture() Fixture {
	assets := make([]Asset, 0, len(l.AssetOrder))
	for _, id := range l.AssetOrder {
		assets = append(assets, l.Assets[id])
	}
	accounts := make([]Account, 0, len(l.AccountOrder))
	for _, account := range l.AccountsInOrder() {
		copy := *account
		copy.Expected = cloneAmountMap(account.Expected)
		copy.Executed = cloneAmountMap(account.Executed)
		copy.Final = cloneAmountMap(account.Final)
		copy.Turnover = cloneAmountMap(account.Turnover)
		copy.Locked = cloneAmountMap(account.Locked)
		copy.Flags = append([]string{}, account.Flags...)
		copy.Metadata = copyStringMap(account.Metadata)
		accounts = append(accounts, copy)
	}
	return Fixture{
		Name:        l.Name,
		Description: l.Description,
		Epoch:       l.Epoch,
		Book:        l.Book,
		Assets:      assets,
		Accounts:    accounts,
		Policy:      l.Policy,
		Labels:      copyStringMap(l.Labels),
	}
}

func copyStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (l *Ledger) AuditString() string {
	parts := make([]string, 0)
	parts = append(parts, stableJoin("ledger", l.Name, fmt.Sprintf("%d", l.Epoch), l.Book))
	for _, asset := range l.AssetOrder {
		parts = append(parts, stableJoin("asset", asset, fmt.Sprintf("%d", l.TotalFor("final", asset))))
	}
	for _, account := range l.AccountsInOrder() {
		for _, asset := range amountKeys(account.Expected, account.Executed, account.Final) {
			parts = append(parts, stableJoin(
				"account",
				account.ID,
				asset,
				fmt.Sprintf("%d", getAmount(account.Expected, asset)),
				fmt.Sprintf("%d", getAmount(account.Executed, asset)),
				fmt.Sprintf("%d", getAmount(account.Final, asset)),
			))
		}
	}
	return strings.Join(parts, "\n")
}
