package main

import (
	"fmt"
	"math"
	"math/big"
	"sort"
)

const capitalBPS int64 = 10_000

type CapitalInput struct {
	Asset                string `json:"asset"`
	Group                string `json:"group"`
	Reserve              Amount `json:"reserve"`
	LiquidReserve        Amount `json:"liquidReserve"`
	GrossExpected        Amount `json:"grossExpected"`
	GrossExecuted        Amount `json:"grossExecuted"`
	OpenCorrections      Amount `json:"openCorrections"`
	GlobalPool           Amount `json:"globalPool"`
	CorrectionCapacity   Amount `json:"correctionCapacity"`
	ReserveHaircutBPS    int64  `json:"reserveHaircutBps"`
	DriftShockBPS        int64  `json:"driftShockBps"`
	OperationalBufferBPS int64  `json:"operationalBufferBps"`
}

type CapitalMetrics struct {
	Asset                    string `json:"asset"`
	Group                    string `json:"group"`
	ObservedDrift            Amount `json:"observedDrift"`
	GrossSettlementDemand    Amount `json:"grossSettlementDemand"`
	StressedSettlementDemand Amount `json:"stressedSettlementDemand"`
	EffectiveReserve         Amount `json:"effectiveReserve"`
	OperationalBuffer        Amount `json:"operationalBuffer"`
	RequiredReserve          Amount `json:"requiredReserve"`
	AvailableReserve         Amount `json:"availableReserve"`
	ReserveShortfall         Amount `json:"reserveShortfall"`
	CoverageBPS              int64  `json:"coverageBps"`
	LiquidityBPS             int64  `json:"liquidityBps"`
	CapacityUtilizationBPS   int64  `json:"capacityUtilizationBps"`
	AvailableCapacity        Amount `json:"availableCapacity"`
	Compliant                bool   `json:"compliant"`
}

type PortfolioCapital struct {
	Routes                []CapitalMetrics `json:"routes"`
	EffectiveReserve      Amount           `json:"effectiveReserve"`
	RequiredReserve       Amount           `json:"requiredReserve"`
	ReserveShortfall      Amount           `json:"reserveShortfall"`
	CoverageBPS           int64            `json:"coverageBps"`
	DemandHHIBPS          int64            `json:"demandHhiBps"`
	LargestDemandShareBPS int64            `json:"largestDemandShareBps"`
	CompliantRoutes       int              `json:"compliantRoutes"`
	Compliant             bool             `json:"compliant"`
}

func (input CapitalInput) Validate() error {
	if input.Asset == "" || input.Group == "" {
		return fmt.Errorf("asset and group are required")
	}
	for name, value := range map[string]Amount{
		"reserve":             input.Reserve,
		"liquid reserve":      input.LiquidReserve,
		"gross expected":      input.GrossExpected,
		"gross executed":      input.GrossExecuted,
		"open corrections":    input.OpenCorrections,
		"correction capacity": input.CorrectionCapacity,
	} {
		if value < 0 {
			return fmt.Errorf("%s must be non-negative", name)
		}
	}
	if input.LiquidReserve > input.Reserve {
		return fmt.Errorf("liquid reserve exceeds reserve")
	}
	if input.ReserveHaircutBPS < 0 || input.ReserveHaircutBPS > capitalBPS {
		return fmt.Errorf("reserve haircut is outside basis-point domain")
	}
	if input.DriftShockBPS < 0 || input.OperationalBufferBPS < 0 {
		return fmt.Errorf("stress parameters must be non-negative")
	}
	return nil
}

func EvaluateCapital(input CapitalInput) (CapitalMetrics, error) {
	if err := input.Validate(); err != nil {
		return CapitalMetrics{}, err
	}
	observedDrift := absAmount(input.GrossExpected - input.GrossExecuted)
	grossDemand, ok := checkedAdd(observedDrift, absAmount(input.GlobalPool))
	if !ok {
		return CapitalMetrics{}, fmt.Errorf("settlement demand overflow")
	}
	grossDemand, ok = checkedAdd(grossDemand, input.OpenCorrections)
	if !ok {
		return CapitalMetrics{}, fmt.Errorf("settlement demand overflow")
	}
	stressedDemand, err := capitalMulDivCeil(grossDemand, capitalBPS+input.DriftShockBPS, capitalBPS)
	if err != nil {
		return CapitalMetrics{}, err
	}
	effectiveReserve, err := capitalMulDivFloor(input.Reserve, capitalBPS-input.ReserveHaircutBPS, capitalBPS)
	if err != nil {
		return CapitalMetrics{}, err
	}
	buffer, err := capitalMulDivCeil(input.GrossExpected, input.OperationalBufferBPS, capitalBPS)
	if err != nil {
		return CapitalMetrics{}, err
	}
	required, ok := checkedAdd(stressedDemand, buffer)
	if !ok {
		return CapitalMetrics{}, fmt.Errorf("required reserve overflow")
	}
	available := minAmount(effectiveReserve, input.LiquidReserve)
	shortfall := maxAmount(0, required-available)
	capacityHeadroom := maxAmount(0, available-required)
	return CapitalMetrics{
		Asset:                    input.Asset,
		Group:                    input.Group,
		ObservedDrift:            observedDrift,
		GrossSettlementDemand:    grossDemand,
		StressedSettlementDemand: stressedDemand,
		EffectiveReserve:         effectiveReserve,
		OperationalBuffer:        buffer,
		RequiredReserve:          required,
		AvailableReserve:         available,
		ReserveShortfall:         shortfall,
		CoverageBPS:              capitalRatioBPS(effectiveReserve, required),
		LiquidityBPS:             capitalRatioBPS(input.LiquidReserve, input.Reserve),
		CapacityUtilizationBPS:   capitalRatioBPS(grossDemand, input.CorrectionCapacity),
		AvailableCapacity:        minAmount(input.CorrectionCapacity, capacityHeadroom),
		Compliant:                shortfall == 0,
	}, nil
}

func EvaluatePortfolio(inputs []CapitalInput) (PortfolioCapital, error) {
	if len(inputs) == 0 {
		return PortfolioCapital{}, fmt.Errorf("portfolio requires at least one route")
	}
	result := PortfolioCapital{Routes: make([]CapitalMetrics, 0, len(inputs))}
	seen := make(map[string]bool)
	var totalDemand Amount
	for _, input := range inputs {
		key := input.Asset + "::" + input.Group
		if seen[key] {
			return PortfolioCapital{}, fmt.Errorf("duplicate capital route %s", key)
		}
		seen[key] = true
		metrics, err := EvaluateCapital(input)
		if err != nil {
			return PortfolioCapital{}, fmt.Errorf("%s: %w", key, err)
		}
		var ok bool
		result.EffectiveReserve, ok = checkedAdd(result.EffectiveReserve, metrics.EffectiveReserve)
		if !ok {
			return PortfolioCapital{}, fmt.Errorf("portfolio reserve overflow")
		}
		result.RequiredReserve, ok = checkedAdd(result.RequiredReserve, metrics.RequiredReserve)
		if !ok {
			return PortfolioCapital{}, fmt.Errorf("portfolio requirement overflow")
		}
		result.ReserveShortfall, ok = checkedAdd(result.ReserveShortfall, metrics.ReserveShortfall)
		if !ok {
			return PortfolioCapital{}, fmt.Errorf("portfolio shortfall overflow")
		}
		totalDemand, ok = checkedAdd(totalDemand, metrics.GrossSettlementDemand)
		if !ok {
			return PortfolioCapital{}, fmt.Errorf("portfolio demand overflow")
		}
		if metrics.Compliant {
			result.CompliantRoutes++
		}
		result.Routes = append(result.Routes, metrics)
	}
	sort.SliceStable(result.Routes, func(i, j int) bool {
		if result.Routes[i].Asset == result.Routes[j].Asset {
			return result.Routes[i].Group < result.Routes[j].Group
		}
		return result.Routes[i].Asset < result.Routes[j].Asset
	})
	result.CoverageBPS = capitalRatioBPS(result.EffectiveReserve, result.RequiredReserve)
	for _, route := range result.Routes {
		share := capitalRatioBPS(route.GrossSettlementDemand, totalDemand)
		result.DemandHHIBPS += share * share / capitalBPS
		if share > result.LargestDemandShareBPS {
			result.LargestDemandShareBPS = share
		}
	}
	result.Compliant = result.ReserveShortfall == 0 && result.CompliantRoutes == len(inputs)
	return result, nil
}

func capitalRatioBPS(numerator Amount, denominator Amount) int64 {
	if denominator == 0 {
		if numerator == 0 {
			return 0
		}
		return capitalBPS
	}
	value, err := capitalMulDivFloor(absAmount(numerator), capitalBPS, int64(absAmount(denominator)))
	if err != nil || value > Amount(math.MaxInt64) {
		return math.MaxInt64
	}
	return int64(value)
}

func capitalMulDivFloor(value Amount, multiplier int64, denominator int64) (Amount, error) {
	return capitalMulDiv(value, multiplier, denominator, false)
}

func capitalMulDivCeil(value Amount, multiplier int64, denominator int64) (Amount, error) {
	return capitalMulDiv(value, multiplier, denominator, true)
}

func capitalMulDiv(value Amount, multiplier int64, denominator int64, roundUp bool) (Amount, error) {
	if value < 0 || multiplier < 0 || denominator <= 0 {
		return 0, fmt.Errorf("invalid capital arithmetic domain")
	}
	product := new(big.Int).Mul(big.NewInt(int64(value)), big.NewInt(multiplier))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(product, big.NewInt(denominator), remainder)
	if roundUp && remainder.Sign() != 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("capital arithmetic overflow")
	}
	return Amount(quotient.Int64()), nil
}
