// Package zones contains service-zone administration and point validation.
package zones

import (
	"math"
	"math/big"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

// Handler implements the admin service-zone CRUD endpoints.
type Handler struct {
	q *db.Queries
}

func NewHandler(q *db.Queries) *Handler { return &Handler{q: q} }

type zoneRequest struct {
	Name        string  `json:"name" binding:"required"`
	Slug        string  `json:"slug" binding:"required"`
	MinLat      float64 `json:"minLat"`
	MinLng      float64 `json:"minLng"`
	MaxLat      float64 `json:"maxLat"`
	MaxLng      float64 `json:"maxLng"`
	IsActive    *bool   `json:"isActive"`
	BaseCents   *int64  `json:"baseCents"`
	PerKmCents  *int64  `json:"perKmCents"`
	PerMinCents *int64  `json:"perMinCents"`
}

type updateZoneRequest struct {
	Name        *string  `json:"name"`
	Slug        *string  `json:"slug"`
	MinLat      *float64 `json:"minLat"`
	MinLng      *float64 `json:"minLng"`
	MaxLat      *float64 `json:"maxLat"`
	MaxLng      *float64 `json:"maxLng"`
	IsActive    *bool    `json:"isActive"`
	BaseCents   *int64   `json:"baseCents"`
	PerKmCents  *int64   `json:"perKmCents"`
	PerMinCents *int64   `json:"perMinCents"`
}

// ValidateBounds validates a rectangle without requiring a database.
func ValidateBounds(minLat, minLng, maxLat, maxLng float64) bool {
	return finite(minLat) && finite(minLng) && finite(maxLat) && finite(maxLng) &&
		minLat >= -90 && maxLat <= 90 && minLng >= -180 && maxLng <= 180 &&
		minLat < maxLat && minLng < maxLng
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func (h *Handler) Create(c *gin.Context) {
	var req zoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if err := validateRequest(req.Name, req.Slug, req.MinLat, req.MinLng, req.MaxLat, req.MaxLng,
		req.BaseCents, req.PerKmCents, req.PerMinCents); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	zone, err := h.q.CreateServiceZone(c.Request.Context(), db.CreateServiceZoneParams{
		Name: req.Name, Slug: req.Slug,
		MinLat: decimal(req.MinLat), MinLng: decimal(req.MinLng),
		MaxLat: decimal(req.MaxLat), MaxLng: decimal(req.MaxLng),
		IsActive: active, BaseCents: optionalInteger(req.BaseCents),
		PerKmCents: optionalInteger(req.PerKmCents), PerMinCents: optionalInteger(req.PerMinCents),
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "service zone could not be created"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"zone": zoneJSON(zone)})
}

func (h *Handler) List(c *gin.Context) {
	includeInactive := c.DefaultQuery("includeInactive", "false") == "true"
	zones, err := h.q.ListServiceZones(c.Request.Context(), !includeInactive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list service zones"})
		return
	}
	out := make([]gin.H, 0, len(zones))
	for _, zone := range zones {
		out = append(out, zoneJSON(zone))
	}
	c.JSON(http.StatusOK, gin.H{"zones": out})
}

func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service zone id"})
		return
	}
	zone, err := h.q.GetServiceZone(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "service zone not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"zone": zoneJSON(zone)})
}

func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service zone id"})
		return
	}
	current, err := h.q.GetServiceZone(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "service zone not found"})
		return
	}
	var req updateZoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	name, slug := current.Name, current.Slug
	minLat, minLng := numericFloat(current.MinLat), numericFloat(current.MinLng)
	maxLat, maxLng := numericFloat(current.MaxLat), numericFloat(current.MaxLng)
	active := current.IsActive
	base, perKM, perMin := numericInt(current.BaseCents), numericInt(current.PerKmCents), numericInt(current.PerMinCents)
	if req.Name != nil {
		name = *req.Name
	}
	if req.Slug != nil {
		slug = *req.Slug
	}
	if req.MinLat != nil {
		minLat = *req.MinLat
	}
	if req.MinLng != nil {
		minLng = *req.MinLng
	}
	if req.MaxLat != nil {
		maxLat = *req.MaxLat
	}
	if req.MaxLng != nil {
		maxLng = *req.MaxLng
	}
	if req.IsActive != nil {
		active = *req.IsActive
	}
	if req.BaseCents != nil {
		base = req.BaseCents
	}
	if req.PerKmCents != nil {
		perKM = req.PerKmCents
	}
	if req.PerMinCents != nil {
		perMin = req.PerMinCents
	}
	if err := validateRequest(name, slug, minLat, minLng, maxLat, maxLng, base, perKM, perMin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	zone, err := h.q.UpdateServiceZone(c.Request.Context(), db.UpdateServiceZoneParams{
		ID: id, Name: name, Slug: slug,
		MinLat: decimal(minLat), MinLng: decimal(minLng),
		MaxLat: decimal(maxLat), MaxLng: decimal(maxLng), IsActive: active,
		BaseCents: optionalInteger(base), PerKmCents: optionalInteger(perKM),
		PerMinCents: optionalInteger(perMin),
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "service zone could not be updated"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"zone": zoneJSON(zone)})
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service zone id"})
		return
	}
	if _, err := h.q.DeleteServiceZone(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "service zone not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

func validateRequest(name, slug string, minLat, minLng, maxLat, maxLng float64, base, perKM, perMin *int64) error {
	if strings.TrimSpace(name) == "" || len(name) > 120 {
		return errText("name must be between 1 and 120 characters")
	}
	if strings.TrimSpace(slug) == "" || len(slug) > 80 {
		return errText("slug must be between 1 and 80 characters")
	}
	if !ValidateBounds(minLat, minLng, maxLat, maxLng) {
		return errText("zone bounds are invalid")
	}
	if base != nil && *base < 0 {
		return errText("baseCents must be non-negative")
	}
	if perKM != nil && *perKM < 0 {
		return errText("perKmCents must be non-negative")
	}
	if perMin != nil && *perMin < 0 {
		return errText("perMinCents must be non-negative")
	}
	return nil
}

type textError string

func (e textError) Error() string { return string(e) }
func errText(s string) error      { return textError(s) }

func zoneJSON(z db.ServiceZone) gin.H {
	return gin.H{
		"id": z.ID, "name": z.Name, "slug": z.Slug,
		"minLat": numericFloat(z.MinLat), "minLng": numericFloat(z.MinLng),
		"maxLat": numericFloat(z.MaxLat), "maxLng": numericFloat(z.MaxLng),
		"isActive":  z.IsActive,
		"baseCents": numericInt(z.BaseCents), "perKmCents": numericInt(z.PerKmCents),
		"perMinCents": numericInt(z.PerMinCents),
		"createdAt":   z.CreatedAt, "updatedAt": z.UpdatedAt,
	}
}

func decimal(v float64) pgtype.Numeric {
	const scale = 1000000
	return pgtype.Numeric{Int: bigInt(int64(math.Round(v * scale))), Exp: -6, Valid: true}
}

func optionalInteger(v *int64) pgtype.Numeric {
	if v == nil {
		return pgtype.Numeric{}
	}
	return pgtype.Numeric{Int: bigInt(*v), Valid: true}
}

func bigInt(v int64) *big.Int { return big.NewInt(v) }

func numericFloat(n pgtype.Numeric) float64 {
	if !n.Valid || n.Int == nil {
		return 0
	}
	v, _ := new(big.Float).SetInt(n.Int).Float64()
	return v * math.Pow10(int(n.Exp))
}

func numericInt(n pgtype.Numeric) *int64 {
	if !n.Valid || n.Int == nil {
		return nil
	}
	value := numericFloat(n)
	if !finite(value) {
		return nil
	}
	result := int64(math.Round(value))
	return &result
}
