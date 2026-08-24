package repositories

import (
	"context"
)

// Section operations
func (r *GormRepository) CreateSection(ctx context.Context, section *Section) error {
	return r.repo.CreateSection(r.toModelSection(section))
}

func (r *GormRepository) GetSectionByID(ctx context.Context, id string) (*Section, error) {
	sectionUUID, err := parseUUID(id)
	if err != nil {
		return nil, err
	}
	modelSection, err := r.repo.GetSectionByID(sectionUUID)
	if err != nil {
		return nil, err
	}
	return r.toDomainSection(modelSection), nil
}

func (r *GormRepository) UpdateSection(ctx context.Context, section *Section) error {
	return r.repo.UpdateSection(r.toModelSection(section))
}

func (r *GormRepository) DeleteSection(ctx context.Context, id string) error {
	sectionUUID, err := parseUUID(id)
	if err != nil {
		return err
	}
	return r.repo.DeleteSection(sectionUUID)
}

func (r *GormRepository) ListSections(ctx context.Context, courseID string) ([]*Section, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	modelSections, err := r.repo.ListSectionsByCourseID(courseUUID)
	if err != nil {
		return nil, err
	}

	sections := make([]*Section, len(modelSections))
	for i, ms := range modelSections {
		sections[i] = r.toDomainSection(&ms)
	}
	return sections, nil
}

func (r *GormRepository) ReorderSections(ctx context.Context, courseID string, sectionIDs []string) error {
	// Implementation would update order_index for each section
	return nil
}
