package ai

import (
	"context"
	"strings"

	"github.com/your-org/job-scheduler/internal/platform/logger"
)

// Service provides AI-generated summaries for job failures.
// In a real system, this would call OpenAI/Gemini APIs.
type Service struct {
	log *logger.Logger
}

func NewService(log *logger.Logger) *Service {
	return &Service{log: log.WithField("service", "ai-summarizer")}
}

func (s *Service) SummarizeFailure(ctx context.Context, jobType, errorMessage, stackTrace string) (string, error) {
	s.log.Info("simulating AI failure summarization")

	// Mock implementation based on common error patterns
	lowerErr := strings.ToLower(errorMessage)
	if strings.Contains(lowerErr, "connection refused") || strings.Contains(lowerErr, "timeout") {
		return "AI Analysis: The job failed due to a network timeout or unreachable downstream service. Recommendation: Verify that the target service is online and accessible from the worker node.", nil
	}

	if strings.Contains(lowerErr, "syntax") || strings.Contains(lowerErr, "parse") {
		return "AI Analysis: The job encountered a data formatting or syntax error while processing the payload. Recommendation: Check the payload schema and ensure all required fields are correctly typed.", nil
	}

	if strings.Contains(lowerErr, "unauthorized") || strings.Contains(lowerErr, "forbidden") {
		return "AI Analysis: Authentication failed when the job attempted to access an external resource. Recommendation: Check the API credentials or token expiration.", nil
	}

	return "AI Analysis: The job failed with an unexpected error. Recommendation: Review the stack trace for application-specific logic errors and consider adding more contextual logging to the handler.", nil
}
