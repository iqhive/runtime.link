package xyz

import "encoding/json"

// Pair holds two values.
type Pair[X, Y any] struct {
	X X
	Y Y
}

// NewPair returns a new [Pair] from the given values.
func NewPair[X, Y any](x X, y Y) Pair[X, Y] {
	return Pair[X, Y]{x, y}
}

// Get returns the values in the [Pair].
func (p Pair[X, Y]) Get() (X, Y) {
	return p.X, p.Y
}

// Split returns the values in the [Pair].
func (p Pair[X, Y]) Split() (X, Y) {
	return p.X, p.Y
}

func (p Pair[X, Y]) MarshalJSON() ([]byte, error) {
	return json.Marshal([2]interface{}{p.X, p.Y})
}

func (p *Pair[X, Y]) UnmarshalJSON(data []byte) error {
	var v [2]json.RawMessage
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if err := json.Unmarshal(v[0], &p.X); err != nil {
		return err
	}
	if err := json.Unmarshal(v[1], &p.Y); err != nil {
		return err
	}
	return nil
}

// Trio holds three values.
type Trio[X, Y, Z any] struct {
	X X
	Y Y
	Z Z
}

// NewTrio returns a new [Trio] from the given values.
func NewTrio[X, Y, Z any](a X, b Y, c Z) Trio[X, Y, Z] {
	return Trio[X, Y, Z]{a, b, c}
}

// Split returns the values in the [Trio].
func (t Trio[X, Y, Z]) Split() (X, Y, Z) {
	return t.X, t.Y, t.Z
}

// Get returns the values in the [Trio].
func (t Trio[X, Y, Z]) Get() (X, Y, Z) {
	return t.X, t.Y, t.Z
}

func (t Trio[X, Y, Z]) MarshalJSON() ([]byte, error) {
	return json.Marshal([3]interface{}{t.X, t.Y, t.Z})
}

func (t *Trio[X, Y, Z]) UnmarshalJSON(data []byte) error {
	var v [3]json.RawMessage
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if err := json.Unmarshal(v[0], &t.X); err != nil {
		return err
	}
	if err := json.Unmarshal(v[1], &t.Y); err != nil {
		return err
	}
	if err := json.Unmarshal(v[2], &t.Z); err != nil {
		return err
	}
	return nil
}

// Quad holds four values.
type Quad[X, Y, Z, W any] struct {
	X X
	Y Y
	Z Z
	W W
}

// NewQuad returns a new [Quad] from the given values.
func NewQuad[X, Y, Z, W any](a X, b Y, c Z, d W) Quad[X, Y, Z, W] {
	return Quad[X, Y, Z, W]{a, b, c, d}
}

// Split returns the values in the [Quad].
func (q Quad[X, Y, Z, W]) Split() (X, Y, Z, W) {
	return q.X, q.Y, q.Z, q.W
}

// Get returns the values in the [Quad].
func (q Quad[X, Y, Z, W]) Get() (X, Y, Z, W) {
	return q.X, q.Y, q.Z, q.W
}

func (q Quad[X, Y, Z, W]) MarshalJSON() ([]byte, error) {
	return json.Marshal([4]interface{}{q.X, q.Y, q.Z, q.W})
}

func (q *Quad[X, Y, Z, W]) UnmarshalJSON(data []byte) error {
	var v [4]json.RawMessage
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if err := json.Unmarshal(v[0], &q.X); err != nil {
		return err
	}
	if err := json.Unmarshal(v[1], &q.Y); err != nil {
		return err
	}
	if err := json.Unmarshal(v[2], &q.Z); err != nil {
		return err
	}
	if err := json.Unmarshal(v[3], &q.W); err != nil {
		return err
	}
	return nil
}
