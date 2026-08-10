package scenario

import (
	"testing"

	"github.com/teko/food-delivery/internal/model"
)

func TestRoadPathStaysOnRoad(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		from model.Position
		to   model.Position
	}{
		{name: "horizontal then vertical", from: model.Position{X: 1.5, Y: 4}, to: model.Position{X: 16, Y: 20}},
		{name: "vertical then horizontal", from: model.Position{X: 8, Y: 7.5}, to: model.Position{X: 20, Y: 8}},
		{name: "snap arbitrary start", from: model.Position{X: 3.7, Y: 7.2}, to: model.Position{X: 12, Y: 16}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := RoadPath(test.from, test.to)
			if len(path) == 0 {
				t.Fatal("path is empty")
			}
			for _, waypoint := range path {
				if !IsOnRoad(waypoint) {
					t.Fatalf("waypoint %+v is not on a road", waypoint)
				}
			}
			if path[len(path)-1] != test.to {
				t.Fatalf("last waypoint = %+v, want %+v", path[len(path)-1], test.to)
			}
		})
	}
}

func TestOrdinalFromPodName(t *testing.T) {
	t.Parallel()
	tests := map[string]int{
		"courier-simulator-0": 1,
		"courier-simulator-4": 5,
		"local":               1,
	}
	for podName, expected := range tests {
		if actual := OrdinalFromPodName(podName); actual != expected {
			t.Errorf("OrdinalFromPodName(%q) = %d, want %d", podName, actual, expected)
		}
	}
}
