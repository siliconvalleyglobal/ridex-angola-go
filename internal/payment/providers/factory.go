// Package providers implements payment provider adapters for RideX Angola.
// Each adapter translates between the provider-neutral payment ledger and
// a specific payment provider's API.
package providers

import (
	"fmt"
	"go.uber.org/zap"
)

// Factory creates payment provider instances based on configuration.
type Factory struct {
	logger *zap.Logger
}

// NewFactory creates a new provider factory.
func NewFactory(logger *zap.Logger) *Factory {
	return &Factory{logger: logger}
}

// CreateAppyPay creates an AppyPay client from config.
func (f *Factory) CreateAppyPay(clientID, clientSecret, baseURL string, gpoEnabled bool) *AppyPayClient {
	f.logger.Info("creating AppyPay client",
		zap.Bool("configured", clientID != "" && clientSecret != ""),
		zap.String("baseURL", baseURL),
		zap.Bool("gpoEnabled", gpoEnabled),
	)

	if clientID == "" || clientSecret == "" {
		f.logger.Warn("AppyPay client not fully configured")
		return nil
	}

	return NewAppyPayClient(AppyPayConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		BaseURL:      baseURL,
		GPONEnabled:  gpoEnabled,
	}, f.logger)
}

// CreateVPOS creates a VPOS client from config.
func (f *Factory) CreateVPOS(developerID, apiKey, baseURL, webhookSecret string) *VPOSClient {
	f.logger.Info("creating VPOS client",
		zap.Bool("configured", developerID != "" && apiKey != ""),
		zap.String("baseURL", baseURL),
	)

	if developerID == "" || apiKey == "" {
		f.logger.Warn("VPOS client not fully configured")
		return nil
	}

	return NewVPOSClient(VPOSConfig{
		DeveloperID:   developerID,
		APIKey:        apiKey,
		BaseURL:       baseURL,
		WebhookSecret: webhookSecret,
	}, f.logger)
}

// CreateProxyPay creates a ProxyPay client from config.
func (f *Factory) CreateProxyPay(apiKey, baseURL string) *ProxyPayClient {
	f.logger.Info("creating ProxyPay client",
		zap.Bool("configured", apiKey != ""),
		zap.String("baseURL", baseURL),
	)

	if apiKey == "" {
		f.logger.Warn("ProxyPay client not fully configured")
		return nil
	}

	return NewProxyPayClient(ProxyPayConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
	}, f.logger)
}

// CreateRegistry creates a provider registry and registers all configured providers.
func (f *Factory) CreateRegistry(appyPay *AppyPayClient, vpos *VPOSClient, proxyPay *ProxyPayClient) (*Registry, error) {
	registry := NewRegistry(f.logger)

	if appyPay != nil {
		registry.Register("appypay", appyPay)
	}

	if vpos != nil {
		registry.Register("vpos", vpos)
	}

	if proxyPay != nil {
		registry.Register("proxypay", proxyPay)
	}

	if len(registry.List()) == 0 {
		return nil, fmt.Errorf("no payment providers configured")
	}

	return registry, nil
}
