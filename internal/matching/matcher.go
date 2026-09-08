package matching

import (
	"chamaelas-api/internal/geo"
	"chamaelas-api/internal/models"
)

// Expanding search radius, in kilometers — matches "aumentar o raio" until a
// driver is found or we give up at 100km.
var radiusTiersKm = []float64{2, 5, 10, 25, 50, 100}

// Nearest picks the closest online driver to the ride's origin, expanding
// the search radius tier by tier. If the origin has no geocoded coordinates
// (Nominatim/Photon failed to resolve it), it falls back to the first
// candidate so the ride still gets matched.
func Nearest(origin models.Address, candidates []models.Driver) (*models.Driver, bool) {
	if len(candidates) == 0 {
		return nil, false
	}
	if origin.Lat == 0 && origin.Lng == 0 {
		return &candidates[0], true
	}

	for _, radius := range radiusTiersKm {
		var nearest *models.Driver
		nearestDist := radius
		for i := range candidates {
			d := candidates[i]
			if d.Lat == nil || d.Lng == nil {
				continue
			}
			dist := geo.DistanceKm(origin.Lat, origin.Lng, *d.Lat, *d.Lng)
			if dist <= nearestDist {
				nearestDist = dist
				nearest = &candidates[i]
			}
		}
		if nearest != nil {
			return nearest, true
		}
	}
	return nil, false
}
