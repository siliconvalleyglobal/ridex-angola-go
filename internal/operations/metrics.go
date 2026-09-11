package operations

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// OperationsMetrics represents real-time operational metrics.
type OperationsMetrics struct {
	ActiveRides       int
	AvailableDrivers  int
	BusyDrivers       int
	RequestsPerMinute float64
	AvgWaitTimeSec    float64
	CompletionRate    float64
	CancellationRate  float64
	RevenuePerHour    int64
	ActiveUsers       int
	Timestamp         time.Time
}

// ZoneMetrics represents metrics for a specific zone.
type ZoneMetrics struct {
	ZoneID           uuid.UUID
	ZoneName         string
	ActiveRiders     int
	AvailableDrivers int
	BusyDrivers      int
	AvgWaitTimeSec   float64
	SurgeMultiplier  float64
	DemandTrend      string
	RevenuePerHour   int64
}

// DemandForecast represents predicted demand.
type DemandForecast struct {
	ZoneID          uuid.UUID
	ForecastTime    time.Time
	PredictedDemand float64
	Confidence      float64
	Factors         []string
}

// MetricsCollector collects and aggregates operational metrics.
type MetricsCollector struct {
	mu         sync.RWMutex
	metrics    OperationsMetrics
	zones      map[uuid.UUID]*ZoneMetrics
	history    []OperationsMetrics
	maxHistory int
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		zones:      make(map[uuid.UUID]*ZoneMetrics),
		history:    make([]OperationsMetrics, 0, 1000),
		maxHistory: 1000,
	}
}

// UpdateMetrics updates the overall operational metrics.
func (c *MetricsCollector) UpdateMetrics(metrics OperationsMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics = metrics
	c.metrics.Timestamp = time.Now()

	// Add to history
	c.history = append(c.history, metrics)
	if len(c.history) > c.maxHistory {
		c.history = c.history[1:]
	}
}

// GetMetrics returns the current operational metrics.
func (c *MetricsCollector) GetMetrics() OperationsMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.metrics
}

// UpdateZoneMetrics updates metrics for a specific zone.
func (c *MetricsCollector) UpdateZoneMetrics(zone ZoneMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.zones[zone.ZoneID] = &zone
}

// GetZoneMetrics returns metrics for all zones.
func (c *MetricsCollector) GetZoneMetrics() []ZoneMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	zones := make([]ZoneMetrics, 0, len(c.zones))
	for _, z := range c.zones {
		zones = append(zones, *z)
	}
	return zones
}

// GetHotspots returns zones with high demand.
func (c *MetricsCollector) GetHotspots() []ZoneMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hotspots := make([]ZoneMetrics, 0)
	for _, z := range c.zones {
		if z.ActiveRiders > z.AvailableDrivers*2 {
			hotspots = append(hotspots, *z)
		}
	}
	return hotspots
}

// GetDriverShortages returns zones with driver shortages.
func (c *MetricsCollector) GetDriverShortages() []ZoneMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	shortages := make([]ZoneMetrics, 0)
	for _, z := range c.zones {
		if z.AvailableDrivers < 3 && z.ActiveRiders > 5 {
			shortages = append(shortages, *z)
		}
	}
	return shortages
}

// CalculateRevenuePerHour calculates revenue per hour from history.
func (c *MetricsCollector) CalculateRevenuePerHour() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.history) < 2 {
		return 0
	}

	totalRevenue := int64(0)
	for _, m := range c.history {
		totalRevenue += m.RevenuePerHour
	}

	return float64(totalRevenue) / float64(len(c.history))
}
