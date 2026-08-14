package aieval

// Honest launch gates. These are contract checks, not live-model accuracy claims.
const (
	EmptyAnswerMustScoreZero       = true
	MaxFollowUpsPerMainQuestion    = 2
	MaxFollowUpBudget              = 5
	MaxToolCallsPerTurn            = 2
	MaxInvalidStructuredRate       = 0.05
	MaxDuplicateQuestionRate       = 0.15
	MinMaterialRelatedQuestionRate = 0.70
	MaxModelErrorRate              = 0.20
	MaxSingleCallCostMicros        = 50_000
)
