package types

type VideoBillingParams struct {
	Tier            string
	DurationSeconds int
	AudioEnabled    bool
	PriceKey        string
	ExtraUnits      map[string]int
}
