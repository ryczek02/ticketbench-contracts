// Package scenario defines the benchmark scenarios and derives their
// parameters from the dataset. Parameters depend only on (dataset, scenario,
// iteration), so every store executes exactly the same operations.
package scenario

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/ryczek02/ticketbench-contracts/pkg/datagen"
	"github.com/ryczek02/ticketbench-contracts/pkg/model"
)

// ID identifies a scenario, for example "C1" or "R6".
type ID string

// Op is the CRUD group of a scenario. Extra scenarios on JSON data use "J".
type Op string

const (
	Create Op = "C"
	Read   Op = "R"
	Update Op = "U"
	Delete Op = "D"
	JSON   Op = "J"
)

type Def struct {
	ID   ID
	Op   Op
	Name string // Polish name used in the report
	// Fixture is true when Prepare inserts data that Run then works on.
	Fixture bool
}

// All lists the 24 CRUD scenarios from the project brief plus 3 JSON ones.
var All = []Def{
	{"C1", Create, "Pojedyncza rezerwacja", false},
	{"C2", Create, "Rezerwacja z kilkoma biletami", false},
	{"C3", Create, "Masowy import biletów", true},
	{"C4", Create, "Rejestracja klienta", false},
	{"C5", Create, "Wydarzenie z cennikiem", false},
	{"C6", Create, "Zapis płatności", true},
	{"R1", Read, "Rezerwacja po identyfikatorze", false},
	{"R2", Read, "Wolne miejsca na wydarzenie", false},
	{"R3", Read, "Wydarzenia po dacie i mieście", false},
	{"R4", Read, "Historia rezerwacji klienta", false},
	{"R5", Read, "Sprzedaż per wydarzenie (agregacja)", false},
	{"R6", Read, "10 najpopularniejszych wydarzeń", false},
	{"U1", Update, "Zmiana statusu rezerwacji", false},
	{"U2", Update, "Zajęcie miejsca (blokada)", false},
	{"U3", Update, "Masowa zmiana cen w sektorze", false},
	{"U4", Update, "Zmiana danych klienta", false},
	{"U5", Update, "Zmiana terminu wydarzenia", false},
	{"U6", Update, "Wygaszenie nieopłaconych rezerwacji", true},
	{"D1", Delete, "Anulowanie rezerwacji", false},
	{"D2", Delete, "Usunięcie rezerwacji wraz z biletami", false},
	{"D3", Delete, "Masowe usunięcie wygasłych rezerwacji", true},
	{"D4", Delete, "Usunięcie konta klienta", false},
	{"D5", Delete, "Usunięcie odwołanego wydarzenia", true},
	{"D6", Delete, "Usunięcie danych starszych niż data", true},
	{"J1", JSON, "Filtr po polu JSON (gatunek i tag)", false},
	{"J2", JSON, "Agregacja po polu JSON (kanał sprzedaży)", false},
	{"J3", JSON, "Modyfikacja tablicy w JSON (tagi wydarzenia)", false},
}

// CRUD returns the ids of the 24 CRUD scenarios.
func CRUD() []ID {
	var out []ID
	for _, d := range All {
		if d.Op != JSON {
			out = append(out, d.ID)
		}
	}
	return out
}

// IDs returns every scenario id.
func IDs() []ID {
	out := make([]ID, len(All))
	for i, d := range All {
		out[i] = d.ID
	}
	return out
}

// Lookup returns the definition of id.
func Lookup(id ID) (Def, bool) {
	for _, d := range All {
		if d.ID == id {
			return d, true
		}
	}
	return Def{}, false
}

// Parse turns "all", "crud", "json", "C*,R1" into a list of ids.
func Parse(spec string) ([]ID, error) {
	var out []ID
	for _, part := range strings.Split(spec, ",") {
		part = strings.ToUpper(strings.TrimSpace(part))
		switch {
		case part == "":
		case part == "ALL":
			out = append(out, IDs()...)
		case part == "CRUD":
			out = append(out, CRUD()...)
		case len(part) == 2 && part[1] == '*':
			for _, d := range All {
				if string(d.Op) == part[:1] {
					out = append(out, d.ID)
				}
			}
		default:
			if _, ok := Lookup(ID(part)); !ok {
				return nil, fmt.Errorf("unknown scenario %q", part)
			}
			out = append(out, ID(part))
		}
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

func index(id ID) int64 {
	for i, d := range All {
		if d.ID == id {
			return int64(i + 1)
		}
	}
	return 99
}

// MaxIteration bounds iteration numbers; larger values would make fixture
// ids of different iterations overlap.
const MaxIteration = 1000

// FixtureID returns an id in the space reserved for scenario fixtures. It is
// far above anything the generator produces.
func FixtureID(id ID, iteration, k int64) int64 {
	return 1_000_000_000_000 + index(id)*10_000_000_000 + iteration*10_000_000 + k
}

// ContentionReservationBase is the first reservation id used by the
// contention test. Stores delete everything above it before each round.
const ContentionReservationBase int64 = 2_000_000_000_000

// Batch sizes of bulk scenarios.
const (
	BulkTickets      = 10_000 // C3
	BulkReservations = 1_000  // U6, D3, D6
)

// Params are the inputs of one scenario iteration.
type Params struct {
	ID        ID
	Iteration int64
	AsOf      time.Time

	ReservationID int64
	CustomerID    int64
	EventID       int64
	SectorID      int64
	OrganizerID   int64
	TicketIDs     []int64

	City     string
	Genre    string
	Tag      string
	From, To time.Time
	Cutoff   time.Time
	NewDate  time.Time
	Factor   float64

	Reservation *model.Reservation // C1, C2: reservation to create (tickets filled)
	Customer    *model.Customer    // C4: new customer, U4: new contact data
	Event       *model.Event       // C5: new event with price list
	Payment     *model.Payment     // C6
	Tickets     []model.Ticket     // C3: bulk inventory

	// Fixture is inserted by Prepare before the timed Run.
	Fixture Fixture
}

// Fixture is scenario-owned data inserted before the measured operation.
type Fixture struct {
	Events       []model.Event
	Tickets      []model.Ticket
	Reservations []model.Reservation
}

func (f Fixture) Empty() bool {
	return len(f.Events) == 0 && len(f.Tickets) == 0 && len(f.Reservations) == 0
}

func (p Params) rngFor(d *datagen.Dataset) *rand.Rand {
	h := fnv.New64a()
	h.Write([]byte(p.ID))
	return rand.New(rand.NewPCG(d.Spec.Seed^h.Sum64(), uint64(p.Iteration)+1))
}

// regularEvent returns a random event index that is not a sandbox event.
func regularEvent(d *datagen.Dataset, r *rand.Rand) int {
	for {
		e := r.IntN(len(d.Events))
		if !d.Events[e].Sandbox {
			return e
		}
	}
}

// SandboxTickets returns the free inventory of sandbox event k.
func SandboxTickets(d *datagen.Dataset, k int) []model.Ticket {
	ev := &d.Events[d.Sandbox[k%len(d.Sandbox)]]
	return datagen.EventTickets(ev, d.Venue(ev), ev.TicketOffset, ev.TicketCount, d.Spec.Seed)
}

// HotTickets returns the ticket ids used by the contention test.
func HotTickets(d *datagen.Dataset, n int) ([]int64, error) {
	ev := &d.Events[d.Sandbox[0]]
	if n > ev.TicketCount {
		return nil, fmt.Errorf("hot tickets %d exceed sandbox event capacity %d", n, ev.TicketCount)
	}
	out := make([]int64, n)
	for i := range out {
		out[i] = ev.TicketOffset + int64(i) + 1
	}
	return out, nil
}

// HotEventID returns the event used by the contention test.
func HotEventID(d *datagen.Dataset) int64 { return d.Events[d.Sandbox[0]].ID }

// fixtureEvents creates events in venue 1 holding at least n free tickets.
func fixtureEvents(d *datagen.Dataset, id ID, iter int64, n int, startsAt time.Time, status string) ([]model.Event, []model.Ticket) {
	v := &d.Venues[0]
	var events []model.Event
	var tickets []model.Ticket
	for k := int64(0); len(tickets) < n; k++ {
		ev := fixtureEvent(d, id, iter, k, startsAt, status)
		count := min(v.Capacity, n-len(tickets))
		base := FixtureID(id, iter, 100_000+int64(len(tickets)))
		tickets = append(tickets, datagen.EventTickets(&ev, v, base-1, count, d.Spec.Seed)...)
		events = append(events, ev)
	}
	return events, tickets
}

func fixtureEvent(d *datagen.Dataset, id ID, iter, k int64, startsAt time.Time, status string) model.Event {
	v := &d.Venues[0]
	evID := FixtureID(id, iter, k)
	ev := model.Event{
		ID:          evID,
		OrganizerID: 1,
		CategoryID:  1,
		VenueID:     v.ID,
		Name:        fmt.Sprintf("fixture %s/%d/%d", id, iter, k),
		StartsAt:    startsAt,
		Status:      status,
		Meta:        model.EventMeta{Genre: "rock", Performers: []string{"Fixture"}, Tags: []string{"fixture"}},
	}
	for i, s := range v.Sectors {
		ev.Prices = append(ev.Prices, model.Price{
			ID:       FixtureID(id, iter, 50_000+k*100+int64(i)),
			EventID:  evID,
			SectorID: s.ID,
			Price:    100 * s.Multiplier,
			Currency: "PLN",
		})
	}
	return ev
}

func reservationFor(id int64, customer int64, created time.Time, status string, tickets []model.Ticket) model.Reservation {
	rv := model.Reservation{
		ID:         id,
		CustomerID: customer,
		CreatedAt:  created,
		Status:     status,
		Meta:       model.ReservationMeta{Channel: "web", Device: "linux", Source: "bezposrednio"},
	}
	for i := range tickets {
		rv.Amount += tickets[i].Price
		rv.Tickets = append(rv.Tickets, datagen.Ref(&tickets[i]))
	}
	return rv
}

// For derives the parameters of scenario id at the given iteration.
func For(d *datagen.Dataset, id ID, iteration int64) (Params, error) {
	if iteration < 0 || iteration >= MaxIteration {
		return Params{}, fmt.Errorf("iteration %d out of range [0,%d)", iteration, MaxIteration)
	}
	p := Params{ID: id, Iteration: iteration, AsOf: datagen.T0}
	r := p.rngFor(d)
	randReservation := func() int64 { return 1 + r.Int64N(d.Reservations) }
	randCustomer := func() int64 { return 1 + r.Int64N(d.Customers) }

	switch id {
	case "C1", "C2":
		k, per := 1, 1
		if id == "C2" {
			k, per = 2, 4
		}
		inv := SandboxTickets(d, k)
		// C1 uses the lower half of sandbox event 1 (U2 holds the upper
		// half), C2 walks the whole sandbox event 2 in steps of 4 seats.
		span := len(inv) / 2
		if id == "C2" {
			span = len(inv) - per
		}
		start := int(iteration) * per % span
		tickets := inv[start : start+per]
		rv := reservationFor(FixtureID(id, iteration, 0), randCustomer(), datagen.T0, model.ReservationPending, tickets)
		exp := datagen.T0.Add(15 * time.Minute)
		rv.ExpiresAt = &exp
		p.Reservation = &rv
		for _, t := range tickets {
			p.TicketIDs = append(p.TicketIDs, t.ID)
		}
	case "C3":
		events, tickets := fixtureEvents(d, id, iteration, BulkTickets, datagen.T0.AddDate(0, 3, 0), model.EventPlanned)
		p.Fixture.Events = events
		p.Tickets = tickets
	case "C4":
		c := d.Customer(1 + int64(iteration)%d.Customers)
		c.ID = FixtureID(id, iteration, 0)
		c.Email = fmt.Sprintf("nowy%d.%d@poczta.example", iteration, r.IntN(1e6))
		c.CreatedAt = datagen.T0
		p.Customer = &c
	case "C5":
		ev := fixtureEvent(d, id, iteration, 0, datagen.T0.AddDate(0, 2, 0), model.EventPlanned)
		p.Event = &ev
	case "C6":
		inv := SandboxTickets(d, 3)[:2]
		rv := reservationFor(FixtureID(id, iteration, 0), randCustomer(), datagen.T0, model.ReservationPending, nil)
		rv.Amount = inv[0].Price + inv[1].Price
		p.Fixture.Reservations = []model.Reservation{rv}
		p.ReservationID = rv.ID
		p.Payment = &model.Payment{
			ID: rv.ID, ReservationID: rv.ID, Amount: rv.Amount, Method: "blik",
			Status: model.PaymentDone, PaidAt: datagen.T0.Add(3 * time.Minute),
			Meta: model.PaymentMeta{Provider: "PayU", TransactionID: fmt.Sprintf("TXF%d", rv.ID), ThreeDS: true},
		}
	case "R1", "U1", "D1", "D2":
		p.ReservationID = randReservation()
	case "R2":
		p.EventID = d.Events[regularEvent(d, r)].ID
	case "R3":
		p.City = d.Venues[r.IntN(len(d.Venues))].City
		p.From = datagen.T0.AddDate(0, 0, r.IntN(500)-180)
		p.To = p.From.AddDate(0, 0, 30)
	case "R4", "D4":
		p.CustomerID = randCustomer()
	case "R5":
		p.OrganizerID = 1 + r.Int64N(int64(len(d.Organizers)))
	case "R6":
	case "U2":
		inv := SandboxTickets(d, 1)
		t := inv[len(inv)/2+int(iteration)%(len(inv)/2)]
		p.TicketIDs = []int64{t.ID}
		p.EventID = t.EventID
		p.NewDate = datagen.T0.Add(10 * time.Minute) // held until
	case "U3":
		e := regularEvent(d, r)
		ev := &d.Events[e]
		v := d.Venue(ev)
		p.EventID = ev.ID
		p.SectorID = v.Sectors[r.IntN(len(v.Sectors))].ID
		p.Factor = 1.1
	case "U4":
		p.CustomerID = randCustomer()
		c := model.Customer{
			ID:    p.CustomerID,
			Email: fmt.Sprintf("zmieniony%d.%d@poczta.example", p.CustomerID, iteration),
			Phone: fmt.Sprintf("+48%09d", 600000000+r.Int64N(99999999)),
			Prefs: model.CustomerPrefs{Language: "en", Newsletter: true},
		}
		p.Customer = &c
	case "U5":
		ev := &d.Events[regularEvent(d, r)]
		p.EventID = ev.ID
		p.NewDate = ev.StartsAt.AddDate(0, 0, 7)
	case "U6":
		events, tickets := fixtureEvents(d, id, iteration, BulkReservations, datagen.T0.AddDate(0, 1, 0), model.EventPlanned)
		exp := datagen.T0.Add(-time.Hour)
		var res []model.Reservation
		for i := range tickets {
			rv := reservationFor(FixtureID(id, iteration, 500_000+int64(i)), randCustomer(), exp.Add(-15*time.Minute), model.ReservationPending, tickets[i:i+1])
			rv.ExpiresAt = &exp
			tickets[i].Status = model.TicketReserved
			tickets[i].ReservationID = rv.ID
			res = append(res, rv)
		}
		p.Fixture = Fixture{Events: events, Tickets: tickets, Reservations: res}
	case "D3":
		exp := datagen.T0.AddDate(0, 0, -30)
		var res []model.Reservation
		for i := range BulkReservations {
			rv := reservationFor(FixtureID(id, iteration, 500_000+int64(i)), randCustomer(), exp.Add(-15*time.Minute), model.ReservationExpired, nil)
			rv.ExpiresAt = &exp
			rv.Amount = 99
			res = append(res, rv)
		}
		p.Fixture.Reservations = res
		p.Cutoff = datagen.T0.AddDate(0, 0, -7)
	case "D5":
		events, tickets := fixtureEvents(d, id, iteration, d.Venues[0].Capacity, datagen.T0.AddDate(0, 1, 0), model.EventCancelled)
		p.Fixture = Fixture{Events: events, Tickets: tickets}
		p.EventID = events[0].ID
	case "D6":
		old := datagen.T0.AddDate(-3, 0, 0)
		events, tickets := fixtureEvents(d, id, iteration, BulkReservations*2, old, model.EventPlanned)
		var res []model.Reservation
		for i := 0; i+1 < len(tickets); i += 2 {
			rv := reservationFor(FixtureID(id, iteration, 500_000+int64(i)), randCustomer(), old.AddDate(0, 0, -10), model.ReservationPaid, tickets[i:i+2])
			rv.Payment = &model.Payment{ID: rv.ID, ReservationID: rv.ID, Amount: rv.Amount, Method: "karta", Status: model.PaymentDone, PaidAt: rv.CreatedAt}
			for k := i; k < i+2; k++ {
				tickets[k].Status = model.TicketSold
				tickets[k].ReservationID = rv.ID
			}
			res = append(res, rv)
		}
		p.Fixture = Fixture{Events: events, Tickets: tickets, Reservations: res}
		p.Cutoff = datagen.T0.AddDate(-2, 0, 0)
	case "J1":
		ev := &d.Events[regularEvent(d, r)]
		p.Genre = ev.Meta.Genre
		all := datagen.Tags()
		p.Tag = all[r.IntN(len(all))]
	case "J2":
		p.From = datagen.T0.AddDate(0, 0, -r.IntN(200))
		p.To = p.From.AddDate(0, 0, 7)
	case "J3":
		p.EventID = d.Events[regularEvent(d, r)].ID
		p.Tag = fmt.Sprintf("promocja-%d", iteration)
	default:
		return Params{}, fmt.Errorf("unknown scenario %q", id)
	}
	return p, nil
}
