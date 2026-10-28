package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCanonicalBondPairMe(t *testing.T) {
	id := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	a, b, err := CanonicalBondPair(id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a != nil || b != id {
		t.Fatalf("got %v %v", a, b)
	}
}

func TestCanonicalBondPairSorts(t *testing.T) {
	low := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	high := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	a, b, err := CanonicalBondPair(high, &low)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil || *a != low || b != high {
		t.Fatalf("got %v %v", a, b)
	}
}

func TestCanonicalBondPairRejectsSelf(t *testing.T) {
	id := uuid.New()
	if _, _, err := CanonicalBondPair(id, &id); err == nil {
		t.Fatal("expected invalid self")
	}
}

func TestPersonBondEndTwice(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	id := uuid.New()
	bond, err := NewPersonBond(nil, id, BondKindFriend, "", MoscowDate(now), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := bond.End(nil, now); err != nil {
		t.Fatal(err)
	}
	if err := bond.End(nil, now); err == nil {
		t.Fatal("expected invalid second end")
	}
}

func TestPersonBondChangeSetsChangedOn(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	id := uuid.New()
	bond, err := NewPersonBond(nil, id, BondKindAcquaintance, "hi", start, start)
	if err != nil {
		t.Fatal(err)
	}
	comment := "now friends"
	if err := bond.Change(BondChange{Kind: "friend", Comment: &comment}, now); err != nil {
		t.Fatal(err)
	}
	if bond.Kind != BondKindFriend || bond.Comment != comment {
		t.Fatalf("got %+v", bond)
	}
	if !bond.ChangedOn.Equal(MoscowDate(now)) {
		t.Fatalf("changedOn %v", bond.ChangedOn)
	}
}

func TestPersonBondViewedFromMe(t *testing.T) {
	id := uuid.New()
	bond, err := NewPersonBond(nil, id, BondKindFriend, "", time.Now(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	view := bond.ViewedFrom(id)
	if view.OtherID != nil || view.OtherName != "Me" {
		t.Fatalf("got %+v", view)
	}
}

func TestParseBondKindOther(t *testing.T) {
	kind, err := ParseBondKind("other")
	if err != nil {
		t.Fatal(err)
	}
	if kind != BondKindOther {
		t.Fatalf("got %s", kind)
	}
}

func TestParseBondKindColleague(t *testing.T) {
	kind, err := ParseBondKind("colleague")
	if err != nil {
		t.Fatal(err)
	}
	if kind != BondKindColleague {
		t.Fatalf("got %s", kind)
	}
}

func TestNewPersonBondColleague(t *testing.T) {
	id := uuid.New()
	bond, err := NewPersonBond(nil, id, BondKindColleague, "", time.Now(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if bond.Kind != BondKindColleague {
		t.Fatalf("got %s", bond.Kind)
	}
}
