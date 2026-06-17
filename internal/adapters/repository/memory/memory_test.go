package memory

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/wukfit/equipment-exposure-service/internal/contracttest"
	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

func TestExposureRepoContract(t *testing.T) {
	contracttest.RunExposureRepository(t, func() domain.ExposureRepository {
		return NewExposureRepo()
	})
}

func TestEquipmentRepoGetMissing(t *testing.T) {
	repo := NewEquipmentRepo()
	_, err := repo.GetByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrEquipmentNotFound)
}

func TestUserRepoGetMissing(t *testing.T) {
	repo := NewUserRepo()
	_, err := repo.GetByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}
