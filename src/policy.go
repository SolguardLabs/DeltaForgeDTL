package main

import (
	"fmt"
	"sort"
	"strings"
)

func (p Policy) AssetPolicy(asset Asset) AssetPolicy {
	p = p.WithDefaults()
	resolved := AssetPolicy{
		Asset:                 asset.ID,
		MicroTolerance:        maxAmount(p.MicroTolerance, asset.MicroTolerance),
		GlobalTolerance:       maxAmount(p.GlobalTolerance, asset.GlobalTolerance),
		SettlementFloor:       maxAmount(p.SettlementFloor, asset.SettlementFloor),
		LiquidationHaircutBps: asset.LiquidationHaircutBps,
		Enabled:               asset.GlobalEnabled,
	}
	if !asset.GlobalEnabled && asset.ID != "" {
		resolved.Enabled = false
	}
	for _, override := range p.AssetOverrides {
		if override.Asset != asset.ID {
			continue
		}
		if override.MicroTolerance != 0 {
			resolved.MicroTolerance = override.MicroTolerance
		}
		if override.GlobalTolerance != 0 {
			resolved.GlobalTolerance = override.GlobalTolerance
		}
		if override.SettlementFloor != 0 {
			resolved.SettlementFloor = override.SettlementFloor
		}
		if override.ReceiverCap != 0 {
			resolved.ReceiverCap = override.ReceiverCap
		}
		if override.LiquidationHaircutBps != 0 {
			resolved.LiquidationHaircutBps = override.LiquidationHaircutBps
		}
		resolved.Enabled = override.Enabled
	}
	return resolved
}

func ValidateFixture(fixture Fixture) error {
	if strings.TrimSpace(fixture.Name) == "" {
		return fmt.Errorf("name is required")
	}
	seenAssets := make(map[string]bool)
	for _, asset := range fixture.Assets {
		if strings.TrimSpace(asset.ID) == "" {
			return fmt.Errorf("asset id is required")
		}
		if seenAssets[asset.ID] {
			return fmt.Errorf("duplicate asset %s", asset.ID)
		}
		seenAssets[asset.ID] = true
		if asset.Scale < 0 {
			return fmt.Errorf("asset %s has invalid scale", asset.ID)
		}
		if asset.Precision < 0 {
			return fmt.Errorf("asset %s has invalid precision", asset.ID)
		}
	}
	if len(fixture.Accounts) == 0 {
		return fmt.Errorf("accounts are required")
	}
	seenAccounts := make(map[string]bool)
	for _, account := range fixture.Accounts {
		if strings.TrimSpace(account.ID) == "" {
			return fmt.Errorf("account id is required")
		}
		if seenAccounts[account.ID] {
			return fmt.Errorf("duplicate account %s", account.ID)
		}
		seenAccounts[account.ID] = true
		if account.SettlementWeight < 0 {
			return fmt.Errorf("account %s has invalid settlement weight", account.ID)
		}
		if account.LiquidityTier < 0 || account.RiskTier < 0 {
			return fmt.Errorf("account %s has invalid tier", account.ID)
		}
		for asset, value := range account.Expected {
			if strings.TrimSpace(asset) == "" {
				return fmt.Errorf("account %s has empty expected asset", account.ID)
			}
			if value < 0 && !fixture.Policy.AllowNegativeFinal {
				return fmt.Errorf("account %s has negative expected balance", account.ID)
			}
		}
		for asset, value := range account.Executed {
			if strings.TrimSpace(asset) == "" {
				return fmt.Errorf("account %s has empty executed asset", account.ID)
			}
			if value < 0 && !fixture.Policy.AllowNegativeFinal {
				return fmt.Errorf("account %s has negative executed balance", account.ID)
			}
		}
	}
	return validatePolicy(fixture.Policy)
}

func validatePolicy(policy Policy) error {
	policy = policy.WithDefaults()
	if policy.MicroTolerance < 0 {
		return fmt.Errorf("micro tolerance cannot be negative")
	}
	if policy.GlobalTolerance < 0 {
		return fmt.Errorf("global tolerance cannot be negative")
	}
	if policy.CorrectionThreshold < 0 {
		return fmt.Errorf("correction threshold cannot be negative")
	}
	if policy.MaxCorrection < 0 {
		return fmt.Errorf("max correction cannot be negative")
	}
	if policy.MinSettlementWeight < 0 {
		return fmt.Errorf("minimum settlement weight cannot be negative")
	}
	if policy.MaxGlobalReceivers < 0 {
		return fmt.Errorf("maximum global receivers cannot be negative")
	}
	switch strings.ToLower(policy.WeightingMode) {
	case "", "turnover", "flat", "liquidity", "risk-adjusted":
	default:
		return fmt.Errorf("unsupported weighting mode %s", policy.WeightingMode)
	}
	seenOverrides := make(map[string]bool)
	for _, override := range policy.AssetOverrides {
		if strings.TrimSpace(override.Asset) == "" {
			return fmt.Errorf("asset override requires asset")
		}
		if seenOverrides[override.Asset] {
			return fmt.Errorf("duplicate asset override %s", override.Asset)
		}
		seenOverrides[override.Asset] = true
		if override.MicroTolerance < 0 || override.GlobalTolerance < 0 {
			return fmt.Errorf("asset override %s has invalid tolerance", override.Asset)
		}
		if override.LiquidationHaircutBps < 0 || override.LiquidationHaircutBps > 10_000 {
			return fmt.Errorf("asset override %s has invalid haircut", override.Asset)
		}
	}
	return nil
}

func NormalizeFixture(fixture Fixture) (Fixture, error) {
	if err := ValidateFixture(fixture); err != nil {
		return Fixture{}, err
	}
	fixture.Policy = fixture.Policy.WithDefaults()
	if fixture.Book == "" {
		fixture.Book = "default"
	}
	if fixture.Epoch == 0 {
		fixture.Epoch = 1
	}
	assets := make([]Asset, 0, len(fixture.Assets))
	for _, asset := range fixture.Assets {
		assets = append(assets, asset.WithDefaults())
	}
	sort.SliceStable(assets, func(i int, j int) bool {
		return assets[i].ID < assets[j].ID
	})
	accounts := make([]Account, 0, len(fixture.Accounts))
	for _, account := range fixture.Accounts {
		normalized := account.WithDefaults()
		normalized.Flags = sortedStringSet(normalized.Flags)
		accounts = append(accounts, normalized)
	}
	sort.SliceStable(accounts, func(i int, j int) bool {
		return accounts[i].ID < accounts[j].ID
	})
	fixture.Assets = assets
	fixture.Accounts = accounts
	return fixture, nil
}

func ValidateLedgerReady(ledger *Ledger) error {
	if ledger == nil {
		return fmt.Errorf("ledger is nil")
	}
	if len(ledger.Assets) == 0 {
		return fmt.Errorf("ledger has no assets")
	}
	if len(ledger.Accounts) == 0 {
		return fmt.Errorf("ledger has no accounts")
	}
	for _, asset := range ledger.AssetOrder {
		if _, ok := ledger.Assets[asset]; !ok {
			return fmt.Errorf("asset order contains unknown asset %s", asset)
		}
	}
	for _, accountID := range ledger.AccountOrder {
		account, ok := ledger.Accounts[accountID]
		if !ok {
			return fmt.Errorf("account order contains unknown account %s", accountID)
		}
		for _, asset := range account.AssetIDs() {
			if _, ok := ledger.Assets[asset]; !ok {
				return fmt.Errorf("account %s references unknown asset %s", accountID, asset)
			}
		}
	}
	return nil
}

func PolicySummary(policy Policy) map[string]string {
	policy = policy.WithDefaults()
	return map[string]string{
		"microTolerance":      fmt.Sprintf("%d", policy.MicroTolerance),
		"globalTolerance":     fmt.Sprintf("%d", policy.GlobalTolerance),
		"correctionThreshold": fmt.Sprintf("%d", policy.CorrectionThreshold),
		"maxCorrection":       fmt.Sprintf("%d", policy.MaxCorrection),
		"weightingMode":       policy.WeightingMode,
		"eventLevel":          policy.EventLevel,
	}
}

func AssetPolicySummary(policy Policy, asset Asset) map[string]string {
	resolved := policy.AssetPolicy(asset)
	return map[string]string{
		"asset":            resolved.Asset,
		"microTolerance":   fmt.Sprintf("%d", resolved.MicroTolerance),
		"globalTolerance":  fmt.Sprintf("%d", resolved.GlobalTolerance),
		"settlementFloor":  fmt.Sprintf("%d", resolved.SettlementFloor),
		"receiverCap":      fmt.Sprintf("%d", resolved.ReceiverCap),
		"haircutBps":       fmt.Sprintf("%d", resolved.LiquidationHaircutBps),
		"enabled":          fmt.Sprintf("%t", resolved.Enabled),
	}
}
