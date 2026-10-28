package cleanup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudinary/cloudinary-go/api/uploader"
	"github.com/jofosuware/go/shopit/internal/products"
	productMocks "github.com/jofosuware/go/shopit/internal/products/mocks"
	cloudMocks "github.com/jofosuware/go/shopit/pkg/cloudinary/mocks"
	loggerMocks "github.com/jofosuware/go/shopit/pkg/logger/mock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWorkerCompletesSuccessfulCleanup(t *testing.T) {
	repo := productMocks.NewRepo(t)
	cloudUploader := cloudMocks.NewCloudUploader(t)
	logger := loggerMocks.NewLogger(t)
	ctx := context.Background()

	repo.On("ClaimCloudinaryCleanup", ctx, batchSize, jobLease).
		Return([]products.CloudinaryCleanupJob{{PublicID: "old-image", Attempts: 1}}, nil)
	cloudUploader.On("Destroy", "old-image").Return(nil, nil)
	repo.On("CompleteCloudinaryCleanup", ctx, "old-image").Return(nil)

	NewWorker(repo, cloudUploader, logger, time.Second).ProcessBatch(ctx)
}

func TestWorkerSchedulesRetryAfterCleanupFailure(t *testing.T) {
	repo := productMocks.NewRepo(t)
	cloudUploader := cloudMocks.NewCloudUploader(t)
	logger := loggerMocks.NewLogger(t)
	ctx := context.Background()
	cleanupErr := errors.New("Cloudinary unavailable")

	repo.On("ClaimCloudinaryCleanup", ctx, batchSize, jobLease).
		Return([]products.CloudinaryCleanupJob{{PublicID: "old-image", Attempts: 2}}, nil)
	cloudUploader.On("Destroy", "old-image").Return((*uploader.DestroyResult)(nil), cleanupErr)
	repo.On("RetryCloudinaryCleanup", ctx, "old-image", cleanupErr.Error(), mock.MatchedBy(func(retryAt time.Time) bool {
		return retryAt.After(time.Now()) && retryAt.Before(time.Now().Add(3*time.Minute))
	})).Return(nil)
	logger.On("Errorf", "Cloudinary cleanup for %q failed; retry scheduled at %s: %v", "old-image", mock.AnythingOfType("string"), cleanupErr).Return()

	NewWorker(repo, cloudUploader, logger, time.Second).ProcessBatch(ctx)
}

func TestRetryDelayIsBounded(t *testing.T) {
	require.Equal(t, time.Minute, retryDelay(1))
	require.Equal(t, 2*time.Minute, retryDelay(2))
	require.Equal(t, maxRetryGap, retryDelay(20))
}
