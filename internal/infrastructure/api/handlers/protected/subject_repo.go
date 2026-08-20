package protected

import (
	"sync"
	db "thanawy-backend/internal/infrastructure/database"
	courserepo "thanawy-backend/internal/infrastructure/persistence/repositories"
)

const (
	preloadTopicsSubTopics = "Topics.SubTopics"
	preloadAdvanced        = "Topics.SubTopics.Attachments"
	subjectIDQuery         = "subject_id = ?"
)

var (
	subjectRepo     *courserepo.SubjectRepository
	subjectRepoOnce sync.Once
)

func getSubjectRepo() *courserepo.SubjectRepository {
	subjectRepoOnce.Do(func() {
		subjectRepo = courserepo.NewSubjectRepository(db.DB)
	})
	return subjectRepo
}
