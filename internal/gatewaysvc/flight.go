package gatewaysvc

import "golang.org/x/sync/singleflight"

type flightGroup struct {
	group singleflight.Group
}

func newFlightGroup() *flightGroup {
	return &flightGroup{}
}

func (g *flightGroup) Do(key string, fn func() (any, error)) (any, bool, error) {
	v, err, shared := g.group.Do(key, fn)
	return v, shared, err
}
