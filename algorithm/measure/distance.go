package measure

import (
	"math"

	"github.com/spatial-go/geoos/algorithm/matrix"
	"github.com/spatial-go/geoos/algorithm/relate"
	"github.com/spatial-go/geoos/coordtransform"
	"github.com/spatial-go/geoos/space/spaceerr"
)

const (

	// R radius of earth.
	R = 6371000.0 //6378137.0
	// E is eccentricity.
	E = 0.006694379990141317
)

// Distance is a func of measure distance.
type Distance func(from, to matrix.Steric) float64

// MercatorDistance scale factor is changed along the meridians as a function of latitude
// https://gis.stackexchange.com/questions/110730/mercator-scale-factor-is-changed-along-the-meridians-as-a-function-of-latitude
// https://gis.stackexchange.com/questions/93332/calculating-distance-scale-factor-by-latitude-for-mercator
func MercatorDistance(distance float64, lat float64) float64 {
	lat = lat * math.Pi / 180
	factor := math.Sqrt(1-math.Pow(E, 2)*math.Pow(math.Sin(lat), 2)) * (1 / math.Cos(lat))
	distance = distance * factor
	return distance
}

// SpheroidDistance Calculate distance, return unit: meter
func SpheroidDistance(fromSteric, toSteric matrix.Steric) float64 {
	if to, ok := toSteric.(matrix.Matrix); ok {
		if from, ok := fromSteric.(matrix.Matrix); ok {
			rad := math.Pi / 180.0
			lat0 := from[1] * rad
			lng0 := from[0] * rad
			lat1 := to[1] * rad
			lng1 := to[0] * rad
			theta := lng1 - lng0
			dist := math.Acos(math.Sin(lat0)*math.Sin(lat1) + math.Cos(lat0)*math.Cos(lat1)*math.Cos(theta))
			return dist * R
		}
	}
	trans := coordtransform.NewTransformer(coordtransform.LLTOMERCATOR)
	from, _ := trans.TransformGeometry(fromSteric)
	to, _ := trans.TransformGeometry(toSteric)
	return PlanarDistance(from, to)
}

// PlanarDistance returns Distance of pq.
func PlanarDistance(fromSteric, toSteric matrix.Steric) float64 {
	switch to := toSteric.(type) {
	case matrix.Matrix:
		if from, ok := fromSteric.(matrix.Matrix); ok {
			return math.Sqrt((from[0]-to[0])*(from[0]-to[0]) + (from[1]-to[1])*(from[1]-to[1]))
		}
		return PlanarDistance(to, fromSteric)

	case matrix.LineMatrix:
		if from, ok := fromSteric.(matrix.Matrix); ok {
			return DistanceLineToPoint(to, from, PlanarDistance)
		} else if from, ok := fromSteric.(matrix.LineMatrix); ok {
			return DistanceLineAndLine(from, to, PlanarDistance)
		}
		return PlanarDistance(to, fromSteric)
	case matrix.PolygonMatrix:
		if from, ok := fromSteric.(matrix.Matrix); ok {
			return DistancePolygonToPoint(to, from, PlanarDistance)
		} else if from, ok := fromSteric.(matrix.LineMatrix); ok {
			return DistancePolygonAndLine(to, from, PlanarDistance)
		} else if from, ok := fromSteric.(matrix.PolygonMatrix); ok {
			var dist = math.MaxFloat64
			for _, v := range from {
				if distP := PlanarDistance(matrix.LineMatrix(v), to); dist > distP {
					dist = distP
				}
			}
			return dist
		}
		return PlanarDistance(to, fromSteric)
	case matrix.Collection:
		var dist = math.MaxFloat64
		for _, v := range to {
			if distP := PlanarDistance(fromSteric, v); dist > distP {
				dist = distP
			}
		}
		return dist
	default:
		return 0
	}
}

// DistanceSegmentToPoint Returns Distance of p,ab
func DistanceSegmentToPoint(p, a, b matrix.Matrix, f Distance) float64 {
	// if start = end, then just compute distance to one of the endpoints
	if a[0] == b[0] && a[1] == b[1] {
		return f(p, a)
	}
	// otherwise use comp.graphics.algorithms Frequently Asked Questions method
	//
	// (1) r = AC dot AB
	//         ---------
	//         ||AB||^2
	//
	// r has the following meaning:
	//   r=0 P = A
	//   r=1 P = B
	//   r<0 P is on the backward extension of AB
	//   r>1 P is on the forward extension of AB
	//   0<r<1 P is interior to AB

	len2 := (b[0]-a[0])*(b[0]-a[0]) + (b[1]-a[1])*(b[1]-a[1])
	r := ((p[0]-a[0])*(b[0]-a[0]) + (p[1]-a[1])*(b[1]-a[1])) / len2

	if r <= 0.0 {
		return f(p, a)
	}
	if r >= 1.0 {
		return f(p, b)
	}

	//
	// (2) s = (Ay-Cy)(Bx-Ax)-(Ax-Cx)(By-Ay)
	//         -----------------------------
	//                    L^2
	//
	// Then the distance from C to P = |s|*L.
	//
	// This is the same calculation .
	// Unrolled here for performance.
	//
	s := ((a[1]-p[1])*(b[0]-a[0]) - (a[0]-p[0])*(b[1]-a[1])) / len2
	return math.Abs(s) * math.Sqrt(len2)
}

// DistanceLineToPoint Returns Distance of p,line
func DistanceLineToPoint(line matrix.LineMatrix, pt matrix.Matrix, f Distance) (dist float64) {
	dist = math.MaxFloat64
	for i, v := range line {
		if i < len(line)-1 {
			if tmpDist := DistanceSegmentToPoint(pt, v, line[i+1], f); dist > tmpDist {
				dist = tmpDist
			}
		}
	}
	return
}

// DistancePolygonToPoint Returns Distance of p,polygon
func DistancePolygonToPoint(poly matrix.PolygonMatrix, pt matrix.Matrix, f Distance) (dist float64) {

	for _, v := range poly {
		tmpDist := DistanceLineToPoint(v, pt, f)
		if dist > tmpDist {
			dist = tmpDist
		}
	}
	return
}

// DistanceLineAndLine returns distance Between the two Geometry.
func DistanceLineAndLine(from, to matrix.LineMatrix, f Distance) (dist float64) {
	dist = math.MaxFloat64
	if mark := relate.IsIntersectionEdge(from, to); mark {
		return 0
	}
	for _, v := range from {
		if distP := f(matrix.Matrix(v), to); dist > distP {
			dist = distP
		}
	}
	for _, v := range to {
		if distP := f(from, matrix.Matrix(v)); dist > distP {
			dist = distP
		}
	}
	return dist
}

// DistancePolygonAndLine returns distance Between the two Geometry.
func DistancePolygonAndLine(poly matrix.PolygonMatrix, line matrix.LineMatrix, f Distance) (dist float64) {
	dist = math.MaxFloat64
	for _, v := range poly {
		if distP := f(matrix.LineMatrix(v), line); dist > distP {
			dist = distP
		}
	}
	return dist
}

// ElementDistance describes a geographic ElementDistance
type ElementDistance struct {
	From, To matrix.Steric
	F        Distance
}

// Distance returns distance Between the two Geometry.
func (el *ElementDistance) Distance() (float64, error) {
	if el.From.IsEmpty() && el.To.IsEmpty() {
		return 0, nil
	}
	if el.From.IsEmpty() != el.To.IsEmpty() {
		return 0, spaceerr.ErrNilGeometry
	}
	return el.F(el.From, el.To), nil
}
