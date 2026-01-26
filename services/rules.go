package services

import (
	"cronmonitor/models"
)

func toFloat64(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	default:
		return 0, false
	}
}

func EvaluateRule(metrics map[string]interface{}, rule models.Rule) (bool, float64) {
	value, ok := metrics[rule.MetricName]
	if !ok {
		return false, 0
	}

	numValue, ok := toFloat64(value)
	if !ok {
		return false, 0
	}

	violated := false
	switch rule.Operator {
	case "==":
		violated = (numValue == rule.ThresholdValue)
	case "<":
		violated = (numValue < rule.ThresholdValue)
	case ">":
		violated = (numValue > rule.ThresholdValue)
	case "!=":
		violated = (numValue != rule.ThresholdValue)
	}

	return violated, numValue
}
