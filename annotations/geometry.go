package annotations

import (
	"errors"
	"math"
)

type Point struct {
	X float32
	Y float32
}

func (p *Point) ToList() []float32 {
	return []float32{p.X, p.Y}
}

type Rect struct {
	LL Point
	UR Point
}

func (r *Rect) Intersects(b Rect) bool {
	if r.LL.X == r.UR.X ||
		r.LL.Y == r.UR.Y ||
		b.LL.X == b.UR.X ||
		b.LL.Y == b.UR.Y {
		return false
	}

	if r.LL.X >= b.UR.Y ||
		b.LL.X >= r.UR.Y {
		return false
	}

	if r.UR.Y <= b.LL.Y ||
		b.UR.Y <= r.LL.Y {
		return false
	}

	return true
}

func RectFromList(l []float32) (Rect, error) {
	if len(l) != 4 {
		return Rect{}, errors.New("List must contain exactly 4 elements")
	}

	return Rect{
		LL: Point{
			X: l[0],
			Y: l[1],
		},
		UR: Point{
			X: l[2],
			Y: l[3],
		},
	}, nil
}

func (r *Rect) Union(b Rect) Rect {
	ll := Point{
		X: float32(math.Min(float64(r.LL.X), float64(b.LL.X))),
		Y: float32(math.Min(float64(r.LL.Y), float64(b.LL.Y)))}

	ur := Point{
		X: float32(math.Max(float64(r.UR.X), float64(b.UR.X))),
		Y: float32(math.Max(float64(r.UR.Y), float64(b.UR.Y)))}

	return Rect{LL: ll, UR: ur}
}

func (r *Rect) ToList() []float32 {
	return []float32{r.LL.X, r.LL.Y, r.UR.X, r.UR.Y}
}

type QuadPoints struct {
	Points []Point
}

func (q *QuadPoints) Append(b QuadPoints) QuadPoints {
	return QuadPoints{Points: append(q.Points, b.Points...)}
}

func (q *QuadPoints) ToList() []float32 {
	list := make([]float32, 0, len(q.Points)*4)
	for _, p := range q.Points {
		list = append(list, p.ToList()...)
	}
	return list
}

func (r *Rect) ToQuadPoints() QuadPoints {
	return QuadPoints{
		Points: []Point{
			{
				X: r.LL.X,
				Y: r.UR.Y,
			},
			{
				X: r.UR.X,
				Y: r.UR.Y,
			},
			{
				X: r.LL.X,
				Y: r.LL.Y,
			},
			{
				X: r.UR.X,
				Y: r.LL.Y,
			},
		},
	}
}

func QuadPointsFromList(list []float32) QuadPoints {
	points := make([]Point, len(list)/2)
	for i := 0; i < len(list)-1; i += 2 {
		points[i/2] = Point{X: list[i], Y: list[i+1]}
	}
	return QuadPoints{Points: points}
}

func (h *Highlight) Union(b Highlight) Highlight {
	rectA, _ := RectFromList(h.Rect)

	rectB, _ := RectFromList(b.Rect)

	united := rectA.Union(rectB)

	qpA := QuadPointsFromList(h.QuadPoints)
	qpB := QuadPointsFromList(b.QuadPoints)
	appended := qpA.Append(qpB)

	return Highlight{
		Contents:   h.Contents + b.Contents,
		Rect:       united.ToList(),
		QuadPoints: appended.ToList(),
		Color:      h.Color,
		Opacity:    h.Opacity,
		Author:     h.Author,
	}
}

func (h *Highlight) Intersects(b Highlight) bool {
	rectA, _ := RectFromList(h.Rect)
	rectB, _ := RectFromList(b.Rect)
	return (rectA.Intersects(rectB))
}
