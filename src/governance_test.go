package main

import "testing"

func governanceFixture() GovernanceSpec {
	return GovernanceSpec{
		Domain:       "deltaforge.governance.v1",
		Network:      "settlement-eu-1",
		Target:       "policy:usd-primary",
		Method:       "set_correction_capacity",
		PayloadHash:  GovernancePayloadHash([]byte("capacity=500000")),
		Salt:         "capacity-2026-08",
		ExecuteAfter: 10,
		ExpiresAt:    20,
	}
}

func TestGovernanceIDIsDeterministicAndSeparated(t *testing.T) {
	first := governanceFixture()
	firstID, err := first.ID()
	if err != nil {
		t.Fatal(err)
	}
	second := first
	second.Network = "settlement-us-1"
	secondID, err := second.ID()
	if err != nil {
		t.Fatal(err)
	}
	if firstID == secondID {
		t.Fatal("network must separate operation ids")
	}
}

func TestGovernanceRequiresQuorumAndTimelock(t *testing.T) {
	book, err := NewGovernanceBook([]string{"alice", "bob", "carol"}, 2, "guardian")
	if err != nil {
		t.Fatal(err)
	}
	record, err := book.Schedule(governanceFixture(), "alice", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := book.Execute(record.ID, 10); err == nil {
		t.Fatal("execution without quorum was accepted")
	}
	if _, err := book.Approve(record.ID, "bob", 5); err != nil {
		t.Fatal(err)
	}
	if _, err := book.Execute(record.ID, 9); err == nil {
		t.Fatal("execution before timelock was accepted")
	}
	executed, err := book.Execute(record.ID, 10)
	if err != nil || executed.Status != GovernanceExecuted {
		t.Fatalf("operation was not executed: %+v %v", executed, err)
	}
}

func TestGovernanceGuardianCancellationIsTerminal(t *testing.T) {
	book, err := NewGovernanceBook([]string{"alice"}, 1, "guardian")
	if err != nil {
		t.Fatal(err)
	}
	record, err := book.Schedule(governanceFixture(), "alice", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := book.Cancel(record.ID, "other", "capacity review", 2); err == nil {
		t.Fatal("unknown guardian was accepted")
	}
	cancelled, err := book.Cancel(record.ID, "guardian", "capacity review", 2)
	if err != nil || cancelled.Status != GovernanceCancelled {
		t.Fatalf("operation was not cancelled: %+v %v", cancelled, err)
	}
	if _, err := book.Execute(record.ID, 10); err == nil {
		t.Fatal("cancelled operation was executed")
	}
}

func TestGovernanceSeparatesGuardianFromCouncil(t *testing.T) {
	if _, err := NewGovernanceBook([]string{"alice"}, 1, "alice"); err == nil {
		t.Fatal("guardian must be independent")
	}
}
