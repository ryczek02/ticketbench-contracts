package datagen

var cities = []string{
	"Warszawa", "Kraków", "Wrocław", "Łódź", "Poznań", "Gdańsk", "Szczecin",
	"Bydgoszcz", "Lublin", "Białystok", "Katowice", "Gdynia", "Częstochowa",
	"Radom", "Toruń", "Kielce", "Rzeszów", "Gliwice", "Zabrze", "Olsztyn",
	"Bielsko-Biała", "Bytom", "Zielona Góra", "Opole",
}

var streets = []string{
	"Długa", "Krótka", "Polna", "Leśna", "Słoneczna", "Kościuszki", "Mickiewicza",
	"Piłsudskiego", "Sienkiewicza", "Kolejowa", "Ogrodowa", "Lipowa", "Parkowa",
	"Szkolna", "Marszałkowska", "Wawelska", "Rynek", "Zamkowa",
}

var venueKinds = []string{
	"Hala", "Arena", "Teatr", "Filharmonia", "Klub", "Stadion", "Centrum Kongresowe", "Opera",
}

var amenities = []string{"szatnia", "bar", "wifi", "winda", "klimatyzacja", "parking rowerowy", "strefa vip"}

var sectorKinds = []string{"parter", "balkon", "loza", "trybuna", "plyta"}

var categories = []string{
	"koncert", "teatr", "sport", "kabaret", "opera", "festiwal",
	"stand-up", "konferencja", "kino", "wystawa", "balet", "e-sport",
}

// genres per category index, same order as categories
var genres = [][]string{
	{"rock", "pop", "jazz", "metal", "hip-hop", "elektronika"},
	{"dramat", "komedia", "musical", "teatr lalek"},
	{"pilka nozna", "siatkowka", "koszykowka", "zuzel", "hokej"},
	{"kabaret klasyczny", "improwizacja"},
	{"opera wloska", "opera polska", "operetka"},
	{"muzyczny", "filmowy", "kulinarny"},
	{"stand-up", "open mic"},
	{"it", "biznes", "nauka"},
	{"premiera", "maraton filmowy"},
	{"malarstwo", "fotografia", "design"},
	{"balet klasyczny", "taniec wspolczesny"},
	{"cs2", "lol", "valorant"},
}

var basePrice = []float64{120, 90, 70, 80, 160, 250, 60, 400, 30, 35, 140, 50}

var tags = []string{"plener", "dla dzieci", "premiera", "wyprzedane", "vip", "akustycznie", "na zywo", "rodzinne", "nocne"}

var firstNames = []string{
	"Anna", "Maria", "Katarzyna", "Małgorzata", "Agnieszka", "Barbara", "Ewa", "Magdalena",
	"Joanna", "Zofia", "Piotr", "Krzysztof", "Andrzej", "Tomasz", "Paweł", "Jan", "Michał",
	"Marcin", "Łukasz", "Jakub", "Mateusz", "Kacper", "Julia", "Natalia", "Oliwia",
}

var lastNames = []string{
	"Nowak", "Kowalski", "Wiśniewski", "Wójcik", "Kowalczyk", "Kamiński", "Lewandowski",
	"Zieliński", "Szymański", "Woźniak", "Dąbrowski", "Kozłowski", "Jankowski", "Mazur",
	"Kwiatkowski", "Krawczyk", "Piotrowski", "Grabowski", "Nowakowski", "Pawłowski",
	"Michalski", "Król", "Wieczorek", "Jabłoński", "Wróbel",
}

var performerA = []string{"Czerwone", "Zielone", "Nocne", "Stare", "Nowe", "Dzikie", "Ciche", "Głośne", "Złote", "Szare"}
var performerB = []string{"Gitary", "Echo", "Miasto", "Ptaki", "Fale", "Kroki", "Światła", "Lustra", "Dachy", "Mosty"}

var channels = []string{"web", "mobile", "kasa", "partner"}
var devices = []string{"android", "ios", "windows", "macos", "linux"}
var sources = []string{"google", "facebook", "newsletter", "bezposrednio", "partner"}
var paymentMethods = []string{"karta", "blik", "przelew", "paypal"}
var providers = []string{"PayU", "Przelewy24", "Stripe", "Tpay"}
var languages = []string{"pl", "pl", "pl", "en", "de", "uk"}

// Tags returns the event tags used by the generator.
func Tags() []string { return append([]string(nil), tags...) }

// Genres returns every genre the generator can assign.
func Genres() []string {
	var out []string
	for _, g := range genres {
		out = append(out, g...)
	}
	return out
}
