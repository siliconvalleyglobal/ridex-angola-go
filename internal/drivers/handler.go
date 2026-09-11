package drivers

import (
	"context"
	"errors"
	"math"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
)

type queryStore interface {
	GetDriverKYC(ctx context.Context, driverID uuid.UUID) (db.DriverKyc, error)
	SetDriverAvailability(ctx context.Context, arg db.SetDriverAvailabilityParams) (db.DriverAvailability, error)
	UpsertDriverLocation(ctx context.Context, arg db.UpsertDriverLocationParams) (db.DriverLocation, error)
	FindNearbyDrivers(ctx context.Context, arg db.FindNearbyDriversParams) ([]db.FindNearbyDriversRow, error)
	CreateDriverVehicleProfile(ctx context.Context, arg db.CreateDriverVehicleProfileParams) (db.DriverVehicleProfile, error)
	UpdateDriverVehicleProfile(ctx context.Context, arg db.UpdateDriverVehicleProfileParams) (db.DriverVehicleProfile, error)
	GetDriverVehicleProfile(ctx context.Context, driverID uuid.UUID) (db.DriverVehicleProfile, error)
}

// Handler implements driver location tracking, vehicle profiles, and nearby
// driver search.
type Handler struct {
	q queryStore
}

// NewHandler creates a driver handler wired to the DB queries.
func NewHandler(q queryStore) *Handler { return &Handler{q: q} }

type locationRequest struct {
	Lat     float64 `json:"lat" binding:"required"`
	Lng     float64 `json:"lng" binding:"required"`
	Heading float64 `json:"heading"`
}

type availabilityRequest struct {
	Online bool `json:"online"`
}

type vehicleProfileRequest struct {
	LicensePlate           string `json:"licensePlate"`
	VehiclePlate           string `json:"vehiclePlate"`
	Make                   string `json:"make" binding:"required"`
	Model                  string `json:"model" binding:"required"`
	Year                   int32  `json:"year" binding:"required"`
	Color                  string `json:"color" binding:"required"`
	LicenseExpiryDate      string `json:"licenseExpiryDate"`
	InsuranceExpiryDate    string `json:"insuranceExpiryDate"`
	RegistrationExpiryDate string `json:"registrationExpiryDate"`
	InspectionExpiryDate   string `json:"inspectionExpiryDate"`
	LicenseExpiry          string `json:"licenseExpiry"`
	InsuranceExpiry        string `json:"insuranceExpiry"`
	RegistrationExpiry     string `json:"registrationExpiry"`
	InspectionExpiry       string `json:"inspectionExpiry"`
}

var licensePlatePattern = regexp.MustCompile(`^(?:[A-Z]{2}-[0-9]{2}-[0-9]{2}-[A-Z]{2}|[A-Z]{2,3}[- ]?[0-9]{2,4}(?:[- ]?[A-Z]{1,3})?)$`)

// CreateVehicleProfile stores the authenticated driver's vehicle and document
// expiry details. A driver can have only one active profile.
func (h *Handler) CreateVehicleProfile(c *gin.Context) {
	driverID, ok := driverID(c)
	if !ok {
		return
	}
	req, params, err := parseVehicleProfileRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	profile, err := h.q.CreateDriverVehicleProfile(c.Request.Context(), db.CreateDriverVehicleProfileParams{
		DriverID: driverID, LicensePlate: req.licensePlate, Make: req.Make,
		Model: req.Model, Year: req.Year, Color: req.Color,
		LicenseExpiryDate: params.licenseExpiryDate, InsuranceExpiryDate: params.insuranceExpiryDate,
		RegistrationExpiryDate: params.registrationExpiryDate, InspectionExpiryDate: params.inspectionExpiryDate,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			c.JSON(http.StatusConflict, gin.H{"error": "vehicle profile already exists or license plate is in use"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create vehicle profile"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"vehicleProfile": VehicleProfileJSON(profile)})
}

// UpdateVehicleProfile replaces the authenticated driver's vehicle profile.
func (h *Handler) UpdateVehicleProfile(c *gin.Context) {
	driverID, ok := driverID(c)
	if !ok {
		return
	}
	req, params, err := parseVehicleProfileRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	profile, err := h.q.UpdateDriverVehicleProfile(c.Request.Context(), db.UpdateDriverVehicleProfileParams{
		DriverID: driverID, LicensePlate: req.licensePlate, Make: req.Make,
		Model: req.Model, Year: req.Year, Color: req.Color,
		LicenseExpiryDate: params.licenseExpiryDate, InsuranceExpiryDate: params.insuranceExpiryDate,
		RegistrationExpiryDate: params.registrationExpiryDate, InspectionExpiryDate: params.inspectionExpiryDate,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "vehicle profile not found"})
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			c.JSON(http.StatusConflict, gin.H{"error": "license plate is already in use"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update vehicle profile"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"vehicleProfile": VehicleProfileJSON(profile)})
}

// GetVehicleProfile returns the authenticated driver's vehicle profile.
func (h *Handler) GetVehicleProfile(c *gin.Context) {
	driverID, ok := driverID(c)
	if !ok {
		return
	}
	profile, err := h.q.GetDriverVehicleProfile(c.Request.Context(), driverID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "vehicle profile not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load vehicle profile"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"vehicleProfile": VehicleProfileJSON(profile)})
}

func (h *Handler) SetAvailability(c *gin.Context) {
	driverID, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(401, gin.H{"error": "unauthenticated"})
		return
	}
	var req availabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if req.Online {
		kyc, err := h.q.GetDriverKYC(c.Request.Context(), driverID)
		if err != nil || kyc.Status != "approved" {
			c.JSON(403, gin.H{"error": "driver verification must be approved before going online"})
			return
		}
	}
	status, err := h.q.SetDriverAvailability(c.Request.Context(), db.SetDriverAvailabilityParams{
		DriverID: driverID,
		IsOnline: req.Online,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to update availability"})
		return
	}
	c.JSON(200, gin.H{"online": status.IsOnline, "updatedAt": status.UpdatedAt})
}

// UpdateLocation records the authenticated driver's live position (ping).
func (h *Handler) UpdateLocation(c *gin.Context) {
	driverID, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(401, gin.H{"error": "unauthenticated"})
		return
	}
	var req locationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if math.IsNaN(req.Lat) || math.IsNaN(req.Lng) || req.Lat < -90 || req.Lat > 90 || req.Lng < -180 || req.Lng > 180 {
		c.JSON(400, gin.H{"error": "lat and lng must be valid coordinates"})
		return
	}
	row, err := h.q.UpsertDriverLocation(c.Request.Context(), db.UpsertDriverLocationParams{
		DriverID: driverID,
		Column2:  pgtype.Point{P: pgtype.Vec2{X: req.Lng, Y: req.Lat}, Valid: true},
		Heading:  decimalHeading(req.Heading),
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to save location"})
		return
	}
	c.JSON(200, gin.H{"status": "ok", "updatedAt": row.UpdatedAt})
}

// Nearby lists active drivers close to a coordinate, closest first.
// Used by riders and the matching service.
func (h *Handler) Nearby(c *gin.Context) {
	var q struct {
		Lat          float64 `form:"lat" binding:"required"`
		Lng          float64 `form:"lng" binding:"required"`
		RadiusMeters float64 `form:"radiusMeters"`
		Limit        int32   `form:"limit"`
	}
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(400, gin.H{"error": "lat and lng query params are required"})
		return
	}
	if math.IsNaN(q.Lat) || math.IsNaN(q.Lng) || q.Lat < -90 || q.Lat > 90 || q.Lng < -180 || q.Lng > 180 {
		c.JSON(400, gin.H{"error": "lat and lng must be valid coordinates"})
		return
	}
	if q.RadiusMeters <= 0 {
		q.RadiusMeters = 5000 // matching_radius_meters default
	}
	if q.Limit <= 0 || q.Limit > 50 {
		q.Limit = 10
	}
	rows, err := h.q.FindNearbyDrivers(c.Request.Context(), db.FindNearbyDriversParams{
		Limit:        q.Limit,
		Location:     pgtype.Point{P: pgtype.Vec2{X: q.Lng, Y: q.Lat}, Valid: true},
		RadiusMeters: q.RadiusMeters,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to search drivers"})
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"driverId":       r.DriverID,
			"name":           r.DriverName,
			"rating":         r.Rating,
			"lat":            r.Location.P.Y,
			"lng":            r.Location.P.X,
			"distanceMeters": r.DistanceMeters,
			"updatedAt":      r.UpdatedAt,
		})
	}
	c.JSON(200, gin.H{"drivers": out})
}

func decimalHeading(h float64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(int64(h * 10)), Exp: -1, Valid: true}
}

type vehicleProfileParams struct {
	licenseExpiryDate      pgtype.Date
	insuranceExpiryDate    pgtype.Date
	registrationExpiryDate pgtype.Date
	inspectionExpiryDate   pgtype.Date
}

type parsedVehicleProfileRequest struct {
	vehicleProfileRequest
	licensePlate string
}

func parseVehicleProfileRequest(c *gin.Context) (parsedVehicleProfileRequest, vehicleProfileParams, error) {
	var req vehicleProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return parsedVehicleProfileRequest{}, vehicleProfileParams{}, errors.New("invalid vehicle profile: make, model, year, color, licensePlate, licenseExpiryDate, and insuranceExpiryDate are required")
	}

	plate := strings.ToUpper(strings.TrimSpace(req.LicensePlate))
	if plate == "" {
		plate = strings.ToUpper(strings.TrimSpace(req.VehiclePlate))
	}
	if !licensePlatePattern.MatchString(plate) {
		return parsedVehicleProfileRequest{}, vehicleProfileParams{}, errors.New("license plate must use letters and numbers in a valid format")
	}
	if req.Year < 1950 || req.Year > int32(time.Now().Year()+1) {
		return parsedVehicleProfileRequest{}, vehicleProfileParams{}, errors.New("year must be between 1950 and next year")
	}
	req.Make = strings.TrimSpace(req.Make)
	req.Model = strings.TrimSpace(req.Model)
	req.Color = strings.TrimSpace(req.Color)
	if req.Make == "" || req.Model == "" || req.Color == "" {
		return parsedVehicleProfileRequest{}, vehicleProfileParams{}, errors.New("make, model, and color are required")
	}

	params := vehicleProfileParams{}
	var err error
	if params.licenseExpiryDate, err = parseExpiryDate(firstNonEmpty(req.LicenseExpiryDate, req.LicenseExpiry), true); err != nil {
		return parsedVehicleProfileRequest{}, vehicleProfileParams{}, err
	}
	if params.insuranceExpiryDate, err = parseExpiryDate(firstNonEmpty(req.InsuranceExpiryDate, req.InsuranceExpiry), true); err != nil {
		return parsedVehicleProfileRequest{}, vehicleProfileParams{}, err
	}
	if params.registrationExpiryDate, err = parseExpiryDate(firstNonEmpty(req.RegistrationExpiryDate, req.RegistrationExpiry), false); err != nil {
		return parsedVehicleProfileRequest{}, vehicleProfileParams{}, err
	}
	if params.inspectionExpiryDate, err = parseExpiryDate(firstNonEmpty(req.InspectionExpiryDate, req.InspectionExpiry), false); err != nil {
		return parsedVehicleProfileRequest{}, vehicleProfileParams{}, err
	}
	return parsedVehicleProfileRequest{vehicleProfileRequest: req, licensePlate: plate}, params, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseExpiryDate(value string, required bool) (pgtype.Date, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return pgtype.Date{}, errors.New("license and insurance expiry dates are required")
		}
		return pgtype.Date{}, nil
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return pgtype.Date{}, errors.New("expiry dates must use YYYY-MM-DD")
	}
	today := time.Now().UTC()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	if date.Before(today) {
		return pgtype.Date{}, errors.New("expiry dates cannot be in the past")
	}
	return pgtype.Date{Time: date, Valid: true}, nil
}

// VehicleProfileJSON keeps date-only database fields in the API's stable
// YYYY-MM-DD representation.
func VehicleProfileJSON(profile db.DriverVehicleProfile) gin.H {
	return gin.H{
		"driverId":               profile.DriverID,
		"licensePlate":           profile.LicensePlate,
		"make":                   profile.Make,
		"model":                  profile.Model,
		"year":                   profile.Year,
		"color":                  profile.Color,
		"licenseExpiryDate":      formatDate(profile.LicenseExpiryDate),
		"insuranceExpiryDate":    formatDate(profile.InsuranceExpiryDate),
		"registrationExpiryDate": formatDate(profile.RegistrationExpiryDate),
		"inspectionExpiryDate":   formatDate(profile.InspectionExpiryDate),
		"createdAt":              profile.CreatedAt,
		"updatedAt":              profile.UpdatedAt,
	}
}

// DocumentExpiryJSON is the compact expiry-only view used by dashboards.
func DocumentExpiryJSON(profile db.DriverVehicleProfile) gin.H {
	return gin.H{
		"license":      formatDate(profile.LicenseExpiryDate),
		"insurance":    formatDate(profile.InsuranceExpiryDate),
		"registration": formatDate(profile.RegistrationExpiryDate),
		"inspection":   formatDate(profile.InspectionExpiryDate),
	}
}

func formatDate(date pgtype.Date) any {
	if !date.Valid {
		return nil
	}
	return date.Time.Format("2006-01-02")
}

func driverID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return uuid.Nil, false
	}
	return id, true
}
