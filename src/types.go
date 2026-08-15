package main

import (
	"sort"
	"strings"
)

type Amount int64

const (
	DefaultScale       int64  = 1_000_000
	DefaultDustLimit   Amount = 5
	DefaultMicroLimit  Amount = 12
	DefaultGlobalLimit Amount = 2_500
	DefaultFloor       Amount = 0
)

type Asset struct {
	ID                    string            `json:"id"`
	Symbol                string            `json:"symbol"`
	Name                  string            `json:"name"`
	Scale                 int64             `json:"scale"`
	Precision             int               `json:"precision"`
	DustLimit             Amount            `json:"dustLimit"`
	MicroTolerance        Amount            `json:"microTolerance"`
	GlobalTolerance       Amount            `json:"globalTolerance"`
	SettlementFloor       Amount            `json:"settlementFloor"`
	LiquidationHaircutBps int64             `json:"liquidationHaircutBps"`
	GlobalEnabled         bool              `json:"globalEnabled"`
	Metadata              map[string]string `json:"metadata,omitempty"`
}

func (a Asset) WithDefaults() Asset {
	if a.Scale == 0 {
		a.Scale = DefaultScale
	}
	if a.Precision == 0 {
		a.Precision = 6
	}
	if a.DustLimit == 0 {
		a.DustLimit = DefaultDustLimit
	}
	if a.MicroTolerance == 0 {
		a.MicroTolerance = DefaultMicroLimit
	}
	if a.GlobalTolerance == 0 {
		a.GlobalTolerance = DefaultGlobalLimit
	}
	if a.SettlementFloor == 0 {
		a.SettlementFloor = DefaultFloor
	}
	if a.LiquidationHaircutBps == 0 {
		a.LiquidationHaircutBps = 15
	}
	if a.Symbol == "" {
		a.Symbol = strings.ToUpper(a.ID)
	}
	if a.Name == "" {
		a.Name = a.Symbol
	}
	return a
}

type Account struct {
	ID               string            `json:"id"`
	Owner            string            `json:"owner"`
	Kind             string            `json:"kind"`
	Region           string            `json:"region,omitempty"`
	Desk             string            `json:"desk,omitempty"`
	Status           string            `json:"status,omitempty"`
	AdjustmentGroup  string            `json:"adjustmentGroup,omitempty"`
	Expected         map[string]Amount `json:"expected"`
	Executed         map[string]Amount `json:"executed"`
	Final            map[string]Amount `json:"final,omitempty"`
	Turnover         map[string]Amount `json:"turnover,omitempty"`
	Locked           map[string]Amount `json:"locked,omitempty"`
	SettlementWeight int64             `json:"settlementWeight"`
	LiquidityTier    int64             `json:"liquidityTier"`
	RiskTier         int64             `json:"riskTier"`
	Flags            []string          `json:"flags,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

func (a Account) WithDefaults() Account {
	a.Expected = ensureAmountMap(a.Expected)
	a.Executed = ensureAmountMap(a.Executed)
	a.Final = ensureAmountMap(a.Final)
	a.Turnover = ensureAmountMap(a.Turnover)
	a.Locked = ensureAmountMap(a.Locked)
	if len(a.Final) == 0 {
		a.Final = cloneAmountMap(a.Executed)
	}
	if a.Owner == "" {
		a.Owner = a.ID
	}
	if a.Kind == "" {
		a.Kind = "member"
	}
	if a.Status == "" {
		a.Status = "active"
	}
	if a.AdjustmentGroup == "" {
		a.AdjustmentGroup = "primary"
	}
	if a.SettlementWeight == 0 {
		a.SettlementWeight = 1
	}
	if a.LiquidityTier == 0 {
		a.LiquidityTier = 1
	}
	if a.RiskTier == 0 {
		a.RiskTier = 1
	}
	return a
}

func (a Account) HasFlag(flag string) bool {
	for _, value := range a.Flags {
		if strings.EqualFold(value, flag) {
			return true
		}
	}
	return false
}

func (a Account) IsActive() bool {
	return strings.EqualFold(a.Status, "active") || strings.EqualFold(a.Status, "settling")
}

func (a Account) CanReceiveGlobal() bool {
	if a.HasFlag("no-global") || a.HasFlag("frozen") {
		return false
	}
	if !a.IsActive() {
		return false
	}
	switch strings.ToLower(a.Kind) {
	case "treasury", "reserve", "observer":
		return false
	default:
		return true
	}
}

func (a Account) AssetIDs() []string {
	set := make(map[string]bool)
	for id := range a.Expected {
		set[id] = true
	}
	for id := range a.Executed {
		set[id] = true
	}
	for id := range a.Final {
		set[id] = true
	}
	for id := range a.Turnover {
		set[id] = true
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

type AssetPolicy struct {
	Asset                 string `json:"asset"`
	MicroTolerance        Amount `json:"microTolerance"`
	GlobalTolerance       Amount `json:"globalTolerance"`
	SettlementFloor       Amount `json:"settlementFloor"`
	ReceiverCap           Amount `json:"receiverCap"`
	LiquidationHaircutBps int64  `json:"liquidationHaircutBps"`
	Enabled               bool   `json:"enabled"`
}

type Policy struct {
	MicroTolerance        Amount        `json:"microTolerance"`
	GlobalTolerance       Amount        `json:"globalTolerance"`
	CorrectionThreshold   Amount        `json:"correctionThreshold"`
	MaxCorrection         Amount        `json:"maxCorrection"`
	MinSettlementWeight   int64         `json:"minSettlementWeight"`
	MaxGlobalReceivers    int           `json:"maxGlobalReceivers"`
	SettlementFloor       Amount        `json:"settlementFloor"`
	WeightingMode         string        `json:"weightingMode"`
	AllowNegativeFinal    bool          `json:"allowNegativeFinal"`
	IncludeInactive       bool          `json:"includeInactive"`
	EventLevel            string        `json:"eventLevel"`
	AssetOverrides        []AssetPolicy `json:"assetOverrides,omitempty"`
	FrozenKinds           []string      `json:"frozenKinds,omitempty"`
	RequireBalancedTotals bool          `json:"requireBalancedTotals"`
}

func (p Policy) WithDefaults() Policy {
	if p.MicroTolerance == 0 {
		p.MicroTolerance = DefaultMicroLimit
	}
	if p.GlobalTolerance == 0 {
		p.GlobalTolerance = DefaultGlobalLimit
	}
	if p.CorrectionThreshold == 0 {
		p.CorrectionThreshold = p.MicroTolerance + 1
	}
	if p.MaxCorrection == 0 {
		p.MaxCorrection = 1_000_000_000
	}
	if p.MinSettlementWeight == 0 {
		p.MinSettlementWeight = 1
	}
	if p.MaxGlobalReceivers == 0 {
		p.MaxGlobalReceivers = 64
	}
	if p.WeightingMode == "" {
		p.WeightingMode = "turnover"
	}
	if p.EventLevel == "" {
		p.EventLevel = "summary"
	}
	return p
}

func (p Policy) IsFrozenKind(kind string) bool {
	for _, value := range p.FrozenKinds {
		if strings.EqualFold(value, kind) {
			return true
		}
	}
	return false
}

type Fixture struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Epoch       int64             `json:"epoch"`
	Book        string            `json:"book,omitempty"`
	Assets      []Asset           `json:"assets"`
	Accounts    []Account         `json:"accounts"`
	Policy      Policy            `json:"policy"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type RunOptions struct {
	IncludeEvents bool
	Strict        bool
	ScenarioName  string
	Source        string
}

type DifferenceKind string

const (
	DifferenceExact      DifferenceKind = "exact"
	DifferenceMicro      DifferenceKind = "micro"
	DifferenceCorrection DifferenceKind = "correction"
	DifferenceBlocked    DifferenceKind = "blocked"
)

type Difference struct {
	Account       string         `json:"account"`
	Owner         string         `json:"owner"`
	Asset         string         `json:"asset"`
	Expected      Amount         `json:"expected"`
	Executed      Amount         `json:"executed"`
	Drift         Amount         `json:"drift"`
	AbsDrift      Amount         `json:"absDrift"`
	Kind          DifferenceKind `json:"kind"`
	Group         string         `json:"group"`
	Reason        string         `json:"reason"`
	Weight        int64          `json:"weight"`
	CorrectionRef string         `json:"correctionRef,omitempty"`
}

type GlobalPool struct {
	Asset           string `json:"asset"`
	Group           string `json:"group"`
	Amount          Amount `json:"amount"`
	AbsAmount       Amount `json:"absAmount"`
	Entries         int    `json:"entries"`
	PositiveEntries int    `json:"positiveEntries"`
	NegativeEntries int    `json:"negativeEntries"`
	MaxSingleDrift  Amount `json:"maxSingleDrift"`
}

func (p GlobalPool) Key() string {
	return p.Asset + "::" + p.Group
}

type CorrectionStatus string

const (
	CorrectionPlanned CorrectionStatus = "planned"
	CorrectionApplied CorrectionStatus = "applied"
	CorrectionSkipped CorrectionStatus = "skipped"
	CorrectionCapped  CorrectionStatus = "capped"
)

type Correction struct {
	ID      string           `json:"id"`
	Account string           `json:"account"`
	Asset   string           `json:"asset"`
	Amount  Amount           `json:"amount"`
	Before  Amount           `json:"before"`
	After   Amount           `json:"after"`
	Reason  string           `json:"reason"`
	Status  CorrectionStatus `json:"status"`
	Epoch   int64            `json:"epoch"`
	Trace   string           `json:"trace"`
}

type Allocation struct {
	ID       string `json:"id"`
	Account  string `json:"account"`
	Asset    string `json:"asset"`
	Group    string `json:"group"`
	Amount   Amount `json:"amount"`
	Weight   int64  `json:"weight"`
	Before   Amount `json:"before"`
	After    Amount `json:"after"`
	Pool     string `json:"pool"`
	Sequence int    `json:"sequence"`
}

type Liquidation struct {
	ID         string `json:"id"`
	Account    string `json:"account"`
	Asset      string `json:"asset"`
	Starting   Amount `json:"starting"`
	Settled    Amount `json:"settled"`
	Haircut    Amount `json:"haircut"`
	Final      Amount `json:"final"`
	Reason     string `json:"reason"`
	HaircutBps int64  `json:"haircutBps"`
	Sequence   int    `json:"sequence"`
}

type Event struct {
	Type     string            `json:"type"`
	Epoch    int64             `json:"epoch"`
	Account  string            `json:"account,omitempty"`
	Asset    string            `json:"asset,omitempty"`
	Amount   Amount            `json:"amount,omitempty"`
	Message  string            `json:"message,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type BalanceLine struct {
	Asset    string `json:"asset"`
	Expected Amount `json:"expected"`
	Executed Amount `json:"executed"`
	Final    Amount `json:"final"`
	Drift    Amount `json:"drift"`
	Delta    Amount `json:"delta"`
	Locked   Amount `json:"locked"`
}

type AccountReport struct {
	ID               string        `json:"id"`
	Owner            string        `json:"owner"`
	Kind             string        `json:"kind"`
	Status           string        `json:"status"`
	AdjustmentGroup  string        `json:"adjustmentGroup"`
	SettlementWeight int64         `json:"settlementWeight"`
	RiskTier         int64         `json:"riskTier"`
	LiquidityTier    int64         `json:"liquidityTier"`
	Balances         []BalanceLine `json:"balances"`
	Flags            []string      `json:"flags,omitempty"`
}

type AssetReport struct {
	ID              string `json:"id"`
	Symbol          string `json:"symbol"`
	ExpectedTotal   Amount `json:"expectedTotal"`
	ExecutedTotal   Amount `json:"executedTotal"`
	FinalTotal      Amount `json:"finalTotal"`
	GlobalNet       Amount `json:"globalNet"`
	CorrectionNet   Amount `json:"correctionNet"`
	LiquidationNet  Amount `json:"liquidationNet"`
	Drift           Amount `json:"drift"`
	AccountCount    int    `json:"accountCount"`
	MicroCount      int    `json:"microCount"`
	CorrectionCount int    `json:"correctionCount"`
}

type SnapshotReport struct {
	Epoch              int64             `json:"epoch"`
	Book               string            `json:"book"`
	AccountCount       int               `json:"accountCount"`
	AssetCount         int               `json:"assetCount"`
	ExpectedTotals     map[string]Amount `json:"expectedTotals"`
	ExecutedTotals     map[string]Amount `json:"executedTotals"`
	FinalTotals        map[string]Amount `json:"finalTotals"`
	LockedTotals       map[string]Amount `json:"lockedTotals"`
	Hash               string            `json:"hash"`
	CanonicalLineCount int               `json:"canonicalLineCount"`
}

type ReconciliationReport struct {
	Differences     []Difference   `json:"differences"`
	GlobalPools     []GlobalPool   `json:"globalPools"`
	MicroTotal      Amount         `json:"microTotal"`
	CorrectionTotal Amount         `json:"correctionTotal"`
	ExactCount      int            `json:"exactCount"`
	MicroCount      int            `json:"microCount"`
	CorrectionCount int            `json:"correctionCount"`
	BlockedCount    int            `json:"blockedCount"`
	BalancedByAsset []AssetBalance `json:"balancedByAsset"`
}

type AssetBalance struct {
	Asset         string `json:"asset"`
	ExpectedTotal Amount `json:"expectedTotal"`
	ExecutedTotal Amount `json:"executedTotal"`
	FinalTotal    Amount `json:"finalTotal"`
	ObservedDrift Amount `json:"observedDrift"`
	SettledDrift  Amount `json:"settledDrift"`
	Balanced      bool   `json:"balanced"`
}

type SettlementReport struct {
	Allocations       []Allocation  `json:"allocations"`
	Liquidations      []Liquidation `json:"liquidations"`
	AllocationTotal   Amount        `json:"allocationTotal"`
	LiquidationTotal  Amount        `json:"liquidationTotal"`
	ReceiverCount     int           `json:"receiverCount"`
	PoolCount         int           `json:"poolCount"`
	RoundingRemainder Amount        `json:"roundingRemainder"`
	Completed         bool          `json:"completed"`
}

type Metrics struct {
	TotalExpected      map[string]Amount `json:"totalExpected"`
	TotalExecuted      map[string]Amount `json:"totalExecuted"`
	TotalFinal         map[string]Amount `json:"totalFinal"`
	NetDrift           map[string]Amount `json:"netDrift"`
	CorrectionVolume   Amount            `json:"correctionVolume"`
	GlobalVolume       Amount            `json:"globalVolume"`
	LiquidationVolume  Amount            `json:"liquidationVolume"`
	MaxAccountDrift    Amount            `json:"maxAccountDrift"`
	RiskWeightedDrift  Amount            `json:"riskWeightedDrift"`
	ReceiverHerfindahl int64             `json:"receiverHerfindahl"`
	AccountsTouched    int               `json:"accountsTouched"`
}

type Report struct {
	Name           string               `json:"name"`
	Description    string               `json:"description,omitempty"`
	Epoch          int64                `json:"epoch"`
	Book           string               `json:"book"`
	Source         string               `json:"source"`
	Assets         []AssetReport        `json:"assets"`
	Accounts       []AccountReport      `json:"accounts"`
	Snapshot       SnapshotReport       `json:"snapshot"`
	Reconciliation ReconciliationReport `json:"reconciliation"`
	Corrections    []Correction         `json:"corrections"`
	Settlement     SettlementReport     `json:"settlement"`
	Metrics        Metrics              `json:"metrics"`
	Events         []Event              `json:"events,omitempty"`
}

type weightedAccount struct {
	Account Account
	Asset   string
	Group   string
	Weight  int64
}

type shareLine struct {
	ID        string
	Weight    int64
	Share     Amount
	Remainder int64
	Index     int
}
