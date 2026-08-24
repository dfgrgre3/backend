package query

import (
	"context"
	"encoding/json"
	"log"

	"gorm.io/gorm"
)

// OptimizedSubjectDetailQuery fetches a subject with its full curriculum
// in an optimized way (single JSON-aggregated round-trip for the curriculum).
func (qo *QueryOptimizer) OptimizedSubjectDetailQuery(ctx context.Context, idOrSlug string) (*SubjectDetail, error) {
	var subject SubjectDetail

	// First, resolve ID from slug in a cheap scan
	var subjectID string
	err := qo.db.WithContext(ctx).
		Table("Subject").
		Select("id").
		Where("id = ? OR slug = ?", idOrSlug, idOrSlug).
		Scan(&subjectID).Error
	if err != nil || subjectID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	// Fetch subject with selected columns — hits covering index
	err = qo.db.WithContext(ctx).
		Table("Subject").
		Select(`
			id, name, name_ar, code, description, icon, color,
			is_active, is_published, price, level, instructor_name,
			instructor_id, category_id, thumbnail_url, trailer_url,
			trailer_duration_minutes, duration_hours, requirements,
			learning_objectives, seo_title, seo_description, slug,
			rating, enrolled_count, created_at, updated_at,
			status, version, short_description, long_description,
			is_featured, is_trending, is_new, is_free, has_certificate,
			available_from, available_until, new_until,
			course_prerequisites, target_audience, what_you_learn
		`).
		Where("id = ?", subjectID).
		Scan(&subject).Error
	if err != nil {
		return nil, err
	}

	// Fetch curriculum in a single JSON-aggregated query
	curriculum, err := qo.fetchCurriculum(ctx, subjectID)
	if err != nil {
		return nil, err
	}
	subject.Curriculum = curriculum

	return &subject, nil
}

// fetchCurriculum retrieves topics and their subtopics for a subject using a
// single JSON-aggregated query to avoid N+1 round-trips.
func (qo *QueryOptimizer) fetchCurriculum(ctx context.Context, subjectID string) ([]TopicWithSubTopics, error) {
	query := `
		SELECT json_agg(topic_data ORDER BY topic_order) AS curriculum
		FROM (
			SELECT
				jsonb_build_object(
					'id',          t.id,
					'subject_id',  t.subject_id,
					'title',       t.title,
					'description', t.description,
					'order',       t."order",
					'created_at',  t.created_at,
					'updated_at',  t.updated_at,
					'sub_topics',  COALESCE(st.sub_topics, '[]'::jsonb)
				) AS topic_data,
				t."order" AS topic_order
			FROM "Topic" t
			LEFT JOIN LATERAL (
				SELECT json_agg(
					jsonb_build_object(
						'id',                   st.id,
						'topic_id',             st.topic_id,
						'title',                st.title,
						'description',          st.description,
						'video_url',            st.video_url,
						'audio_url',            st.audio_url,
						'audio_duration_seconds', st.audio_duration_seconds,
						'external_link_url',    st.external_link_url,
						'external_link_title',  st.external_link_title,
						'type',                 st.type,
						'is_free',              st.is_free,
						'order',                st."order",
						'duration_minutes',     st.duration_minutes,
						'exam_id',              st.exam_id,
						'is_drip_enabled',      st.is_drip_enabled,
						'drip_release_date',    st.drip_release_date,
						'is_content_protected', st.is_content_protected,
						'view_count',           st.view_count,
						'completion_count',     st.completion_count,
						'subtitle_urls',        st.subtitle_urls,
						'video_chapters_data',  st.video_chapters_data
					) ORDER BY st."order"
				) AS sub_topics
				FROM "SubTopic" st
				WHERE st.topic_id = t.id
			) st ON true
			WHERE t.subject_id = ?
		) topics`

	// Scan into a raw JSON byte slice first, then unmarshal.
	var curriculumJSON []byte
	if err := qo.db.WithContext(ctx).Raw(query, subjectID).Scan(&curriculumJSON).Error; err != nil {
		return nil, err
	}

	if len(curriculumJSON) == 0 || string(curriculumJSON) == "null" {
		return []TopicWithSubTopics{}, nil
	}

	var curriculum []TopicWithSubTopics
	if err := json.Unmarshal(curriculumJSON, &curriculum); err != nil {
		log.Printf("[WARN] fetchCurriculum: failed to unmarshal curriculum JSON for subject %s: %v", subjectID, err)
		return []TopicWithSubTopics{}, nil
	}

	return curriculum, nil
}
