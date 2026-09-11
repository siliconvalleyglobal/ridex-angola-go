package luanda

import (
	"math/big"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

// LuandaZone defines a service zone in the Luanda metropolitan area.
type LuandaZone struct {
	Name        string
	Slug        string
	MinLat      float64
	MinLng      float64
	MaxLat      float64
	MaxLng      float64
	IsActive    bool
	BaseCents   int64
	PerKmCents  int64
	PerMinCents int64
}

// LuandaZones returns the predefined Luanda metropolitan service zones.
// Coordinates are approximate and should be refined with actual operational data.
func LuandaZones() []LuandaZone {
	return []LuandaZone{
		{
			// Luanda City Center - Baixa, Samba, Cidade Alta
			Name:        "Luanda City Center",
			Slug:        "luanda-city-center",
			MinLat:      -9.4800,
			MinLng:      13.1800,
			MaxLat:      -9.4300,
			MaxLng:      13.2500,
			IsActive:    true,
			BaseCents:   500,
			PerKmCents:  50,
			PerMinCents: 15,
		},
		{
			// Talatona - Business district, high-end area
			Name:        "Talatona",
			Slug:        "talatona",
			MinLat:      -9.4600,
			MinLng:      13.2500,
			MaxLat:      -9.4400,
			MaxLng:      13.2800,
			IsActive:    true,
			BaseCents:   600,
			PerKmCents:  55,
			PerMinCents: 16,
		},
		{
			// Kilamba - Large residential area (Kilamba Kiaxi)
			Name:        "Kilamba",
			Slug:        "kilamba",
			MinLat:      -9.4900,
			MinLng:      13.2400,
			MaxLat:      -9.4400,
			MaxLng:      13.3000,
			IsActive:    true,
			BaseCents:   500,
			PerKmCents:  48,
			PerMinCents: 14,
		},
		{
			// Viana - Commercial and residential area
			Name:        "Viana",
			Slug:        "viana",
			MinLat:      -9.5300,
			MinLng:      13.2700,
			MaxLat:      -9.4900,
			MaxLng:      13.3300,
			IsActive:    true,
			BaseCents:   500,
			PerKmCents:  45,
			PerMinCents: 13,
		},
		{
			// Benfica - Residential area
			Name:        "Benfica",
			Slug:        "benfica",
			MinLat:      -9.5200,
			MinLng:      13.2100,
			MaxLat:      -9.4900,
			MaxLng:      13.2600,
			IsActive:    true,
			BaseCents:   450,
			PerKmCents:  45,
			PerMinCents: 13,
		},
		{
			// Cacuaco - Outskirts, north of Luanda
			Name:        "Cacuaco",
			Slug:        "cacuaco",
			MinLat:      -9.5800,
			MinLng:      13.2400,
			MaxLat:      -9.5200,
			MaxLng:      13.3200,
			IsActive:    true,
			BaseCents:   550,
			PerKmCents:  50,
			PerMinCents: 15,
		},
		{
			// Alvalade - Residential/commercial
			Name:        "Alvalade",
			Slug:        "alvalade",
			MinLat:      -9.4900,
			MinLng:      13.1800,
			MaxLat:      -9.4600,
			MaxLng:      13.2300,
			IsActive:    true,
			BaseCents:   500,
			PerKmCents:  48,
			PerMinCents: 14,
		},
		{
			// Maianga - Central area
			Name:        "Maianga",
			Slug:        "maianga",
			MinLat:      -9.4700,
			MinLng:      13.2000,
			MaxLat:      -9.4400,
			MaxLng:      13.2500,
			IsActive:    true,
			BaseCents:   500,
			PerKmCents:  48,
			PerMinCents: 14,
		},
		{
			// Rocha Sanchez - Residential
			Name:        "Rocha Sanchez",
			Slug:        "rocha-sanchez",
			MinLat:      -9.5000,
			MinLng:      13.2000,
			MaxLat:      -9.4700,
			MaxLng:      13.2500,
			IsActive:    true,
			BaseCents:   450,
			PerKmCents:  45,
			PerMinCents: 13,
		},
		{
			// Cazenga - Eastern area
			Name:        "Cazenga",
			Slug:        "cazenga",
			MinLat:      -9.5200,
			MinLng:      13.2800,
			MaxLat:      -9.4700,
			MaxLng:      13.3500,
			IsActive:    true,
			BaseCents:   500,
			PerKmCents:  48,
			PerMinCents: 14,
		},
	}
}

// ToDBParams converts LuandaZone to database parameters for creation.
func (z LuandaZone) ToDBParams() db.CreateServiceZoneParams {
	return db.CreateServiceZoneParams{
		Name:        z.Name,
		Slug:        z.Slug,
		MinLat:      decimal(z.MinLat),
		MinLng:      decimal(z.MinLng),
		MaxLat:      decimal(z.MaxLat),
		MaxLng:      decimal(z.MaxLng),
		IsActive:    z.IsActive,
		BaseCents:   optionalInt64(z.BaseCents),
		PerKmCents:  optionalInt64(z.PerKmCents),
		PerMinCents: optionalInt64(z.PerMinCents),
	}
}

// decimal converts float64 to pgtype.Numeric.
func decimal(v float64) pgtype.Numeric {
	return pgtype.Numeric{
		Int:   big.NewInt(int64(v * 1000000)),
		Exp:   -6,
		Valid: true,
	}
}

// optionalInt64 converts int64 to pgtype.Numeric.
func optionalInt64(v int64) pgtype.Numeric {
	if v == 0 {
		return pgtype.Numeric{}
	}
	return pgtype.Numeric{
		Int:   big.NewInt(v),
		Valid: true,
	}
}
