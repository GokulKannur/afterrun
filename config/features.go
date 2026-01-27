package config

import "os"

type Features struct {
	AuthEnabled      bool
	BillingEnabled   bool
	BaselinesEnabled bool
	WriteUIEnabled   bool
}

func LoadFeatures() Features {
	return Features{
		AuthEnabled:      os.Getenv("AUTH_ENABLED") == "true",
		BillingEnabled:   os.Getenv("BILLING_ENABLED") == "true",
		BaselinesEnabled: os.Getenv("BASELINES_ENABLED") == "true",
		WriteUIEnabled:   os.Getenv("WRITE_UI_ENABLED") == "true",
	}
}
