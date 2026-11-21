// Package model holds the domain types of the ticket reservation system.
//
// The same structs feed every store: relational adapters map them to the
// 12 normalized tables, MongoDB embeds them into documents and Valkey stores
// them as hashes and JSON strings. JSON and BSON field names are Polish to
// match the relational column names.
package model

import "time"

// Ticket statuses.
const (
	TicketFree     = "wolny"
	TicketHeld     = "zablokowany"
	TicketReserved = "zarezerwowany"
	TicketSold     = "sprzedany"
)

// Reservation statuses.
const (
	ReservationPending   = "oczekujaca"
	ReservationPaid      = "oplacona"
	ReservationCancelled = "anulowana"
	ReservationExpired   = "wygasla"
)

// Event statuses.
const (
	EventPlanned   = "planowane"
	EventCancelled = "odwolane"
)

// Payment statuses.
const (
	PaymentDone     = "zakonczona"
	PaymentRejected = "odrzucona"
)

type Category struct {
	ID   int64  `json:"id" bson:"_id"`
	Name string `json:"nazwa" bson:"nazwa"`
}

type Organizer struct {
	ID    int64  `json:"id" bson:"_id"`
	Name  string `json:"nazwa" bson:"nazwa"`
	TaxID string `json:"nip" bson:"nip"`
	Email string `json:"email" bson:"email"`
}

type VenueMeta struct {
	Parking    bool     `json:"parking" bson:"parking"`
	Accessible bool     `json:"dla_niepelnosprawnych" bson:"dla_niepelnosprawnych"`
	Amenities  []string `json:"udogodnienia" bson:"udogodnienia"`
}

type Venue struct {
	ID       int64     `json:"id" bson:"_id"`
	Name     string    `json:"nazwa" bson:"nazwa"`
	City     string    `json:"miasto" bson:"miasto"`
	Address  string    `json:"adres" bson:"adres"`
	Capacity int       `json:"pojemnosc" bson:"pojemnosc"`
	Meta     VenueMeta `json:"metadane" bson:"metadane"`
	Sectors  []Sector  `json:"sektory" bson:"sektory"`
	// SeatOffset is the id of the first seat of this venue minus one.
	SeatOffset int64 `json:"-" bson:"-"`
}

type Sector struct {
	ID      int64  `json:"id" bson:"id"`
	VenueID int64  `json:"obiekt_id" bson:"obiekt_id"`
	Name    string `json:"nazwa" bson:"nazwa"`
	Kind    string `json:"typ" bson:"typ"`
	Rows    int    `json:"rzedy" bson:"rzedy"`
	PerRow  int    `json:"miejsc_w_rzedzie" bson:"miejsc_w_rzedzie"`
	// Multiplier scales the base price of an event for this sector.
	Multiplier float64 `json:"-" bson:"-"`
}

// Seats returns the number of seats in the sector.
func (s Sector) Seats() int { return s.Rows * s.PerRow }

type Seat struct {
	ID       int64 `json:"id" bson:"_id"`
	SectorID int64 `json:"sektor_id" bson:"sektor_id"`
	Row      int   `json:"rzad" bson:"rzad"`
	Number   int   `json:"numer" bson:"numer"`
}

type EventMeta struct {
	Genre      string   `json:"gatunek" bson:"gatunek"`
	Performers []string `json:"wykonawcy" bson:"wykonawcy"`
	AgeLimit   int      `json:"limit_wiekowy" bson:"limit_wiekowy"`
	Tags       []string `json:"tagi" bson:"tagi"`
}

type Price struct {
	ID       int64   `json:"id" bson:"id"`
	EventID  int64   `json:"wydarzenie_id" bson:"-"`
	SectorID int64   `json:"sektor_id" bson:"sektor_id"`
	Price    float64 `json:"cena" bson:"cena"`
	Currency string  `json:"waluta" bson:"waluta"`
}

type Event struct {
	ID          int64     `json:"id" bson:"_id"`
	OrganizerID int64     `json:"organizator_id" bson:"organizator_id"`
	CategoryID  int64     `json:"kategoria_id" bson:"kategoria_id"`
	VenueID     int64     `json:"obiekt_id" bson:"obiekt_id"`
	Name        string    `json:"nazwa" bson:"nazwa"`
	StartsAt    time.Time `json:"data_rozpoczecia" bson:"data_rozpoczecia"`
	Status      string    `json:"status" bson:"status"`
	Meta        EventMeta `json:"metadane" bson:"metadane"`
	Prices      []Price   `json:"cennik" bson:"cennik"`

	// Layout of the generated inventory (not persisted).
	TicketOffset int64 `json:"-" bson:"-"` // id of the first ticket minus one
	TicketCount  int   `json:"-" bson:"-"`
	Sandbox      bool  `json:"-" bson:"-"`
}

type CustomerPrefs struct {
	Newsletter bool    `json:"newsletter" bson:"newsletter"`
	Favorite   []int64 `json:"ulubione_kategorie" bson:"ulubione_kategorie"`
	Language   string  `json:"jezyk" bson:"jezyk"`
}

type Customer struct {
	ID        int64         `json:"id" bson:"_id"`
	FirstName string        `json:"imie" bson:"imie"`
	LastName  string        `json:"nazwisko" bson:"nazwisko"`
	Email     string        `json:"email" bson:"email"`
	Phone     string        `json:"telefon" bson:"telefon"`
	City      string        `json:"miasto" bson:"miasto"`
	CreatedAt time.Time     `json:"utworzono" bson:"utworzono"`
	Prefs     CustomerPrefs `json:"preferencje" bson:"preferencje"`
}

type DiscountCode struct {
	ID         int64     `json:"id" bson:"_id"`
	Code       string    `json:"kod" bson:"kod"`
	Percent    int       `json:"procent" bson:"procent"`
	ValidFrom  time.Time `json:"wazny_od" bson:"wazny_od"`
	ValidUntil time.Time `json:"wazny_do" bson:"wazny_do"`
	UsageLimit int       `json:"limit_uzyc" bson:"limit_uzyc"`
}

type TicketMeta struct {
	Kind    string `json:"rodzaj" bson:"rodzaj"`
	Barcode string `json:"kod_kreskowy" bson:"kod_kreskowy"`
}

type Ticket struct {
	ID            int64      `json:"id" bson:"_id"`
	EventID       int64      `json:"wydarzenie_id" bson:"wydarzenie_id"`
	SeatID        int64      `json:"miejsce_id" bson:"miejsce_id"`
	SectorID      int64      `json:"sektor_id" bson:"sektor_id"`
	Row           int        `json:"rzad" bson:"rzad"`
	Number        int        `json:"numer" bson:"numer"`
	Price         float64    `json:"cena" bson:"cena"`
	Status        string     `json:"status" bson:"status"`
	ReservationID int64      `json:"rezerwacja_id,omitempty" bson:"rezerwacja_id,omitempty"`
	HeldUntil     *time.Time `json:"zablokowany_do,omitempty" bson:"zablokowany_do,omitempty"`
	Meta          TicketMeta `json:"metadane" bson:"metadane"`
}

// TicketRef is the ticket snapshot embedded in a reservation document.
type TicketRef struct {
	TicketID int64   `json:"bilet_id" bson:"bilet_id"`
	EventID  int64   `json:"wydarzenie_id" bson:"wydarzenie_id"`
	SectorID int64   `json:"sektor_id" bson:"sektor_id"`
	Row      int     `json:"rzad" bson:"rzad"`
	Number   int     `json:"numer" bson:"numer"`
	Price    float64 `json:"cena" bson:"cena"`
}

type ReservationMeta struct {
	Channel string `json:"kanal" bson:"kanal"`
	Device  string `json:"urzadzenie" bson:"urzadzenie"`
	Source  string `json:"zrodlo" bson:"zrodlo"`
}

type PaymentMeta struct {
	Provider      string `json:"operator" bson:"operator"`
	TransactionID string `json:"id_transakcji" bson:"id_transakcji"`
	ThreeDS       bool   `json:"3ds" bson:"3ds"`
}

type Payment struct {
	ID            int64       `json:"id" bson:"id"`
	ReservationID int64       `json:"rezerwacja_id" bson:"-"`
	Amount        float64     `json:"kwota" bson:"kwota"`
	Method        string      `json:"metoda" bson:"metoda"`
	Status        string      `json:"status" bson:"status"`
	PaidAt        time.Time   `json:"zaplacono" bson:"zaplacono"`
	Meta          PaymentMeta `json:"metadane" bson:"metadane"`
}

type Reservation struct {
	ID         int64           `json:"id" bson:"_id"`
	CustomerID int64           `json:"klient_id" bson:"klient_id"`
	CreatedAt  time.Time       `json:"utworzono" bson:"utworzono"`
	Status     string          `json:"status" bson:"status"`
	ExpiresAt  *time.Time      `json:"wygasa_o,omitempty" bson:"wygasa_o,omitempty"`
	Amount     float64         `json:"kwota" bson:"kwota"`
	DiscountID int64           `json:"kod_rabatowy_id,omitempty" bson:"kod_rabatowy_id,omitempty"`
	Meta       ReservationMeta `json:"metadane" bson:"metadane"`
	Tickets    []TicketRef     `json:"bilety" bson:"bilety"`
	Payment    *Payment        `json:"platnosc,omitempty" bson:"platnosc,omitempty"`
}
