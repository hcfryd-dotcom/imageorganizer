// Package places names the town nearest a GPS position, offline: a list of
// the world's towns of more than 1000 people (and smaller seats of local
// government), from the GeoNames gazetteer (CC BY 4.0, geonames.org),
// embedded in the program. Positions are never sent anywhere.
package places

import (
	"bufio"
	"bytes"
	"cmp"
	"compress/gzip"
	_ "embed"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
)

//go:embed cities.tsv.gz
var citiesGz []byte

//go:embed admin.tsv.gz
var adminGz []byte

// Place is a town.
type Place struct {
	Name    string
	Country string // ISO 3166 code, e.g. "DK"
	Lat     float64
	Lon     float64
	// Region and Area are the first and second level of local government
	// (in Denmark: region and kommune), in the local language when known.
	Region string
	Area   string
}

type cell struct{ lat, lon int }

var (
	once  sync.Once
	grid  map[cell][]Place
	count int
)

func lines(gz []byte, fn func(f []string)) {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return
	}
	defer zr.Close()
	sc := bufio.NewScanner(zr)
	for sc.Scan() {
		fn(strings.Split(sc.Text(), "\t"))
	}
}

func load() {
	grid = map[cell][]Place{}
	admin := map[string]string{}
	lines(adminGz, func(f []string) {
		if len(f) == 2 {
			admin[f[0]] = f[1]
		}
	})
	lines(citiesGz, func(f []string) {
		if len(f) != 6 {
			return
		}
		lat, err1 := strconv.ParseFloat(f[1], 64)
		lon, err2 := strconv.ParseFloat(f[2], 64)
		if err1 != nil || err2 != nil {
			return
		}
		p := Place{Name: f[0], Country: f[3], Lat: lat, Lon: lon}
		if f[4] != "" {
			p.Region = admin[f[3]+"."+f[4]]
			if f[5] != "" {
				p.Area = admin[f[3]+"."+f[4]+"."+f[5]]
			}
		}
		c := cellOf(lat, lon)
		grid[c] = append(grid[c], p)
		count++
	})
}

func cellOf(lat, lon float64) cell { return cell{int(math.Floor(lat)), int(math.Floor(lon))} }

// Count is how many places are known.
func Count() int { once.Do(load); return count }

// MaxDistance is how far, in kilometres, the nearest town may be: further
// out (at sea, in a desert) a position gets no place.
const MaxDistance = 60.0

// Nearest returns the town nearest the position, within MaxDistance.
func Nearest(lat, lon float64) (Place, bool) {
	once.Do(load)
	if math.IsNaN(lat) || math.IsNaN(lon) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return Place{}, false
	}
	c := cellOf(lat, lon)
	best, bestD := Place{}, math.Inf(1)
	// One degree of latitude is 111 km; search the cells around until the
	// ring is surely further than the best found.
	for r := 0; r <= 2; r++ {
		for dlat := -r; dlat <= r; dlat++ {
			for dlon := -r; dlon <= r; dlon++ {
				if max(abs(dlat), abs(dlon)) != r {
					continue // inner cells were searched already
				}
				lo := c.lon + dlon
				if lo < -180 {
					lo += 360
				} else if lo >= 180 {
					lo -= 360
				}
				for _, p := range grid[cell{c.lat + dlat, lo}] {
					if d := distance(lat, lon, p.Lat, p.Lon); d < bestD {
						best, bestD = p, d
					}
				}
			}
		}
		if bestD <= float64(r)*111*math.Cos(lat*math.Pi/180) {
			break
		}
	}
	if bestD > MaxDistance {
		return Place{}, false
	}
	return best, true
}

// Search finds up to limit towns whose name starts with q (any case):
// exact names first, then shorter names, then by country and name.
func Search(q string, limit int) []Place {
	once.Do(load)
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" || limit <= 0 {
		return nil
	}
	var out []Place
	for _, ps := range grid {
		for _, p := range ps {
			if strings.HasPrefix(strings.ToLower(p.Name), q) {
				out = append(out, p)
			}
		}
	}
	exact := func(p Place) int {
		if strings.ToLower(p.Name) == q {
			return 0
		}
		return 1
	}
	slices.SortFunc(out, func(a, b Place) int {
		return cmp.Or(cmp.Compare(exact(a), exact(b)), cmp.Compare(len(a.Name), len(b.Name)),
			cmp.Compare(a.Country, b.Country), cmp.Compare(a.Name, b.Name), cmp.Compare(a.Area, b.Area))
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Distance in kilometres between two positions.
func Distance(lat1, lon1, lat2, lon2 float64) float64 { return distance(lat1, lon1, lat2, lon2) }

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// distance in kilometres between two positions (haversine).
func distance(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dlat, dlon := (lat2-lat1)*rad, (lon2-lon1)*rad
	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dlon/2)*math.Sin(dlon/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}
