package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

type GovernanceStatus string

const (
	GovernanceScheduled GovernanceStatus = "scheduled"
	GovernanceReady     GovernanceStatus = "ready"
	GovernanceExecuted  GovernanceStatus = "executed"
	GovernanceCancelled GovernanceStatus = "cancelled"
	GovernanceExpired   GovernanceStatus = "expired"
)

type GovernanceSpec struct {
	Domain       string `json:"domain"`
	Network      string `json:"network"`
	Target       string `json:"target"`
	Method       string `json:"method"`
	PayloadHash  string `json:"payloadHash"`
	Salt         string `json:"salt"`
	ExecuteAfter int64  `json:"executeAfter"`
	ExpiresAt    int64  `json:"expiresAt"`
	Predecessor  string `json:"predecessor,omitempty"`
}

type GovernanceRecord struct {
	ID               string           `json:"id"`
	Spec             GovernanceSpec   `json:"spec"`
	Approvals        []string         `json:"approvals"`
	Status           GovernanceStatus `json:"status"`
	ScheduledAt      int64            `json:"scheduledAt"`
	ExecutedAt       int64            `json:"executedAt,omitempty"`
	CancelledAt      int64            `json:"cancelledAt,omitempty"`
	CancellationNote string           `json:"cancellationNote,omitempty"`
}

type GovernanceBook struct {
	governors  map[string]bool
	guardian   string
	quorum     int
	operations map[string]*GovernanceRecord
}

func NewGovernanceBook(governors []string, quorum int, guardian string) (*GovernanceBook, error) {
	set := make(map[string]bool)
	for _, governor := range governors {
		if governor == "" || governor != strings.TrimSpace(governor) {
			return nil, fmt.Errorf("governor identity is not normalized")
		}
		if set[governor] {
			return nil, fmt.Errorf("duplicate governor %s", governor)
		}
		set[governor] = true
	}
	if len(set) == 0 || quorum <= 0 || quorum > len(set) {
		return nil, fmt.Errorf("invalid governance quorum")
	}
	if guardian == "" || guardian != strings.TrimSpace(guardian) || set[guardian] {
		return nil, fmt.Errorf("guardian must be normalized and independent")
	}
	return &GovernanceBook{
		governors:  set,
		guardian:   guardian,
		quorum:     quorum,
		operations: make(map[string]*GovernanceRecord),
	}, nil
}

func (spec GovernanceSpec) Validate() error {
	for name, value := range map[string]string{
		"domain":       spec.Domain,
		"network":      spec.Network,
		"target":       spec.Target,
		"method":       spec.Method,
		"payload hash": spec.PayloadHash,
		"salt":         spec.Salt,
	} {
		if value == "" || value != strings.TrimSpace(value) {
			return fmt.Errorf("%s is not normalized", name)
		}
	}
	if !isSHA256(spec.PayloadHash) {
		return fmt.Errorf("payload hash must be lowercase SHA-256")
	}
	if spec.Predecessor != "" && !isSHA256(spec.Predecessor) {
		return fmt.Errorf("predecessor must be a lowercase operation id")
	}
	if spec.ExecuteAfter <= 0 || spec.ExpiresAt <= spec.ExecuteAfter {
		return fmt.Errorf("invalid governance time window")
	}
	return nil
}

func (spec GovernanceSpec) CanonicalBytes() ([]byte, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	output := make([]byte, 0, 256)
	for _, value := range []string{
		spec.Domain,
		spec.Network,
		spec.Target,
		spec.Method,
		spec.PayloadHash,
		spec.Salt,
		spec.Predecessor,
	} {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		output = append(output, length[:]...)
		output = append(output, value...)
	}
	var timestamp [8]byte
	binary.BigEndian.PutUint64(timestamp[:], uint64(spec.ExecuteAfter))
	output = append(output, timestamp[:]...)
	binary.BigEndian.PutUint64(timestamp[:], uint64(spec.ExpiresAt))
	output = append(output, timestamp[:]...)
	return output, nil
}

func (spec GovernanceSpec) ID() (string, error) {
	encoded, err := spec.CanonicalBytes()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func GovernancePayloadHash(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func (book *GovernanceBook) Schedule(spec GovernanceSpec, proposer string, now int64) (GovernanceRecord, error) {
	if !book.governors[proposer] {
		return GovernanceRecord{}, fmt.Errorf("proposer is not a governor")
	}
	if spec.ExecuteAfter <= now {
		return GovernanceRecord{}, fmt.Errorf("timelock must start in the future")
	}
	id, err := spec.ID()
	if err != nil {
		return GovernanceRecord{}, err
	}
	if _, exists := book.operations[id]; exists {
		return GovernanceRecord{}, fmt.Errorf("operation already scheduled")
	}
	record := &GovernanceRecord{
		ID:          id,
		Spec:        spec,
		Approvals:   []string{proposer},
		Status:      GovernanceScheduled,
		ScheduledAt: now,
	}
	book.operations[id] = record
	return book.snapshot(record, now), nil
}

func (book *GovernanceBook) Approve(id string, governor string, now int64) (GovernanceRecord, error) {
	if !book.governors[governor] {
		return GovernanceRecord{}, fmt.Errorf("approver is not a governor")
	}
	record, err := book.live(id, now)
	if err != nil {
		return GovernanceRecord{}, err
	}
	for _, existing := range record.Approvals {
		if existing == governor {
			return book.snapshot(record, now), nil
		}
	}
	record.Approvals = append(record.Approvals, governor)
	sort.Strings(record.Approvals)
	return book.snapshot(record, now), nil
}

func (book *GovernanceBook) Execute(id string, now int64) (GovernanceRecord, error) {
	record, err := book.live(id, now)
	if err != nil {
		return GovernanceRecord{}, err
	}
	if record.Spec.Predecessor != "" {
		predecessor, exists := book.operations[record.Spec.Predecessor]
		if !exists || predecessor.Status != GovernanceExecuted {
			return GovernanceRecord{}, fmt.Errorf("predecessor is not executed")
		}
	}
	if now < record.Spec.ExecuteAfter || len(record.Approvals) < book.quorum {
		return GovernanceRecord{}, fmt.Errorf("operation is not ready")
	}
	record.Status = GovernanceExecuted
	record.ExecutedAt = now
	return book.snapshot(record, now), nil
}

func (book *GovernanceBook) Cancel(id string, guardian string, note string, now int64) (GovernanceRecord, error) {
	if guardian != book.guardian || note == "" || note != strings.TrimSpace(note) {
		return GovernanceRecord{}, fmt.Errorf("guardian and normalized note are required")
	}
	record, err := book.live(id, now)
	if err != nil {
		return GovernanceRecord{}, err
	}
	record.Status = GovernanceCancelled
	record.CancelledAt = now
	record.CancellationNote = note
	return book.snapshot(record, now), nil
}

func (book *GovernanceBook) Get(id string, now int64) (GovernanceRecord, bool) {
	record, exists := book.operations[id]
	if !exists {
		return GovernanceRecord{}, false
	}
	return book.snapshot(record, now), true
}

func (book *GovernanceBook) live(id string, now int64) (*GovernanceRecord, error) {
	record, exists := book.operations[id]
	if !exists {
		return nil, fmt.Errorf("operation not found")
	}
	if now >= record.Spec.ExpiresAt && record.Status != GovernanceExecuted && record.Status != GovernanceCancelled {
		record.Status = GovernanceExpired
	}
	switch record.Status {
	case GovernanceExecuted, GovernanceCancelled, GovernanceExpired:
		return nil, fmt.Errorf("operation is terminal")
	default:
		return record, nil
	}
}

func (book *GovernanceBook) snapshot(record *GovernanceRecord, now int64) GovernanceRecord {
	copy := *record
	copy.Approvals = append([]string{}, record.Approvals...)
	if copy.Status != GovernanceExecuted && copy.Status != GovernanceCancelled {
		switch {
		case now >= copy.Spec.ExpiresAt:
			copy.Status = GovernanceExpired
		case now >= copy.Spec.ExecuteAfter && len(copy.Approvals) >= book.quorum:
			copy.Status = GovernanceReady
		default:
			copy.Status = GovernanceScheduled
		}
	}
	return copy
}

func isSHA256(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
