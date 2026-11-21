package datagen

import (
	"testing"

	"github.com/ryczek02/ticketbench-contracts/pkg/model"
)

func mustNew(t testing.TB, n int64) *Dataset {
	t.Helper()
	d, err := New(Spec{Tickets: n, Seed: 42})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestTooSmall(t *testing.T) {
	if _, err := New(Spec{Tickets: 10, Seed: 1}); err == nil {
		t.Fatal("expected error for a tiny dataset")
	}
}

func TestDeterministic(t *testing.T) {
	a, b := mustNew(t, 50_000), mustNew(t, 50_000)
	if a.Checksum() != b.Checksum() {
		t.Fatal("same spec produced different data")
	}
	c, _ := New(Spec{Tickets: 50_000, Seed: 43})
	if a.Checksum() == c.Checksum() {
		t.Fatal("different seeds produced the same data")
	}
}

func TestTicketCountAndIDs(t *testing.T) {
	for _, n := range []int64{MinTickets, 20_000, 123_457} {
		d := mustNew(t, n)
		var total int64
		next := int64(1)
		for e := range d.Events {
			s := d.Sales(e)
			for _, tk := range s.Tickets {
				if tk.ID != next {
					t.Fatalf("n=%d: ticket id %d, want %d", n, tk.ID, next)
				}
				next++
			}
			total += int64(len(s.Tickets))
		}
		if total != n {
			t.Fatalf("generated %d tickets, want %d", total, n)
		}
	}
}

func TestSalesConsistency(t *testing.T) {
	d := mustNew(t, 60_000)
	var reservations int64
	statuses := map[string]int{}
	for e := range d.Events {
		s := d.Sales(e)
		first, count := d.ReservationRange(e)
		if int64(len(s.Reservations)) != count {
			t.Fatalf("event %d: %d reservations, layout says %d", e, len(s.Reservations), count)
		}
		byID := map[int64]*model.Reservation{}
		for i := range s.Reservations {
			r := &s.Reservations[i]
			if r.ID != first+int64(i) {
				t.Fatalf("reservation id %d, want %d", r.ID, first+int64(i))
			}
			if r.CustomerID < 1 || r.CustomerID > d.Customers {
				t.Fatalf("customer id %d out of range", r.CustomerID)
			}
			if (r.Payment != nil) != (r.Status == model.ReservationPaid) {
				t.Fatalf("reservation %d: payment presence does not match status %s", r.ID, r.Status)
			}
			if r.Status == model.ReservationPending && (r.ExpiresAt == nil || !r.ExpiresAt.After(T0)) {
				t.Fatalf("pending reservation %d must expire after T0", r.ID)
			}
			statuses[r.Status]++
			byID[r.ID] = r
		}
		reservations += count
		for _, tk := range s.Tickets {
			if tk.ReservationID == 0 {
				if tk.Status != model.TicketFree {
					t.Fatalf("ticket %d has status %s but no reservation", tk.ID, tk.Status)
				}
				continue
			}
			r := byID[tk.ReservationID]
			if r == nil {
				t.Fatalf("ticket %d points to reservation %d of another event", tk.ID, tk.ReservationID)
			}
			want := model.TicketReserved
			if r.Status == model.ReservationPaid {
				want = model.TicketSold
			}
			if tk.Status != want {
				t.Fatalf("ticket %d status %s, reservation status %s", tk.ID, tk.Status, r.Status)
			}
		}
		if d.Events[e].Sandbox && len(s.Reservations) != 0 {
			t.Fatalf("sandbox event %d has sales", e)
		}
	}
	if reservations != d.Reservations {
		t.Fatalf("total reservations %d, layout says %d", reservations, d.Reservations)
	}
	for _, st := range []string{model.ReservationPaid, model.ReservationPending, model.ReservationCancelled} {
		if statuses[st] == 0 {
			t.Errorf("no reservation with status %s", st)
		}
	}
}

func TestSandbox(t *testing.T) {
	d := mustNew(t, 200_000)
	if len(d.Sandbox) != SandboxEvents {
		t.Fatalf("got %d sandbox events, want %d", len(d.Sandbox), SandboxEvents)
	}
	for _, e := range d.Sandbox {
		ev := d.Events[e]
		if ev.TicketCount != d.Venue(&ev).Capacity {
			t.Fatalf("sandbox event %d is truncated", e)
		}
		if ev.TicketCount < 600 {
			t.Fatalf("sandbox event %d too small for the contention test", e)
		}
	}
}

func TestVenueSeat(t *testing.T) {
	d := mustNew(t, MinTickets)
	v := &d.Venues[0]
	i := 0
	Seats(v, func(s model.Seat) {
		got := VenueSeat(v, i)
		if got != s {
			t.Fatalf("seat %d: VenueSeat %+v, Seats %+v", i, got, s)
		}
		i++
	})
	if i != v.Capacity {
		t.Fatalf("walked %d seats, capacity %d", i, v.Capacity)
	}
}

func TestCustomerStable(t *testing.T) {
	d := mustNew(t, MinTickets)
	a, b := d.Customer(7), d.Customer(7)
	if a.Email != b.Email || a.Phone != b.Phone || a.CreatedAt != b.CreatedAt {
		t.Fatal("customer is not deterministic")
	}
}

func BenchmarkNew1M(b *testing.B) {
	for b.Loop() {
		if _, err := New(Spec{Tickets: 1_000_000, Seed: 42}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSalesPerEvent(b *testing.B) {
	d := mustNew(b, 1_000_000)
	b.ResetTimer()
	var tickets int
	for i := 0; b.Loop(); i++ {
		s := d.Sales(i % len(d.Events))
		tickets += len(s.Tickets)
	}
	b.ReportMetric(float64(tickets)/b.Elapsed().Seconds(), "tickets/s")
}

func BenchmarkCustomer(b *testing.B) {
	d := mustNew(b, 1_000_000)
	for i := int64(0); b.Loop(); i++ {
		_ = d.Customer(1 + i%d.Customers)
	}
}

func BenchmarkChecksum100k(b *testing.B) {
	d := mustNew(b, 100_000)
	for b.Loop() {
		_ = d.Checksum()
	}
}
