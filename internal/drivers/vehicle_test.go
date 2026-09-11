package drivers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
)

type vehicleQueryMock struct {
	created db.DriverVehicleProfile
	creates int
}

func (m *vehicleQueryMock) GetDriverKYC(context.Context, uuid.UUID) (db.DriverKyc, error) {
	return db.DriverKyc{}, nil
}

func (m *vehicleQueryMock) SetDriverAvailability(context.Context, db.SetDriverAvailabilityParams) (db.DriverAvailability, error) {
	return db.DriverAvailability{}, nil
}

func (m *vehicleQueryMock) UpsertDriverLocation(context.Context, db.UpsertDriverLocationParams) (db.DriverLocation, error) {
	return db.DriverLocation{}, nil
}

func (m *vehicleQueryMock) FindNearbyDrivers(context.Context, db.FindNearbyDriversParams) ([]db.FindNearbyDriversRow, error) {
	return nil, nil
}

func (m *vehicleQueryMock) CreateDriverVehicleProfile(_ context.Context, arg db.CreateDriverVehicleProfileParams) (db.DriverVehicleProfile, error) {
	m.creates++
	m.created = db.DriverVehicleProfile{
		DriverID: arg.DriverID, LicensePlate: arg.LicensePlate, Make: arg.Make,
		Model: arg.Model, Year: arg.Year, Color: arg.Color,
		LicenseExpiryDate: arg.LicenseExpiryDate, InsuranceExpiryDate: arg.InsuranceExpiryDate,
		RegistrationExpiryDate: arg.RegistrationExpiryDate, InspectionExpiryDate: arg.InspectionExpiryDate,
	}
	return m.created, nil
}

func (m *vehicleQueryMock) UpdateDriverVehicleProfile(context.Context, db.UpdateDriverVehicleProfileParams) (db.DriverVehicleProfile, error) {
	return m.created, nil
}

func (m *vehicleQueryMock) GetDriverVehicleProfile(context.Context, uuid.UUID) (db.DriverVehicleProfile, error) {
	return m.created, nil
}

func vehicleTestContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/drivers/vehicle-profile", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(auth.ContextUserID, uuid.New().String())
	return c, recorder
}

func futureDate() string {
	return time.Now().UTC().AddDate(1, 0, 0).Format("2006-01-02")
}

func TestCreateVehicleProfileValidatesPlateAndExpiryDates(t *testing.T) {
	mock := &vehicleQueryMock{}
	handler := NewHandler(mock)
	c, recorder := vehicleTestContext(`{
		"licensePlate":"LD-12-34-AB",
		"make":"Toyota","model":"Corolla","year":2020,"color":"white",
		"licenseExpiryDate":"` + futureDate() + `",
		"insuranceExpiryDate":"` + futureDate() + `"
	}`)

	handler.CreateVehicleProfile(c)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if mock.creates != 1 || mock.created.LicensePlate != "LD-12-34-AB" {
		t.Fatalf("profile was not persisted: %+v", mock.created)
	}
	var response map[string]map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["vehicleProfile"]["licenseExpiryDate"] != futureDate() {
		t.Fatalf("expiry date was not serialized as a date: %v", response)
	}
}

func TestCreateVehicleProfileRejectsInvalidPlateAndExpiredDate(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"plate", `{"licensePlate":"not a plate","make":"Toyota","model":"Corolla","year":2020,"color":"white","licenseExpiryDate":"` + futureDate() + `","insuranceExpiryDate":"` + futureDate() + `"}`},
		{"expiry format", `{"licensePlate":"LD-12-34-AB","make":"Toyota","model":"Corolla","year":2020,"color":"white","licenseExpiryDate":"2030/01/01","insuranceExpiryDate":"` + futureDate() + `"}`},
		{"expired", `{"licensePlate":"LD-12-34-AB","make":"Toyota","model":"Corolla","year":2020,"color":"white","licenseExpiryDate":"2000-01-01","insuranceExpiryDate":"` + futureDate() + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &vehicleQueryMock{}
			handler := NewHandler(mock)
			c, recorder := vehicleTestContext(tt.body)
			handler.CreateVehicleProfile(c)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
			if mock.creates != 0 {
				t.Fatal("invalid profile was sent to the database")
			}
		})
	}
}

func TestVehicleProfileJSONOmitsOptionalExpiryDates(t *testing.T) {
	profile := db.DriverVehicleProfile{
		LicenseExpiryDate:   pgtype.Date{Time: time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC), Valid: true},
		InsuranceExpiryDate: pgtype.Date{Time: time.Date(2030, 2, 3, 0, 0, 0, 0, time.UTC), Valid: true},
	}
	jsonValue := VehicleProfileJSON(profile)
	if jsonValue["licenseExpiryDate"] != "2030-01-02" || jsonValue["registrationExpiryDate"] != nil {
		t.Fatalf("unexpected date output: %#v", jsonValue)
	}
}
