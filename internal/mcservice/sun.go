package mcservice

import (
	"math"
	"time"
)

// SunTimes holds local sunrise/sunset as "HH:MM" strings.
type SunTimes struct {
	Sunrise string `json:"sunrise"`
	Sunset  string `json:"sunset"`
}

// approxLatitude is the latitude assumed for the computation. The launcher no
// longer geolocates the user by IP, so the location is estimated from the
// system time zone: longitude from the standard (non-DST) UTC offset, latitude
// fixed at a mid-latitude value. That is accurate enough to switch light/dark
// themes.
const approxLatitude = 35.0

func GetSunTimes() SunTimes {
	return sunTimesAt(time.Now(), approxLatitude)
}

func sunTimesAt(now time.Time, lat float64) SunTimes {
	lon := float64(standardOffset(now)) / 3600.0 * 15.0

	rise, set, ok := solveSun(now, lat, lon)
	if !ok {
		return SunTimes{Sunrise: "06:00", Sunset: "18:00"}
	}
	return SunTimes{
		Sunrise: rise.In(now.Location()).Format("15:04"),
		Sunset:  set.In(now.Location()).Format("15:04"),
	}
}

// standardOffset returns the zone's UTC offset without daylight saving time:
// the smaller of the January and July offsets, which holds in both
// hemispheres.
func standardOffset(t time.Time) int {
	loc := t.Location()
	_, jan := time.Date(t.Year(), time.January, 1, 12, 0, 0, 0, loc).Zone()
	_, jul := time.Date(t.Year(), time.July, 1, 12, 0, 0, 0, loc).Zone()
	if jan < jul {
		return jan
	}
	return jul
}

// solveSun implements the standard sunrise equation (NOAA approximation) and
// returns the UTC sunrise and sunset for the calendar day of t at lat/lon.
// ok is false for polar day/night, where the sun does not rise or set.
func solveSun(t time.Time, lat, lon float64) (rise, set time.Time, ok bool) {
	const rad = math.Pi / 180

	y, m, d := t.Date()
	dayStart := time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
	n := float64(dayStart.YearDay())

	// Fractional year, equation of time (minutes) and solar declination.
	g := 2 * math.Pi / 365 * (n - 1)
	eqTime := 229.18 * (0.000075 + 0.001868*math.Cos(g) - 0.032077*math.Sin(g) -
		0.014615*math.Cos(2*g) - 0.040849*math.Sin(2*g))
	decl := 0.006918 - 0.399912*math.Cos(g) + 0.070257*math.Sin(g) -
		0.006758*math.Cos(2*g) + 0.000907*math.Sin(2*g) -
		0.002697*math.Cos(3*g) + 0.00148*math.Sin(3*g)

	// Hour angle at sunrise/sunset (zenith 90.833° accounts for refraction).
	cosH := math.Cos(90.833*rad)/(math.Cos(lat*rad)*math.Cos(decl)) -
		math.Tan(lat*rad)*math.Tan(decl)
	if cosH > 1 || cosH < -1 {
		return time.Time{}, time.Time{}, false
	}
	h := math.Acos(cosH) / rad

	riseMin := 720 - 4*(lon+h) - eqTime
	setMin := 720 - 4*(lon-h) - eqTime

	midnight := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	rise = midnight.Add(time.Duration(riseMin * float64(time.Minute)))
	set = midnight.Add(time.Duration(setMin * float64(time.Minute)))
	return rise, set, true
}
