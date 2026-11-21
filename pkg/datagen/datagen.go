// Package datagen generates the synthetic ticketing dataset.
//
// The generator is deterministic: every entity is derived from (seed, kind, id)
// so each store can generate its own copy of the data locally and all four
// databases end up with identical rows. Sizes are expressed in tickets, which
// are the inventory of seats offered for events (free, held, reserved, sold).
package datagen

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"time"

	"github.com/ryczek02/ticketbench-contracts/pkg/model"
)

// T0 is the reference "now" of the dataset. Scenarios use it instead of the
// wall clock so that results do not depend on the day the benchmark runs.
var T0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// Spec selects the dataset.
type Spec struct {
	Tickets int64
	Seed    uint64
}

// MinTickets is the smallest dataset that still has sandbox events.
const MinTickets = 5000

// golden is the 64-bit golden ratio constant used to spread seeds per kind.
var golden uint64 = 0x9E3779B97F4A7C15

// SandboxEvents is the number of events kept free of sales; scenarios that
// book or hold seats work on them so they never collide with loaded data.
const SandboxEvents = 4

const (
	kVenue uint64 = iota + 1
	kEvent
	kGroup
	kTicket
	kReservation
	kCustomer
	kOrganizer
	kDiscount
)

// Dataset is the in-memory layout of a generated dataset. Large tables
// (seats, tickets, reservations, customers) are produced on demand.
type Dataset struct {
	Spec       Spec
	Categories []model.Category
	Organizers []model.Organizer
	Venues     []model.Venue
	Events     []model.Event
	Discounts  []model.DiscountCode

	Customers    int64
	Seats        int64
	Sectors      int64
	Prices       int64
	Reservations int64
	// Sandbox holds indexes into Events.
	Sandbox []int

	resOffset []int64
	resCount  []int32
}

func (d *Dataset) rng(kind, id uint64) *rand.Rand {
	return rand.New(rand.NewPCG(d.Spec.Seed^(kind*golden), id*0xBF58476D1CE4E5B9+kind))
}

// New builds the dataset layout. It runs in O(events + tickets) time and
// O(events + venues) memory.
func New(spec Spec) (*Dataset, error) {
	if spec.Tickets < MinTickets {
		return nil, fmt.Errorf("datagen: need at least %d tickets, got %d", MinTickets, spec.Tickets)
	}
	d := &Dataset{Spec: spec}
	for i, name := range categories {
		d.Categories = append(d.Categories, model.Category{ID: int64(i + 1), Name: name})
	}
	d.buildVenues()
	d.buildEvents()
	d.buildOrganizers()
	d.assignEventOwners()
	d.buildDiscounts()
	d.Customers = max(500, spec.Tickets/8)
	d.countReservations()
	return d, nil
}

func (d *Dataset) buildVenues() {
	n := max(8, int(d.Spec.Tickets/60000))
	var seatOffset, sectorID int64
	for i := range n {
		r := d.rng(kVenue, uint64(i+1))
		city := cities[int(float64(len(cities))*r.Float64()*r.Float64())]
		kind := venueKinds[r.IntN(len(venueKinds))]
		v := model.Venue{
			ID:      int64(i + 1),
			Name:    fmt.Sprintf("%s %s %d", kind, city, i+1),
			City:    city,
			Address: fmt.Sprintf("ul. %s %d", streets[r.IntN(len(streets))], 1+r.IntN(120)),
			Meta: model.VenueMeta{
				Parking:    r.IntN(3) > 0,
				Accessible: r.IntN(5) > 0,
				Amenities:  pick(r, amenities, 1+r.IntN(4)),
			},
			SeatOffset: seatOffset,
		}
		sectors := 5 + r.IntN(6)
		for s := range sectors {
			sectorID++
			sec := model.Sector{
				ID:         sectorID,
				VenueID:    v.ID,
				Name:       string(rune('A' + s)),
				Kind:       sectorKinds[r.IntN(len(sectorKinds))],
				Rows:       8 + r.IntN(13),
				PerRow:     15 + r.IntN(16),
				Multiplier: math.Round((1+1.5*r.Float64())*100) / 100,
			}
			v.Capacity += sec.Seats()
			v.Sectors = append(v.Sectors, sec)
		}
		seatOffset += int64(v.Capacity)
		d.Venues = append(d.Venues, v)
	}
	d.Seats = seatOffset
	d.Sectors = sectorID
}

func (d *Dataset) buildEvents() {
	var off, priceID int64
	id := int64(0)
	for off < d.Spec.Tickets {
		id++
		r := d.rng(kEvent, uint64(id))
		v := &d.Venues[r.IntN(len(d.Venues))]
		count := min(int64(v.Capacity), d.Spec.Tickets-off)
		cat := r.IntN(len(categories))
		day := r.IntN(545) - 180
		starts := T0.AddDate(0, 0, day).Add(time.Duration(17+r.IntN(5)) * time.Hour)
		perf := performerA[r.IntN(len(performerA))] + " " + performerB[r.IntN(len(performerB))]
		ev := model.Event{
			ID:         id,
			CategoryID: int64(cat + 1),
			VenueID:    v.ID,
			Name:       fmt.Sprintf("%s: %s #%d", categories[cat], perf, id),
			StartsAt:   starts,
			Status:     model.EventPlanned,
			Meta: model.EventMeta{
				Genre:      genres[cat][r.IntN(len(genres[cat]))],
				Performers: []string{perf},
				AgeLimit:   []int{0, 0, 7, 12, 16, 18}[r.IntN(6)],
				Tags:       pick(r, tags, r.IntN(3)),
			},
			TicketOffset: off,
			TicketCount:  int(count),
		}
		if r.IntN(100) < 3 {
			ev.Status = model.EventCancelled
		}
		base := basePrice[cat] * (0.7 + 0.6*r.Float64())
		for _, s := range v.Sectors {
			priceID++
			ev.Prices = append(ev.Prices, model.Price{
				ID:       priceID,
				EventID:  id,
				SectorID: s.ID,
				Price:    math.Round(base*s.Multiplier*100) / 100,
				Currency: "PLN",
			})
		}
		off += count
		d.Events = append(d.Events, ev)
	}
	d.Prices = priceID
	// Sandbox events: evenly spaced, never the last (possibly truncated) one.
	n := len(d.Events)
	step := max(1, (n-1)/SandboxEvents)
	for i := 0; i < SandboxEvents && i*step < n-1; i++ {
		idx := i * step
		d.Events[idx].Sandbox = true
		d.Events[idx].Status = model.EventPlanned
		d.Events[idx].StartsAt = T0.AddDate(0, 6, i)
		d.Sandbox = append(d.Sandbox, idx)
	}
}

func (d *Dataset) buildOrganizers() {
	n := max(5, len(d.Events)/25)
	for i := range n {
		r := d.rng(kOrganizer, uint64(i+1))
		d.Organizers = append(d.Organizers, model.Organizer{
			ID:    int64(i + 1),
			Name:  fmt.Sprintf("%s %s Events %d", performerA[r.IntN(len(performerA))], cities[r.IntN(len(cities))], i+1),
			TaxID: fmt.Sprintf("%010d", r.Int64N(1e10)),
			Email: fmt.Sprintf("biuro%d@organizator.example", i+1),
		})
	}
}

func (d *Dataset) assignEventOwners() {
	for i := range d.Events {
		r := d.rng(kEvent, uint64(d.Events[i].ID)+1<<40)
		d.Events[i].OrganizerID = int64(1 + r.IntN(len(d.Organizers)))
	}
}

func (d *Dataset) buildDiscounts() {
	n := max(20, len(d.Events)/5)
	for i := range n {
		r := d.rng(kDiscount, uint64(i+1))
		from := T0.AddDate(0, 0, -r.IntN(365))
		d.Discounts = append(d.Discounts, model.DiscountCode{
			ID:         int64(i + 1),
			Code:       fmt.Sprintf("RABAT%05d", i+1),
			Percent:    []int{5, 10, 15, 20, 25}[r.IntN(5)],
			ValidFrom:  from,
			ValidUntil: from.AddDate(0, 0, 30+r.IntN(300)),
			UsageLimit: 100 + r.IntN(900),
		})
	}
}

type group struct {
	start, size int
}

// groups splits the inventory of an event into contiguous reservations.
func (d *Dataset) groups(e int) []group {
	ev := &d.Events[e]
	if ev.Sandbox || ev.Status == model.EventCancelled {
		return nil
	}
	r := d.rng(kGroup, uint64(ev.ID))
	var frac float64
	if ev.StartsAt.Before(T0) {
		frac = 0.5 + 0.45*r.Float64()
	} else {
		frac = 0.2 + 0.7*r.Float64()
	}
	// average group size is 3.5 seats, so start a group with p = frac/3.5
	// per free position to sell roughly frac of the seats
	p := frac / (frac + (1-frac)*3.5)
	out := make([]group, 0, ev.TicketCount/4)
	for pos := 0; pos < ev.TicketCount; {
		if r.Float64() < p {
			size := min(1+r.IntN(6), ev.TicketCount-pos)
			out = append(out, group{start: pos, size: size})
			pos += size
		} else {
			pos++
		}
	}
	return out
}

func (d *Dataset) countReservations() {
	d.resOffset = make([]int64, len(d.Events))
	d.resCount = make([]int32, len(d.Events))
	var off int64
	for e := range d.Events {
		d.resOffset[e] = off
		n := len(d.groups(e))
		d.resCount[e] = int32(n)
		off += int64(n)
	}
	d.Reservations = off
}

// ReservationRange returns the reservation ids [first, last] of event e.
func (d *Dataset) ReservationRange(e int) (first, count int64) {
	return d.resOffset[e] + 1, int64(d.resCount[e])
}

// Customer returns customer id (1-based).
func (d *Dataset) Customer(id int64) model.Customer {
	r := d.rng(kCustomer, uint64(id))
	fn := firstNames[r.IntN(len(firstNames))]
	ln := lastNames[r.IntN(len(lastNames))]
	return model.Customer{
		ID:        id,
		FirstName: fn,
		LastName:  ln,
		Email:     fmt.Sprintf("klient%d@poczta.example", id),
		Phone:     fmt.Sprintf("+48%09d", 500000000+r.Int64N(299999999)),
		City:      cities[r.IntN(len(cities))],
		CreatedAt: T0.AddDate(0, 0, -r.IntN(1500)).Add(time.Duration(r.IntN(86400)) * time.Second),
		Prefs: model.CustomerPrefs{
			Newsletter: r.IntN(2) == 0,
			Favorite:   pickIDs(r, int64(len(categories)), r.IntN(4)),
			Language:   languages[r.IntN(len(languages))],
		},
	}
}

// VenueSeat returns the seat at position pos (0-based) of a venue.
func VenueSeat(v *model.Venue, pos int) model.Seat {
	p := pos
	for _, s := range v.Sectors {
		if p < s.Seats() {
			return model.Seat{
				ID:       v.SeatOffset + int64(pos) + 1,
				SectorID: s.ID,
				Row:      p/s.PerRow + 1,
				Number:   p%s.PerRow + 1,
			}
		}
		p -= s.Seats()
	}
	panic("datagen: seat position out of range")
}

// Seats calls fn for every seat of venue v.
func Seats(v *model.Venue, fn func(model.Seat)) {
	id := v.SeatOffset
	for _, s := range v.Sectors {
		for row := 1; row <= s.Rows; row++ {
			for n := 1; n <= s.PerRow; n++ {
				id++
				fn(model.Seat{ID: id, SectorID: s.ID, Row: row, Number: n})
			}
		}
	}
}

// Venue returns the venue of an event.
func (d *Dataset) Venue(ev *model.Event) *model.Venue {
	return &d.Venues[ev.VenueID-1]
}

// Sales holds the generated inventory and reservations of one event.
type Sales struct {
	Event        *model.Event
	Tickets      []model.Ticket
	Reservations []model.Reservation
}

// Payments returns the payments embedded in the reservations.
func (s *Sales) Payments() []model.Payment {
	var out []model.Payment
	for i := range s.Reservations {
		if p := s.Reservations[i].Payment; p != nil {
			out = append(out, *p)
		}
	}
	return out
}

// EventTickets builds the free inventory of an event placed in venue v.
// Ticket ids are offset+1 .. offset+count.
func EventTickets(ev *model.Event, v *model.Venue, offset int64, count int, seed uint64) []model.Ticket {
	prices := make(map[int64]float64, len(ev.Prices))
	for _, p := range ev.Prices {
		prices[p.SectorID] = p.Price
	}
	r := rand.New(rand.NewPCG(seed^(kTicket*golden), uint64(ev.ID)))
	out := make([]model.Ticket, 0, count)
	pos := 0
	seatID := v.SeatOffset
	for _, s := range v.Sectors {
		for row := 1; row <= s.Rows && pos < count; row++ {
			for n := 1; n <= s.PerRow && pos < count; n++ {
				pos++
				seatID++
				t := model.Ticket{
					ID:       offset + int64(pos),
					EventID:  ev.ID,
					SeatID:   seatID,
					SectorID: s.ID,
					Row:      row,
					Number:   n,
					Price:    prices[s.ID],
					Status:   model.TicketFree,
					Meta:     model.TicketMeta{Kind: "normalny"},
				}
				if r.IntN(10) == 0 {
					t.Meta.Kind = "ulgowy"
					t.Price = math.Round(t.Price*50) / 100
				}
				t.Meta.Barcode = fmt.Sprintf("%016x", r.Uint64())
				out = append(out, t)
			}
		}
	}
	return out
}

// Sales generates tickets and reservations of event index e.
func (d *Dataset) Sales(e int) Sales {
	ev := &d.Events[e]
	v := d.Venue(ev)
	tickets := EventTickets(ev, v, ev.TicketOffset, ev.TicketCount, d.Spec.Seed)
	groups := d.groups(e)
	res := make([]model.Reservation, 0, len(groups))
	past := ev.StartsAt.Before(T0)
	for gi, g := range groups {
		id := d.resOffset[e] + int64(gi) + 1
		r := d.rng(kReservation, uint64(id))
		ref := ev.StartsAt
		if ref.After(T0) {
			ref = T0
		}
		rv := model.Reservation{
			ID:         id,
			CustomerID: 1 + r.Int64N(d.Customers),
			CreatedAt:  ref.AddDate(0, 0, -r.IntN(60)).Add(-time.Duration(r.IntN(86400)) * time.Second),
			Meta: model.ReservationMeta{
				Channel: channels[r.IntN(len(channels))],
				Device:  devices[r.IntN(len(devices))],
				Source:  sources[r.IntN(len(sources))],
			},
		}
		roll := r.IntN(100)
		switch {
		case roll < 8:
			rv.Status = model.ReservationCancelled
		case past || roll < 80:
			rv.Status = model.ReservationPaid
		default:
			rv.Status = model.ReservationPending
			exp := T0.Add(time.Duration(5+r.IntN(55)) * time.Minute)
			rv.ExpiresAt = &exp
		}
		discount := 0
		if r.IntN(10) == 0 {
			dc := d.Discounts[r.IntN(len(d.Discounts))]
			rv.DiscountID = dc.ID
			discount = dc.Percent
		}
		var sum float64
		if rv.Status != model.ReservationCancelled {
			for k := g.start; k < g.start+g.size; k++ {
				t := &tickets[k]
				t.ReservationID = id
				if rv.Status == model.ReservationPaid {
					t.Status = model.TicketSold
				} else {
					t.Status = model.TicketReserved
				}
				sum += t.Price
				rv.Tickets = append(rv.Tickets, Ref(t))
			}
		}
		rv.Amount = math.Round(sum*float64(100-discount)) / 100
		if rv.Status == model.ReservationPaid {
			rv.Payment = &model.Payment{
				ID:            id,
				ReservationID: id,
				Amount:        rv.Amount,
				Method:        paymentMethods[r.IntN(len(paymentMethods))],
				Status:        model.PaymentDone,
				PaidAt:        rv.CreatedAt.Add(time.Duration(1+r.IntN(30)) * time.Minute),
				Meta: model.PaymentMeta{
					Provider:      providers[r.IntN(len(providers))],
					TransactionID: fmt.Sprintf("TX%012d", r.Int64N(1e12)),
					ThreeDS:       r.IntN(2) == 0,
				},
			}
		}
		res = append(res, rv)
	}
	return Sales{Event: ev, Tickets: tickets, Reservations: res}
}

// Ref returns the snapshot of a ticket embedded in reservations.
func Ref(t *model.Ticket) model.TicketRef {
	return model.TicketRef{TicketID: t.ID, EventID: t.EventID, SectorID: t.SectorID, Row: t.Row, Number: t.Number, Price: t.Price}
}

// Counts returns the expected number of rows per relational table.
// Payments are left out: their count is only known after walking the sales.
func (d *Dataset) Counts() map[string]int64 {
	return map[string]int64{
		"kategorie":     int64(len(d.Categories)),
		"organizatorzy": int64(len(d.Organizers)),
		"obiekty":       int64(len(d.Venues)),
		"sektory":       d.Sectors,
		"miejsca":       d.Seats,
		"wydarzenia":    int64(len(d.Events)),
		"cenniki":       d.Prices,
		"klienci":       d.Customers,
		"kody_rabatowe": int64(len(d.Discounts)),
		"rezerwacje":    d.Reservations,
		"bilety":        d.Spec.Tickets,
	}
}

// Checksum walks the whole dataset and returns a FNV-64 digest of the rows.
// Two stores that loaded the same spec report the same checksum.
func (d *Dataset) Checksum() string {
	h := fnv.New64a()
	var buf []byte
	for id := int64(1); id <= d.Customers; id++ {
		c := d.Customer(id)
		buf = fmt.Appendf(buf[:0], "c%d|%s|%s;", c.ID, c.Email, c.Phone)
		h.Write(buf)
	}
	for e := range d.Events {
		s := d.Sales(e)
		for _, t := range s.Tickets {
			buf = fmt.Appendf(buf[:0], "t%d|%s|%d|%d;", t.ID, t.Status, int64(math.Round(t.Price*100)), t.ReservationID)
			h.Write(buf)
		}
		for _, r := range s.Reservations {
			buf = fmt.Appendf(buf[:0], "r%d|%d|%s|%d;", r.ID, r.CustomerID, r.Status, int64(math.Round(r.Amount*100)))
			h.Write(buf)
		}
	}
	return fmt.Sprintf("%016x", h.Sum64())
}

func pick(r *rand.Rand, from []string, n int) []string {
	out := make([]string, 0, n)
	seen := map[int]bool{}
	for len(out) < n && len(out) < len(from) {
		i := r.IntN(len(from))
		if !seen[i] {
			seen[i] = true
			out = append(out, from[i])
		}
	}
	return out
}

func pickIDs(r *rand.Rand, max int64, n int) []int64 {
	out := make([]int64, 0, n)
	seen := map[int64]bool{}
	for len(out) < n {
		i := 1 + r.Int64N(max)
		if !seen[i] {
			seen[i] = true
			out = append(out, i)
		}
	}
	return out
}
