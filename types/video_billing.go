package types

type VideoBillingParams struct {
	Tier            string
	DurationSeconds int
	AudioEnabled    bool
	ExtraUnits      map[string]int
}
