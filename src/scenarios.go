package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

func BuiltInFixtures() map[string]Fixture {
	return map[string]Fixture{
		"baseline":          baselineFixture(),
		"correction-window": correctionFixture(),
		"settlement-floor":  settlementFixture(),
		"multi-asset":       multiAssetFixture(),
	}
}

func ScenarioNames() []string {
	fixtures := BuiltInFixtures()
	names := make([]string, 0, len(fixtures))
	for name := range fixtures {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func LoadScenario(name string) (Fixture, error) {
	fixtures := BuiltInFixtures()
	if fixture, ok := fixtures[name]; ok {
		return fixture, nil
	}
	return Fixture{}, fmt.Errorf("unknown scenario %s", name)
}

func LoadFixtureFile(path string) (Fixture, error) {
	file, err := os.Open(path)
	if err != nil {
		return Fixture{}, err
	}
	defer file.Close()
	fixture, err := DecodeFixture(file)
	if err != nil {
		return Fixture{}, err
	}
	if fixture.Name == "" {
		fixture.Name = strings.TrimSuffix(strings.TrimSuffix(path, ".json"), ".fixture")
	}
	return fixture, nil
}

func defaultAssets() []Asset {
	return []Asset{
		{
			ID:                    "USD",
			Symbol:                "USD",
			Name:                  "DeltaForge Settlement Dollar",
			Scale:                 1_000_000,
			Precision:             6,
			DustLimit:             5,
			MicroTolerance:        12,
			GlobalTolerance:       5_000,
			SettlementFloor:       0,
			LiquidationHaircutBps: 0,
			GlobalEnabled:         true,
		},
		{
			ID:                    "EUR",
			Symbol:                "EUR",
			Name:                  "Euro Cash Lane",
			Scale:                 1_000_000,
			Precision:             6,
			DustLimit:             5,
			MicroTolerance:        10,
			GlobalTolerance:       4_000,
			SettlementFloor:       0,
			LiquidationHaircutBps: 0,
			GlobalEnabled:         true,
		},
	}
}

func defaultPolicy() Policy {
	return Policy{
		MicroTolerance:        12,
		GlobalTolerance:       5_000,
		CorrectionThreshold:   13,
		MaxCorrection:         50_000,
		MinSettlementWeight:   1,
		MaxGlobalReceivers:    8,
		SettlementFloor:       0,
		WeightingMode:         "turnover",
		AllowNegativeFinal:    false,
		IncludeInactive:       false,
		EventLevel:            "summary",
		RequireBalancedTotals: true,
		AssetOverrides: []AssetPolicy{
			{Asset: "USD", MicroTolerance: 12, GlobalTolerance: 5_000, SettlementFloor: 0, Enabled: true},
			{Asset: "EUR", MicroTolerance: 10, GlobalTolerance: 4_000, SettlementFloor: 0, Enabled: true},
		},
	}
}

func baselineFixture() Fixture {
	return Fixture{
		Name:        "baseline",
		Description: "daily settlement cycle with compacted operational adjustments",
		Epoch:       4401,
		Book:        "df-main",
		Assets:      defaultAssets(),
		Policy:      defaultPolicy(),
		Labels: map[string]string{
			"desk": "cash-recon",
			"mode": "standard",
		},
		Accounts: []Account{
			{
				ID:               "member-alpha",
				Owner:            "Alpha Markets",
				Kind:             "member",
				Status:           "active",
				AdjustmentGroup:  "primary",
				Expected:         map[string]Amount{"USD": 1_000_000, "EUR": 220_000},
				Executed:         map[string]Amount{"USD": 999_992, "EUR": 220_000},
				Turnover:         map[string]Amount{"USD": 20_000_000, "EUR": 1_250_000},
				SettlementWeight: 3,
				LiquidityTier:    2,
				RiskTier:         1,
			},
			{
				ID:               "member-beta",
				Owner:            "Beta Clearing",
				Kind:             "member",
				Status:           "active",
				AdjustmentGroup:  "primary",
				Expected:         map[string]Amount{"USD": 700_000, "EUR": 100_000},
				Executed:         map[string]Amount{"USD": 699_993, "EUR": 100_000},
				Turnover:         map[string]Amount{"USD": 18_000_000, "EUR": 950_000},
				SettlementWeight: 2,
				LiquidityTier:    1,
				RiskTier:         1,
			},
			{
				ID:               "liquidity-core",
				Owner:            "Core Liquidity",
				Kind:             "member",
				Status:           "active",
				AdjustmentGroup:  "primary",
				Expected:         map[string]Amount{"USD": 2_500_000, "EUR": 600_000},
				Executed:         map[string]Amount{"USD": 2_500_000, "EUR": 600_000},
				Turnover:         map[string]Amount{"USD": 90_000_000, "EUR": 7_000_000},
				SettlementWeight: 9,
				LiquidityTier:    4,
				RiskTier:         1,
			},
			{
				ID:               "retail-omega",
				Owner:            "Omega Retail",
				Kind:             "member",
				Status:           "active",
				AdjustmentGroup:  "primary",
				Expected:         map[string]Amount{"USD": 300_000, "EUR": 40_000},
				Executed:         map[string]Amount{"USD": 300_000, "EUR": 39_996},
				Turnover:         map[string]Amount{"USD": 600_000, "EUR": 120_000},
				SettlementWeight: 1,
				LiquidityTier:    1,
				RiskTier:         2,
			},
			{
				ID:               "treasury",
				Owner:            "DeltaForge Treasury",
				Kind:             "treasury",
				Status:           "active",
				AdjustmentGroup:  "primary",
				Expected:         map[string]Amount{"USD": 500_000, "EUR": 250_000},
				Executed:         map[string]Amount{"USD": 500_000, "EUR": 250_000},
				Turnover:         map[string]Amount{"USD": 0, "EUR": 0},
				SettlementWeight: 1,
				LiquidityTier:    1,
				RiskTier:         1,
			},
		},
	}
}

func correctionFixture() Fixture {
	policy := defaultPolicy()
	policy.MaxCorrection = 75_000
	return Fixture{
		Name:        "correction-window",
		Description: "cycle with direct correction entries above the operational window",
		Epoch:       4402,
		Book:        "df-main",
		Assets:      defaultAssets(),
		Policy:      policy,
		Labels: map[string]string{
			"desk": "cash-recon",
			"mode": "correction",
		},
		Accounts: []Account{
			{
				ID:               "member-alpha",
				Owner:            "Alpha Markets",
				Kind:             "member",
				Status:           "active",
				AdjustmentGroup:  "primary",
				Expected:         map[string]Amount{"USD": 1_400_000},
				Executed:         map[string]Amount{"USD": 1_399_180},
				Turnover:         map[string]Amount{"USD": 22_000_000},
				SettlementWeight: 4,
				LiquidityTier:    2,
				RiskTier:         1,
			},
			{
				ID:               "member-beta",
				Owner:            "Beta Clearing",
				Kind:             "member",
				Status:           "active",
				AdjustmentGroup:  "primary",
				Expected:         map[string]Amount{"USD": 900_000},
				Executed:         map[string]Amount{"USD": 899_990},
				Turnover:         map[string]Amount{"USD": 18_500_000},
				SettlementWeight: 2,
				LiquidityTier:    1,
				RiskTier:         1,
			},
			{
				ID:               "liquidity-core",
				Owner:            "Core Liquidity",
				Kind:             "member",
				Status:           "active",
				AdjustmentGroup:  "primary",
				Expected:         map[string]Amount{"USD": 1_800_000},
				Executed:         map[string]Amount{"USD": 1_800_000},
				Turnover:         map[string]Amount{"USD": 70_000_000},
				SettlementWeight: 8,
				LiquidityTier:    4,
				RiskTier:         1,
			},
		},
	}
}

func settlementFixture() Fixture {
	policy := defaultPolicy()
	policy.SettlementFloor = 100
	policy.AssetOverrides = []AssetPolicy{
		{Asset: "USD", MicroTolerance: 12, GlobalTolerance: 5_000, SettlementFloor: 100, Enabled: true},
		{Asset: "EUR", MicroTolerance: 10, GlobalTolerance: 4_000, SettlementFloor: 0, Enabled: true},
	}
	return Fixture{
		Name:        "settlement-floor",
		Description: "small final settlement cycle with account floor handling",
		Epoch:       4403,
		Book:        "df-floor",
		Assets:      defaultAssets(),
		Policy:      policy,
		Labels: map[string]string{
			"desk": "floor-recon",
			"mode": "settlement",
		},
		Accounts: []Account{
			{
				ID:               "member-floor",
				Owner:            "Floor Member",
				Kind:             "member",
				Status:           "active",
				AdjustmentGroup:  "floor",
				Expected:         map[string]Amount{"USD": 95},
				Executed:         map[string]Amount{"USD": 90},
				Turnover:         map[string]Amount{"USD": 1_000},
				SettlementWeight: 1,
				LiquidityTier:    1,
				RiskTier:         2,
			},
			{
				ID:               "member-router",
				Owner:            "Router Member",
				Kind:             "member",
				Status:           "active",
				AdjustmentGroup:  "floor",
				Expected:         map[string]Amount{"USD": 10_000},
				Executed:         map[string]Amount{"USD": 10_000},
				Turnover:         map[string]Amount{"USD": 15_000_000},
				SettlementWeight: 6,
				LiquidityTier:    3,
				RiskTier:         1,
			},
		},
	}
}

func multiAssetFixture() Fixture {
	policy := defaultPolicy()
	policy.WeightingMode = "risk-adjusted"
	return Fixture{
		Name:        "multi-asset",
		Description: "two-asset snapshot with independent reconciliation lanes",
		Epoch:       4404,
		Book:        "df-cross",
		Assets:      defaultAssets(),
		Policy:      policy,
		Labels: map[string]string{
			"desk": "cross-asset",
			"mode": "snapshot",
		},
		Accounts: []Account{
			{
				ID:               "member-alpha",
				Owner:            "Alpha Markets",
				Kind:             "member",
				Status:           "active",
				AdjustmentGroup:  "cross",
				Expected:         map[string]Amount{"USD": 1_100_000, "EUR": 900_000},
				Executed:         map[string]Amount{"USD": 1_099_994, "EUR": 900_000},
				Turnover:         map[string]Amount{"USD": 9_000_000, "EUR": 5_000_000},
				SettlementWeight: 3,
				LiquidityTier:    2,
				RiskTier:         1,
			},
			{
				ID:               "member-beta",
				Owner:            "Beta Clearing",
				Kind:             "member",
				Status:           "active",
				AdjustmentGroup:  "cross",
				Expected:         map[string]Amount{"USD": 400_000, "EUR": 450_000},
				Executed:         map[string]Amount{"USD": 400_000, "EUR": 449_992},
				Turnover:         map[string]Amount{"USD": 4_000_000, "EUR": 3_500_000},
				SettlementWeight: 2,
				LiquidityTier:    2,
				RiskTier:         2,
			},
			{
				ID:               "member-gamma",
				Owner:            "Gamma Flow",
				Kind:             "member",
				Status:           "active",
				AdjustmentGroup:  "cross",
				Expected:         map[string]Amount{"USD": 750_000, "EUR": 250_000},
				Executed:         map[string]Amount{"USD": 750_000, "EUR": 250_000},
				Turnover:         map[string]Amount{"USD": 12_000_000, "EUR": 2_500_000},
				SettlementWeight: 5,
				LiquidityTier:    3,
				RiskTier:         1,
			},
		},
	}
}
