package main

import "testing"

func capitalFixture(asset string, pool Amount) CapitalInput {
	return CapitalInput{
		Asset:                asset,
		Group:                "primary",
		Reserve:              1_000_000,
		LiquidReserve:        800_000,
		GrossExpected:        900_000,
		GrossExecuted:        870_000,
		OpenCorrections:      20_000,
		GlobalPool:           pool,
		CorrectionCapacity:   500_000,
		ReserveHaircutBPS:    500,
		DriftShockBPS:        2_000,
		OperationalBufferBPS: 800,
	}
}

func TestCapitalUsesConservativeRounding(t *testing.T) {
	metrics, err := EvaluateCapital(capitalFixture("USD", 10_000))
	if err != nil {
		t.Fatal(err)
	}
	if metrics.EffectiveReserve != 950_000 || metrics.GrossSettlementDemand != 60_000 {
		t.Fatalf("unexpected reserve or demand: %+v", metrics)
	}
	if metrics.StressedSettlementDemand != 72_000 || metrics.OperationalBuffer != 72_000 {
		t.Fatalf("unexpected stress result: %+v", metrics)
	}
	if metrics.RequiredReserve != 144_000 || !metrics.Compliant {
		t.Fatalf("unexpected capital result: %+v", metrics)
	}
}

func TestCapitalFailsClosedOnIlliquidity(t *testing.T) {
	input := capitalFixture("USD", 10_000)
	input.LiquidReserve = 100_000
	metrics, err := EvaluateCapital(input)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.ReserveShortfall != 44_000 || metrics.Compliant {
		t.Fatalf("expected liquidity shortfall: %+v", metrics)
	}
}

func TestPortfolioReportsDemandConcentration(t *testing.T) {
	first := capitalFixture("USD", 10_000)
	second := capitalFixture("EUR", 0)
	second.GrossExpected = 880_000
	portfolio, err := EvaluatePortfolio([]CapitalInput{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if len(portfolio.Routes) != 2 || portfolio.DemandHHIBPS == 0 || !portfolio.Compliant {
		t.Fatalf("unexpected portfolio: %+v", portfolio)
	}
}

func TestPortfolioRejectsDuplicateRoutes(t *testing.T) {
	input := capitalFixture("USD", 0)
	if _, err := EvaluatePortfolio([]CapitalInput{input, input}); err == nil {
		t.Fatal("expected duplicate route error")
	}
}
