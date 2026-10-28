package mocks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/products"
	"github.com/stretchr/testify/mock"
)

// Repo is a testify mock for the product repository interface.
type Repo struct {
	mock.Mock
}

func (_m *Repo) InsertProduct(ctx context.Context, p *models.Product) (models.Product, error) {
	ret := _m.Called(ctx, p)
	if fn, ok := ret.Get(0).(func(context.Context, *models.Product) (models.Product, error)); ok {
		return fn(ctx, p)
	}
	var result models.Product
	if ret.Get(0) != nil {
		result = ret.Get(0).(models.Product)
	}
	return result, ret.Error(1)
}

func (_m *Repo) InsertImageUrl(ctx context.Context, image *models.Images) (models.Images, error) {
	ret := _m.Called(ctx, image)
	if fn, ok := ret.Get(0).(func(context.Context, *models.Images) (models.Images, error)); ok {
		return fn(ctx, image)
	}
	var result models.Images
	if ret.Get(0) != nil {
		result = ret.Get(0).(models.Images)
	}
	return result, ret.Error(1)
}

func (_m *Repo) FetchProductByName(ctx context.Context, keyword string, page int) ([]models.Product, int, int, error) {
	ret := _m.Called(ctx, keyword, page)
	if fn, ok := ret.Get(0).(func(context.Context, string, int) ([]models.Product, int, int, error)); ok {
		return fn(ctx, keyword, page)
	}
	var result []models.Product
	if ret.Get(0) != nil {
		result = ret.Get(0).([]models.Product)
	}
	return result, ret.Get(1).(int), ret.Get(2).(int), ret.Error(3)
}

func (_m *Repo) FetchImageUrlById(ctx context.Context, id uuid.UUID) ([]models.Images, error) {
	ret := _m.Called(ctx, id)
	if fn, ok := ret.Get(0).(func(context.Context, uuid.UUID) ([]models.Images, error)); ok {
		return fn(ctx, id)
	}
	var result []models.Images
	if ret.Get(0) != nil {
		result = ret.Get(0).([]models.Images)
	}
	return result, ret.Error(1)
}

func (_m *Repo) FetchImagesByProductIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]models.Images, error) {
	ret := _m.Called(ctx, ids)
	if fn, ok := ret.Get(0).(func(context.Context, []uuid.UUID) (map[uuid.UUID][]models.Images, error)); ok {
		return fn(ctx, ids)
	}
	var result map[uuid.UUID][]models.Images
	if ret.Get(0) != nil {
		result = ret.Get(0).(map[uuid.UUID][]models.Images)
	}
	return result, ret.Error(1)
}

func (_m *Repo) FetchAllProducts(ctx context.Context) ([]*models.Product, error) {
	ret := _m.Called(ctx)
	if fn, ok := ret.Get(0).(func(context.Context) ([]*models.Product, error)); ok {
		return fn(ctx)
	}
	var result []*models.Product
	if ret.Get(0) != nil {
		result = ret.Get(0).([]*models.Product)
	}
	return result, ret.Error(1)
}

func (_m *Repo) FetchProductById(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	ret := _m.Called(ctx, id)
	if fn, ok := ret.Get(0).(func(context.Context, uuid.UUID) (*models.Product, error)); ok {
		return fn(ctx, id)
	}
	var result *models.Product
	if ret.Get(0) != nil {
		result = ret.Get(0).(*models.Product)
	}
	return result, ret.Error(1)
}

func (_m *Repo) UpdateProductWithImages(ctx context.Context, id uuid.UUID, product *models.Product, images []models.Images) (models.Product, []models.Images, error) {
	ret := _m.Called(ctx, id, product, images)
	if fn, ok := ret.Get(0).(func(context.Context, uuid.UUID, *models.Product, []models.Images) (models.Product, []models.Images, error)); ok {
		return fn(ctx, id, product, images)
	}
	var updated models.Product
	if ret.Get(0) != nil {
		updated = ret.Get(0).(models.Product)
	}
	var persisted []models.Images
	if ret.Get(1) != nil {
		persisted = ret.Get(1).([]models.Images)
	}
	return updated, persisted, ret.Error(2)
}

func (_m *Repo) DeleteProductAndQueueImageCleanup(ctx context.Context, id uuid.UUID, additionalIDs []string) error {
	return _m.Called(ctx, id, additionalIDs).Error(0)
}

func (_m *Repo) ScheduleCloudinaryCleanup(ctx context.Context, ids []string) error {
	return _m.Called(ctx, ids).Error(0)
}

func (_m *Repo) StageCloudinaryCleanup(ctx context.Context, ids []string) error {
	return _m.Called(ctx, ids).Error(0)
}

func (_m *Repo) ClaimCloudinaryCleanup(ctx context.Context, limit int, lease time.Duration) ([]products.CloudinaryCleanupJob, error) {
	ret := _m.Called(ctx, limit, lease)
	var result []products.CloudinaryCleanupJob
	if ret.Get(0) != nil {
		result = ret.Get(0).([]products.CloudinaryCleanupJob)
	}
	return result, ret.Error(1)
}

func (_m *Repo) CompleteCloudinaryCleanup(ctx context.Context, publicID string) error {
	return _m.Called(ctx, publicID).Error(0)
}

func (_m *Repo) RetryCloudinaryCleanup(ctx context.Context, publicID, failure string, retryAt time.Time) error {
	return _m.Called(ctx, publicID, failure, retryAt).Error(0)
}

func (_m *Repo) FetchReviews(ctx context.Context) ([]models.Reviews, error) {
	ret := _m.Called(ctx)
	var result []models.Reviews
	if ret.Get(0) != nil {
		result = ret.Get(0).([]models.Reviews)
	}
	return result, ret.Error(1)
}

func (_m *Repo) UpdateProduct(ctx context.Context, id uuid.UUID, product *models.Product) (models.Product, error) {
	ret := _m.Called(ctx, id, product)
	var result models.Product
	if ret.Get(0) != nil {
		result = ret.Get(0).(models.Product)
	}
	return result, ret.Error(1)
}

func (_m *Repo) InsertReviewAndUpdateProduct(ctx context.Context, review *models.Reviews) error {
	return _m.Called(ctx, review).Error(0)
}

func (_m *Repo) DeleteReviewAndUpdateProduct(ctx context.Context, productID, reviewID, userID uuid.UUID, isAdmin bool) error {
	return _m.Called(ctx, productID, reviewID, userID, isAdmin).Error(0)
}

func (_m *Repo) FetchReviewById(ctx context.Context, productID uuid.UUID) ([]models.Reviews, error) {
	ret := _m.Called(ctx, productID)
	var result []models.Reviews
	if ret.Get(0) != nil {
		result = ret.Get(0).([]models.Reviews)
	}
	return result, ret.Error(1)
}

func NewRepo(t interface {
	mock.TestingT
	Cleanup(func())
}) *Repo {
	repo := &Repo{}
	repo.Mock.Test(t)
	t.Cleanup(func() { repo.AssertExpectations(t) })
	return repo
}
