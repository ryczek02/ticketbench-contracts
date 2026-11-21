package scenario

import (
	"reflect"
	"testing"

	"github.com/ryczek02/ticketbench-contracts/pkg/datagen"
	"github.com/ryczek02/ticketbench-contracts/pkg/model"
)

func dataset(t testing.TB) *datagen.Dataset {
	t.Helper()
	d, err := datagen.New(datagen.Spec{Tickets: 100_000, Seed: 42})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestCatalog(t *testing.T) {
	if got := len(CRUD()); got != 24 {
		t.Fatalf("brief requires 24 CRUD scenarios, have %d", got)
	}
	per := map[Op]int{}
	for _, d := range All {
		per[d.Op]++
	}
	for _, op := range []Op{Create, Read, Update, Delete} {
		if per[op] != 6 {
			t.Errorf("op %s has %d scenarios, want 6", op, per[op])
		}
	}
}

func TestParse(t *testing.T) {
	cases := map[string]int{"all": len(All), "crud": 24, "C*": 6, "r1, R2,r1": 2, "J*,D6": 4}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if len(got) != want {
			t.Errorf("%q: got %d ids, want %d", in, len(got), want)
		}
	}
	if _, err := Parse("X9"); err == nil {
		t.Error("unknown id accepted")
	}
}

func TestParamsDeterministic(t *testing.T) {
	d := dataset(t)
	for _, id := range IDs() {
		a, err := For(d, id, 3)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		b, _ := For(d, id, 3)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("%s: params differ between calls", id)
		}
	}
}

func TestParamsInRange(t *testing.T) {
	d := dataset(t)
	for _, id := range IDs() {
		for it := int64(0); it < 100; it++ {
			p, err := For(d, id, it)
			if err != nil {
				t.Fatalf("%s/%d: %v", id, it, err)
			}
			if p.ReservationID != 0 && p.ReservationID < 1_000_000_000_000 && p.ReservationID > d.Reservations {
				t.Fatalf("%s: reservation %d out of range", id, p.ReservationID)
			}
			if p.CustomerID > d.Customers {
				t.Fatalf("%s: customer %d out of range", id, p.CustomerID)
			}
		}
	}
	if _, err := For(d, "R1", MaxIteration); err == nil {
		t.Fatal("iteration bound not enforced")
	}
}

// Iterations of the same booking scenario must not reuse tickets, otherwise
// later iterations would find the seat taken and do less work.
func TestBookingTicketsDistinct(t *testing.T) {
	d := dataset(t)
	seen := map[int64]string{}
	for _, id := range []ID{"C1", "C2", "U2"} {
		for it := int64(0); it < 100; it++ {
			p, _ := For(d, id, it)
			for _, tk := range p.TicketIDs {
				if prev, ok := seen[tk]; ok {
					t.Fatalf("ticket %d used by %s and %s/%d", tk, prev, id, it)
				}
				seen[tk] = string(id)
			}
		}
	}
	hot, err := HotTickets(d, 500)
	if err != nil {
		t.Fatal(err)
	}
	for _, tk := range hot {
		if _, ok := seen[tk]; ok {
			t.Fatalf("hot ticket %d also used by a scenario", tk)
		}
	}
}

func TestFixtures(t *testing.T) {
	d := dataset(t)
	ids := map[int64]bool{}
	for _, id := range []ID{"C3", "U6", "D3", "D5", "D6"} {
		for it := int64(0); it < 3; it++ {
			p, _ := For(d, id, it)
			all := append([]model.Ticket{}, p.Fixture.Tickets...)
			all = append(all, p.Tickets...)
			for _, tk := range all {
				if ids[tk.ID] {
					t.Fatalf("%s/%d: duplicate fixture ticket id %d", id, it, tk.ID)
				}
				ids[tk.ID] = true
				if tk.ID <= d.Spec.Tickets {
					t.Fatalf("%s: fixture ticket id %d overlaps generated data", id, tk.ID)
				}
			}
		}
	}
	p, _ := For(d, "C3", 0)
	if len(p.Tickets) != BulkTickets {
		t.Fatalf("C3 imports %d tickets, want %d", len(p.Tickets), BulkTickets)
	}
	p, _ = For(d, "U6", 0)
	if len(p.Fixture.Reservations) != BulkReservations {
		t.Fatalf("U6 prepares %d reservations", len(p.Fixture.Reservations))
	}
	for _, r := range p.Fixture.Reservations {
		if !r.ExpiresAt.Before(p.AsOf) {
			t.Fatal("U6 fixture must be expired at AsOf")
		}
	}
}

func BenchmarkParams(b *testing.B) {
	d := dataset(b)
	ids := IDs()
	for i := 0; b.Loop(); i++ {
		if _, err := For(d, ids[i%len(ids)], int64(i%MaxIteration)); err != nil {
			b.Fatal(err)
		}
	}
}
