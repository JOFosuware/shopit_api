package cleanup

import (
	"context"
	"time"

	"github.com/jofosuware/go/shopit/internal/products"
	"github.com/jofosuware/go/shopit/pkg/cloudinary"
	"github.com/jofosuware/go/shopit/pkg/logger"
)

const (
	batchSize   = 25
	jobLease    = time.Minute
	maxRetryGap = 6 * time.Hour
)

type Worker struct {
	repo     products.Repo
	uploader cloudinary.CloudUploader
	logger   logger.Logger
	interval time.Duration
}

func NewWorker(repo products.Repo, uploader cloudinary.CloudUploader, logger logger.Logger, interval time.Duration) *Worker {
	return &Worker{repo: repo, uploader: uploader, logger: logger, interval: interval}
}

func (w *Worker) Run(ctx context.Context) {
	if w.interval <= 0 {
		w.logger.Errorf("Cloudinary cleanup worker interval must be positive")
		return
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.ProcessBatch(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.ProcessBatch(ctx)
		}
	}
}

func (w *Worker) ProcessBatch(ctx context.Context) {
	jobs, err := w.repo.ClaimCloudinaryCleanup(ctx, batchSize, jobLease)
	if err != nil {
		if ctx.Err() == nil {
			w.logger.Errorf("claiming Cloudinary cleanup jobs: %v", err)
		}
		return
	}

	for _, job := range jobs {
		if err := ctx.Err(); err != nil {
			return
		}
		if _, err := w.uploader.Destroy(job.PublicID); err != nil {
			retryAt := time.Now().Add(retryDelay(job.Attempts))
			if retryErr := w.repo.RetryCloudinaryCleanup(ctx, job.PublicID, err.Error(), retryAt); retryErr != nil {
				w.logger.Errorf("retrying Cloudinary cleanup job %q after %v failed: %v", job.PublicID, err, retryErr)
				continue
			}
			w.logger.Errorf("Cloudinary cleanup for %q failed; retry scheduled at %s: %v", job.PublicID, retryAt.Format(time.RFC3339), err)
			continue
		}

		if err := w.repo.CompleteCloudinaryCleanup(ctx, job.PublicID); err != nil {
			w.logger.Errorf("completing Cloudinary cleanup job %q: %v", job.PublicID, err)
		}
	}
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Minute << min(attempt-1, 9)
	if delay > maxRetryGap {
		return maxRetryGap
	}
	return delay
}
