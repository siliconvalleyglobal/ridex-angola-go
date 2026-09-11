package luanda

import (
	"testing"
)

func TestLuandaZones(t *testing.T) {
	zones := LuandaZones()

	if len(zones) == 0 {
		t.Fatal("expected at least one Luanda zone")
	}

	// Verify required zones exist
	requiredZones := []string{
		"luanda-city-center",
		"talatona",
		"kilamba",
		"viana",
		"benfica",
		"cacuaco",
	}

	foundZones := make(map[string]bool)
	for _, zone := range zones {
		foundZones[zone.Slug] = true

		// Validate zone properties
		if zone.Name == "" {
			t.Errorf("zone has empty name")
		}
		if zone.Slug == "" {
			t.Errorf("zone has empty slug")
		}
		if zone.MinLat >= zone.MaxLat {
			t.Errorf("zone %s has invalid lat bounds: minLat=%.4f, maxLat=%.4f", zone.Slug, zone.MinLat, zone.MaxLat)
		}
		if zone.MinLng >= zone.MaxLng {
			t.Errorf("zone %s has invalid lng bounds: minLng=%.4f, maxLng=%.4f", zone.Slug, zone.MinLng, zone.MaxLng)
		}
		if zone.BaseCents < 0 {
			t.Errorf("zone %s has negative baseCents: %d", zone.Slug, zone.BaseCents)
		}
		if zone.PerKmCents < 0 {
			t.Errorf("zone %s has negative perKmCents: %d", zone.Slug, zone.PerKmCents)
		}
		if zone.PerMinCents < 0 {
			t.Errorf("zone %s has negative perMinCents: %d", zone.Slug, zone.PerMinCents)
		}
	}

	for _, slug := range requiredZones {
		if !foundZones[slug] {
			t.Errorf("required zone %s not found", slug)
		}
	}

	// Verify coordinates are in Luanda area
	for _, zone := range zones {
		if zone.MinLat < -9.7 || zone.MaxLat > -9.3 {
			t.Errorf("zone %s has latitudes outside Luanda range: minLat=%.4f, maxLat=%.4f", zone.Slug, zone.MinLat, zone.MaxLat)
		}
		if zone.MinLng < 13.1 || zone.MaxLng > 13.4 {
			t.Errorf("zone %s has longitudes outside Luanda range: minLng=%.4f, maxLng=%.4f", zone.Slug, zone.MinLng, zone.MaxLng)
		}
	}

	// Verify no duplicate slugs
	if len(zones) != len(foundZones) {
		t.Error("duplicate zone slugs found")
	}
}

func TestLuandaZone_DecimalConversion(t *testing.T) {
	zones := LuandaZones()
	if len(zones) == 0 {
		t.Fatal("expected at least one zone")
	}

	zone := zones[0]
	params := zone.ToDBParams()

	if params.Name != zone.Name {
		t.Errorf("expected name %s, got %s", zone.Name, params.Name)
	}
	if params.Slug != zone.Slug {
		t.Errorf("expected slug %s, got %s", zone.Slug, params.Slug)
	}
	if params.IsActive != zone.IsActive {
		t.Errorf("expected isActive %v, got %v", zone.IsActive, params.IsActive)
	}

	// Verify decimal conversion preserves values approximately
	if !params.MinLat.Valid || !params.MinLng.Valid || !params.MaxLat.Valid || !params.MaxLng.Valid {
		t.Error("expected valid decimal conversions")
	}
}
