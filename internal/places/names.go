package places

import "strings"

// Folder names for places, as a person would label a holiday's photos.

// countryDA names countries in Danish; others are given by their code.
var countryDA = map[string]string{
	"DK": "Danmark", "SE": "Sverige", "NO": "Norge", "DE": "Tyskland", "FI": "Finland", "IS": "Island", "FO": "Færøerne",
	"GL": "Grønland", "GB": "England", "IE": "Irland", "NL": "Holland", "BE": "Belgien", "LU": "Luxembourg", "FR": "Frankrig",
	"ES": "Spanien", "PT": "Portugal", "IT": "Italien", "CH": "Schweiz", "AT": "Østrig", "PL": "Polen", "CZ": "Tjekkiet",
	"HU": "Ungarn", "HR": "Kroatien", "SI": "Slovenien", "GR": "Grækenland", "CY": "Cypern", "MT": "Malta", "TR": "Tyrkiet",
	"EE": "Estland", "LV": "Letland", "LT": "Litauen", "BG": "Bulgarien", "RO": "Rumænien", "US": "USA", "CA": "Canada",
	"MX": "Mexico", "TH": "Thailand", "EG": "Egypten", "MA": "Marokko", "TN": "Tunesien", "AE": "Emiraterne", "AU": "Australien",
	"NZ": "New Zealand", "JP": "Japan", "CN": "Kina", "IN": "Indien", "BR": "Brasilien", "AR": "Argentina", "ZA": "Sydafrika",
	"KE": "Kenya", "TZ": "Tanzania", "VN": "Vietnam", "ID": "Indonesien", "SG": "Singapore", "MY": "Malaysia", "PH": "Filippinerne",
	"CU": "Cuba", "DO": "Den Dominikanske Republik", "MV": "Maldiverne", "LK": "Sri Lanka", "IL": "Israel", "JO": "Jordan",
}

// Country is the Danish name of a country code, or the code.
func Country(cc string) string {
	if n, ok := countryDA[cc]; ok {
		return n
	}
	return cc
}

// Suffixes that say what kind of area it is, left off folder names:
// "Frederikshavn Kommune" is filed as "Frederikshavn".
var areaSuffixes = []string{" Kommune", " Municipality", " kommun", " Kommun", " kommune", " District", " County", " Province", " Region"}

// and prefixes, so "Província de Barcelona" is "Barcelona".
var areaPrefixes = []string{"Província de ", "Provincia de ", "Provincia di ", "Province of ", "Département de ", "Landkreis ", "Kreis ", "County ", "City of ", "Comune di ", "Gemeente "}

func trimArea(s string) string {
	for _, suf := range areaSuffixes {
		s = strings.TrimSuffix(s, suf)
	}
	for _, pre := range areaPrefixes {
		s = strings.TrimPrefix(s, pre)
	}
	return strings.TrimSpace(s)
}

// Level is how finely photos are sorted by place.
type Level int

const (
	ByArea Level = iota // kommune (municipality) or the like: a town with its suburbs and villages
	ByTown              // the nearest town or village
)

// Label is the folder name for a place: the area (or the town) and, abroad
// (outside home, a country code such as "DK"), the country.
func (p Place) Label(level Level, home string) string {
	name := p.Name
	if level == ByArea {
		switch a := trimArea(p.Area); {
		case a != "" && !isCode(a):
			name = a
		case p.Region != "" && !isCode(p.Region):
			name = trimArea(p.Region)
		}
	}
	if p.Country != home {
		name += ", " + Country(p.Country)
	}
	return SafeName(name)
}

func isCode(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

// SafeName makes s usable as a folder name on Windows as well.
func SafeName(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r < 32, strings.ContainsRune(`<>:"/\|?*`, r):
			return '-'
		}
		return r
	}, s)
	s = strings.TrimRight(strings.TrimSpace(s), ". ")
	if s == "" {
		return "-"
	}
	return s
}
