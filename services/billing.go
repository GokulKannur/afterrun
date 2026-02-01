package services

import "strings"

const (
	PlanFree  = "free"
	PlanIndie = "indie"
	PlanTeam  = "team"

	LimitFree  = 5
	LimitIndie = 10
	LimitTeam  = 50
)

func GetJobLimit(tier string) int {
	switch strings.ToLower(tier) {
	case PlanIndie:
		return LimitIndie
	case PlanTeam:
		return LimitTeam
	case PlanFree:
		return LimitFree
	default:
		// Default to Free limit for unknown plans or empty strings
		return LimitFree
	}
}

func IsValidPlan(plan string) bool {
	p := strings.ToLower(plan)
	return p == PlanIndie || p == PlanTeam
}
