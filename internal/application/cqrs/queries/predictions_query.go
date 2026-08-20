package queries

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"gorm.io/gorm"

	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"
)

// MilestoneReadModel is a dated checkpoint on a learner's projected path.
type MilestoneReadModel struct {
	Date   string `json:"date"`
	Goal   string `json:"goal"`
	Status string `json:"status"`
}

// PredictionReadModel projects a learner's score for one horizon.
type PredictionReadModel struct {
	Period          string               `json:"period"`
	PredictedScore  int                  `json:"predictedScore"`
	Confidence      int                  `json:"confidence"`
	Milestones      []MilestoneReadModel `json:"milestones"`
	Recommendations []string             `json:"recommendations"`
}

// PredictionsQuerier defines the contract for fetching performance predictions.
type PredictionsQuerier interface {
	GetPredictions(userID string) ([]PredictionReadModel, error)
}

// PredictionsQueryService derives forward-looking projections from exam history
// and study consistency. It never invents a projection without evidence.
type PredictionsQueryService struct{}

// Compile-time check to ensure PredictionsQueryService implements PredictionsQuerier.
var _ PredictionsQuerier = (*PredictionsQueryService)(nil)

// NewPredictionsQueryService creates a new instance of PredictionsQueryService.
func NewPredictionsQueryService() *PredictionsQueryService {
	return &PredictionsQueryService{}
}

func (s *PredictionsQueryService) readDB() *gorm.DB {
	return db.ReadDB()
}

// examDataPoint is one graded attempt used as a regression sample.
type examDataPoint struct {
	TakenAt time.Time `gorm:"column:taken_at"`
	Score   float64   `gorm:"column:score"`
}

const (
	// Data fetching constants
	minSamplesForPrediction  = 5 // Fix #1: Raised from 3 to avoid overfitting
	minSamplesForReliableFit = 7 // Fix #1: Threshold for full confidence
	predictionLookbackDays   = 90
	activeDaysLookback       = 30
	maxExamPoints            = 500 // Fix #10: Protect against massive data

	// Confidence score weights and thresholds
	// Note #6: These weights are empirically chosen and should be validated via backtesting.
	maxSampleCountForFullWeight = 15.0
	maxActiveDaysForFullWeight  = 20.0
	decayRate                   = 0.015 // Fix #3: Exponential decay rate (~90 days = 25% penalty)
	weightSamples               = 0.30
	weightFitQuality            = 0.40 // Fit quality is most important
	weightConsistency           = 0.30
	minConfidence               = 10
	maxConfidence               = 95

	// Recommendation thresholds
	slopeImprovementThreshold  = 0.05
	slopeDeclineThreshold      = -0.05
	lowActiveDaysThreshold     = 15
	targetActiveDays           = 20
	lowPredictedScoreThreshold = 60.0
)

/*
⚠️ REQUIRED DATABASE INDEXES FOR PERFORMANCE
Ensure the following composite indexes exist to prevent full table scans:
1. CREATE INDEX idx_exam_results_user_taken_at ON exam_results(user_id, taken_at);
2. CREATE INDEX idx_study_sessions_user_start_time ON study_sessions(user_id, start_time);
*/

// GetPredictions returns projections for 30/60/90 day horizons.
func (s *PredictionsQueryService) GetPredictions(userID string) ([]PredictionReadModel, error) {
	// Fix #7: Validate userID to prevent accidental full-table scans
	if userID == "" {
		return nil, errors.New("userID cannot be empty")
	}

	rdb := s.readDB()
	if rdb == nil {
		return nil, errors.New("database connection is not initialized")
	}

	// Capture current time once in UTC to ensure absolute consistency.
	now := time.Now().UTC()

	var points []examDataPoint
	var activeDaysResult sql.NullInt64 // Fix #8: Use NullInt64 to distinguish error from 0
	var wg sync.WaitGroup
	var errExam, errSession error

	// Run independent queries concurrently with panic recovery
	wg.Add(2)

	// Goroutine 1: Exam Results
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errExam = fmt.Errorf("panic in exam query: %v", r)
			}
			wg.Done()
		}()
		// Fix: Use separate session to avoid concurrent map writes
		examRdb := rdb.Session(&gorm.Session{NewDB: true})
		errExam = examRdb.Model(&models.ExamResult{}).
			Where("user_id = ? AND deleted_at IS NULL AND taken_at >= ?", userID, now.AddDate(0, 0, -predictionLookbackDays)).
			Order("taken_at ASC").
			Limit(maxExamPoints). // Fix #10: Protect against massive result sets
			Select("taken_at, score").
			Scan(&points).Error
	}()

	// Goroutine 2: Active Days
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errSession = fmt.Errorf("panic in session query: %v", r)
			}
			wg.Done()
		}()
		// Fix: Use separate session to avoid concurrent map writes
		sessionRdb := rdb.Session(&gorm.Session{NewDB: true})
		errSession = sessionRdb.Model(&models.StudySession{}).
			Where("user_id = ? AND start_time >= ?", userID, now.AddDate(0, 0, -activeDaysLookback)).
			Select("COUNT(DISTINCT DATE(start_time))").
			Scan(&activeDaysResult).Error
	}()

	wg.Wait()

	// Fix #12: Check both errors comprehensively
	if errExam != nil {
		slog.Error("failed to fetch exam results for predictions", "userID", userID, "error", errExam)
		return nil, errExam
	}
	if errSession != nil {
		slog.Error("failed to fetch active days for predictions", "userID", userID, "error", errSession)
		return nil, errSession
	}

	// Extract activeDays safely
	activeDays := 0
	if activeDaysResult.Valid {
		activeDays = int(activeDaysResult.Int64)
	}

	if len(points) < minSamplesForPrediction {
		return []PredictionReadModel{}, nil
	}

	slope, interceptAtOrigin := linearFit(points)
	origin := points[0].TakenAt

	// Fix #2: Calculate current score at time.Now() instead of using old origin
	daysFromOriginToNow := now.Sub(origin).Hours() / 24
	currentScore := interceptAtOrigin + slope*daysFromOriginToNow

	// Fix #4: Use Adjusted R² to penalize small sample sizes
	rawFitQuality := rSquared(points, slope, interceptAtOrigin)
	fitQuality := adjustedRSquared(points, rawFitQuality)

	horizons := []struct {
		days  int
		label string
	}{
		{30, "الشهر القادم"},
		{60, "الشهرين القادمين"},
		{90, "الفصل القادم"},
	}

	predictions := make([]PredictionReadModel, 0, len(horizons))

	for _, h := range horizons {
		// Fix #2: Predict from now, not from old origin
		rawPredicted := currentScore + slope*float64(h.days)

		// Fix #5: Log warning if model produces unrealistic predictions
		if rawPredicted > 100 || rawPredicted < 0 {
			slog.Warn("prediction model produced out-of-range value",
				"userID", userID,
				"rawPredicted", rawPredicted,
				"horizon", h.days,
				"slope", slope)
		}

		predicted := clampScore(rawPredicted)
		confidence := confidenceScore(len(points), fitQuality, h.days, activeDays)

		predictions = append(predictions, PredictionReadModel{
			Period:          h.label,
			PredictedScore:  int(math.Round(predicted)),
			Confidence:      confidence,
			Milestones:      buildMilestones(h.days, currentScore, slope, now), // Fix #9: Pass currentScore
			Recommendations: buildRecommendations(slope, activeDays, predicted),
		})
	}

	return predictions, nil
}

// linearFit performs least-squares regression of score against days elapsed
// since the first sample.
func linearFit(points []examDataPoint) (slope, intercept float64) {
	origin := points[0].TakenAt
	n := float64(len(points))

	var sumX, sumY, sumXY, sumXX float64
	for _, p := range points {
		x := p.TakenAt.Sub(origin).Hours() / 24
		sumX += x
		sumY += p.Score
		sumXY += x * p.Score
		sumXX += x * x
	}

	denominator := n*sumXX - sumX*sumX
	if math.Abs(denominator) < 1e-9 {
		return 0, sumY / n
	}

	slope = (n*sumXY - sumX*sumY) / denominator
	intercept = (sumY - slope*sumX) / n
	return slope, intercept
}

// rSquared reports how well the fitted line explains the observed scores.
func rSquared(points []examDataPoint, slope, intercept float64) float64 {
	origin := points[0].TakenAt
	var sumY float64
	for _, p := range points {
		sumY += p.Score
	}
	mean := sumY / float64(len(points))

	var residual, total float64
	for _, p := range points {
		x := p.TakenAt.Sub(origin).Hours() / 24
		predicted := intercept + slope*x
		residual += (p.Score - predicted) * (p.Score - predicted)
		total += (p.Score - mean) * (p.Score - mean)
	}

	if total < 1e-9 {
		return 1
	}
	fit := 1 - residual/total
	if fit < 0 {
		return 0
	}
	return fit
}

// adjustedRSquared applies a penalty for small sample sizes to prevent overfitting confidence.
// Formula: Adjusted R² = 1 - [(1-R²)(n-1)/(n-k-1)] where k=1 (one predictor).
func adjustedRSquared(points []examDataPoint, r2 float64) float64 {
	n := float64(len(points))
	k := 1.0 // Number of independent variables

	if n-k-1 <= 0 {
		return r2
	}

	adjusted := 1 - (1-r2)*(n-1)/(n-k-1)
	if adjusted < 0 {
		return 0
	}
	return adjusted
}

// confidenceScore blends sample size, fit quality, horizon distance and study
// consistency into a single percentage using exponential decay for realism.
func confidenceScore(sampleCount int, fitQuality float64, horizonDays, activeDays int) int {
	sampleWeight := math.Min(float64(sampleCount)/maxSampleCountForFullWeight, 1.0)

	// Fix #3: Exponential decay instead of linear penalty
	// e^(-0.015 * 90) ≈ 0.26, meaning 90-day predictions lose ~74% confidence
	horizonPenalty := math.Exp(-decayRate * float64(horizonDays))

	consistencyWeight := math.Min(float64(activeDays)/maxActiveDaysForFullWeight, 1.0)

	// Bonus for having enough samples for reliable fit
	reliabilityBonus := 1.0
	if sampleCount >= minSamplesForReliableFit {
		reliabilityBonus = 1.1 // 10% bonus for sufficient data
	}

	confidence := (sampleWeight*weightSamples + fitQuality*weightFitQuality + consistencyWeight*weightConsistency) * horizonPenalty * reliabilityBonus * 100

	if confidence < minConfidence {
		confidence = minConfidence
	}
	if confidence > maxConfidence {
		confidence = maxConfidence
	}

	return int(math.Round(confidence))
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

// buildMilestones lays out monthly checkpoints progressing forward from today.
// Fix #9: Calculate targets progressively from current score instead of regressing from end.
func buildMilestones(horizonDays int, currentScore, slope float64, now time.Time) []MilestoneReadModel {
	milestones := make([]MilestoneReadModel, 0, horizonDays/30)

	for offset := 30; offset <= horizonDays; offset += 30 {
		date := now.AddDate(0, 0, offset)
		// Calculate target progressively: current + slope * days_forward
		target := clampScore(currentScore + slope*float64(offset))

		status := "upcoming"
		if offset == 30 {
			status = "current"
		}

		milestones = append(milestones, MilestoneReadModel{
			Date:   date.Format("2006-01-02"),
			Goal:   fmt.Sprintf("الوصول إلى %d%% في متوسط الدرجات", int(math.Round(target))),
			Status: status,
		})
	}

	return milestones
}

// buildRecommendations turns the measured trend into concrete guidance.
func buildRecommendations(slope float64, activeDays int, predicted float64) []string {
	recommendations := make([]string, 0, 3)

	switch {
	case slope > slopeImprovementThreshold:
		recommendations = append(recommendations, "أداؤك في تحسن مستمر، حافظ على نفس وتيرة المذاكرة الحالية")
	case slope < slopeDeclineThreshold:
		recommendations = append(recommendations, "درجاتك في انخفاض، راجع الدروس السابقة قبل الانتقال لمحتوى جديد")
	default:
		recommendations = append(recommendations, "أداؤك مستقر، زد صعوبة التدريبات لتحقيق تقدم ملحوظ")
	}

	if activeDays < lowActiveDaysThreshold {
		recommendations = append(recommendations,
			fmt.Sprintf("ذاكرت %d يوم فقط خلال الشهر الماضي، استهدف %d يوماً على الأقل", activeDays, targetActiveDays))
	} else {
		recommendations = append(recommendations, "انتظامك في المذاكرة جيد، استمر على نفس الجدول")
	}

	if predicted < lowPredictedScoreThreshold {
		recommendations = append(recommendations, "ركز على حل امتحانات تجريبية إضافية لرفع متوسط درجاتك")
	}

	return recommendations
}
