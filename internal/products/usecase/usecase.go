package usecase

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"time"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/products"
	"github.com/jofosuware/go/shopit/pkg/cloudinary"
)

const cloudinaryUploadTimeout = 5 * time.Minute

// ProductsUC provides product-related use cases.
type ProductsUC struct {
	cld  cloudinary.CloudUploader
	repo products.Repo
}

// NewProductsUC returns a new ProductsUC.
func NewProductsUC(cld cloudinary.CloudUploader, repo products.Repo) *ProductsUC {
	return &ProductsUC{
		repo: repo,
		cld:  cld,
	}
}

// CreateProduct creates a new product and uploads its images to cloudinary.
func (p *ProductsUC) CreateProduct(ctx context.Context, prod models.Product, img []*multipart.FileHeader) (*models.ProdResponse, error) {
	prod, err := p.repo.InsertProduct(ctx, &prod)
	if err != nil {
		return nil, fmt.Errorf("saving product: %w", err)
	}

	imageRecords, publicIDs, err := p.uploadImages(ctx, img, prod.ProductId)
	if err != nil {
		return nil, errors.Join(err, p.repo.DeleteProductAndQueueImageCleanup(context.Background(), prod.ProductId, publicIDs))
	}
	for i, imageRecord := range imageRecords {
		if i > 0 {
			if err := p.repo.StageCloudinaryCleanup(ctx, publicIDs[i:]); err != nil {
				return nil, errors.Join(
					fmt.Errorf("renewing staged image cleanup before saving images: %w", err),
					p.repo.DeleteProductAndQueueImageCleanup(context.Background(), prod.ProductId, publicIDs),
				)
			}
		}
		imageRecord, err = p.repo.InsertImageUrl(ctx, &imageRecord)
		if err != nil {
			return nil, errors.Join(
				fmt.Errorf("saving image URL: %w", err),
				p.repo.DeleteProductAndQueueImageCleanup(context.Background(), prod.ProductId, publicIDs),
			)
		}
		prod.Images = append(prod.Images, imageRecord)
	}

	pr := models.ProdResponse{
		Success: true,
		Product: prod,
	}

	return &pr, nil
}

// GetProducts returns products filtered by keyword with pagination.
func (p *ProductsUC) GetProducts(ctx context.Context, keyword string, page int) (*models.GetProd, error) {
	if !products.IsValidProductPage(page) {
		return nil, products.ErrInvalidPage
	}
	prods, productCount, filteredCount, err := p.repo.FetchProductByName(ctx, keyword, page)
	if err != nil {
		return nil, fmt.Errorf("fetching products: %w", err)
	}

	productIDs := make([]uuid.UUID, len(prods))
	for i := range prods {
		productIDs[i] = prods[i].ProductId
	}
	imagesByProductID, err := p.repo.FetchImagesByProductIDs(ctx, productIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching image URLs: %w", err)
	}
	for i := range prods {
		prods[i].Images = imagesByProductID[prods[i].ProductId]
	}

	jr := models.GetProd{
		Success:               true,
		ProductCount:          productCount,
		ResPerPage:            products.ProductsPerPage,
		FilteredProductsCount: filteredCount,
		Products:              prods,
	}

	return &jr, nil
}

// GetAdminProducts returns all products for admin.
func (p *ProductsUC) GetAdminProducts(ctx context.Context) ([]*models.Product, error) {
	prods, err := p.repo.FetchAllProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching products: %w", err)
	}

	return prods, nil
}

// GetSingleProduct returns a product by ID, including images and reviews.
func (p *ProductsUC) GetSingleProduct(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	prod, err := p.repo.FetchProductById(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching product: %w", err)
	}

	img, err := p.repo.FetchImageUrlById(ctx, prod.ProductId)
	if err != nil {
		return nil, fmt.Errorf("fetching image URLs: %w", err)
	}

	review, err := p.repo.FetchReviewById(ctx, prod.ProductId)
	if err != nil {
		return nil, fmt.Errorf("fetching product reviews: %w", err)
	}

	prod.Images = img
	prod.Reviews = review

	return prod, nil
}

// UpdateProduct updates a product's details and images by ID.
func (p *ProductsUC) UpdateProduct(ctx context.Context, id uuid.UUID, prod models.Product, img []*multipart.FileHeader) (*models.ProdResponse, error) {
	var images []models.Images
	var err error
	if len(img) > 0 {
		var publicIDs []string
		images, publicIDs, err = p.uploadImages(ctx, img, id)
		if err != nil {
			return nil, errors.Join(err, p.repo.ScheduleCloudinaryCleanup(context.Background(), publicIDs))
		}

		prod, images, err = p.repo.UpdateProductWithImages(ctx, id, &prod, images)
		if err != nil {
			return nil, fmt.Errorf("updating product and replacing image records: %w", err)
		}
	} else {
		prod, err = p.repo.UpdateProduct(ctx, id, &prod)
		if err != nil {
			return nil, fmt.Errorf("updating product: %w", err)
		}
		images, err = p.repo.FetchImageUrlById(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("fetching image URLs: %w", err)
		}
	}

	prod.Images = images

	res := models.ProdResponse{
		Success: true,
		Product: prod,
	}

	return &res, nil
}

func (p *ProductsUC) uploadImages(ctx context.Context, headers []*multipart.FileHeader, productID uuid.UUID) ([]models.Images, []string, error) {
	images := make([]models.Images, 0, len(headers))
	stagedIDs := make([]string, 0, len(headers))
	cleanupIDs := make([]string, 0, len(headers))
	for _, header := range headers {
		if header == nil {
			err := errors.New("image file header is nil")
			return nil, cleanupIDs, errors.Join(err, p.repo.ScheduleCloudinaryCleanup(context.Background(), cleanupIDs))
		}
		image, err := header.Open()
		if err != nil {
			return nil, cleanupIDs, errors.Join(
				fmt.Errorf("opening image: %w", err),
				p.repo.ScheduleCloudinaryCleanup(context.Background(), cleanupIDs),
			)
		}
		publicID := uuid.NewString()
		expectedPublicID := "products/" + publicID
		stagedIDs = append(stagedIDs, expectedPublicID)
		if err := p.repo.StageCloudinaryCleanup(ctx, stagedIDs); err != nil {
			closeErr := image.Close()
			return nil, cleanupIDs, errors.Join(
				fmt.Errorf("staging uploaded image for cleanup: %w", err),
				closeErr,
				p.repo.ScheduleCloudinaryCleanup(context.Background(), cleanupIDs),
			)
		}
		uploadCtx, cancelUpload := context.WithTimeout(ctx, cloudinaryUploadTimeout)
		uploaded, uploadErr := p.cld.UploadToCloudWithPublicID(uploadCtx, "products", publicID, image)
		cancelUpload()
		closeErr := image.Close()
		if uploadErr != nil {
			return nil, cleanupIDs, errors.Join(
				fmt.Errorf("%w: uploading image to Cloudinary: %w", products.ErrImageUploadFailed, uploadErr),
				closeErr,
				p.repo.ScheduleCloudinaryCleanup(context.Background(), cleanupIDs),
			)
		}
		if uploaded == nil {
			return nil, cleanupIDs, errors.Join(
				fmt.Errorf("%w: Cloudinary returned an empty upload result", products.ErrImageUploadFailed),
				closeErr,
				p.repo.ScheduleCloudinaryCleanup(context.Background(), cleanupIDs),
			)
		}
		if uploaded.PublicID == "" || uploaded.URL == "" || uploaded.PublicID != expectedPublicID {
			cleanupIDs = append(cleanupIDs, expectedPublicID)
			if uploaded.PublicID != "" && uploaded.PublicID != expectedPublicID {
				cleanupIDs = append(cleanupIDs, uploaded.PublicID)
			}
			return nil, cleanupIDs, errors.Join(
				fmt.Errorf("%w: Cloudinary returned an incomplete or unexpected upload result", products.ErrImageUploadFailed),
				closeErr,
				p.repo.ScheduleCloudinaryCleanup(context.Background(), cleanupIDs),
			)
		}
		cleanupIDs = append(cleanupIDs, expectedPublicID)
		if closeErr != nil {
			return nil, cleanupIDs, errors.Join(
				fmt.Errorf("closing image: %w", closeErr),
				p.repo.ScheduleCloudinaryCleanup(context.Background(), cleanupIDs),
			)
		}
		images = append(images, models.Images{
			PublicId:  uploaded.PublicID,
			Url:       uploaded.URL,
			ProductId: productID,
		})
	}
	return images, cleanupIDs, nil
}

// DeleteProduct deletes a product and its images by ID.
func (p *ProductsUC) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	err := p.repo.DeleteProductAndQueueImageCleanup(ctx, id, nil)
	if err != nil {
		return fmt.Errorf("deleting product: %w", err)
	}
	return nil
}

// CreateProductReview creates a review and updates product aggregates atomically.
func (p *ProductsUC) CreateProductReview(ctx context.Context, review models.Reviews) error {
	if review.Rating < 1 || review.Rating > 5 {
		return products.ErrInvalidRating
	}
	if err := p.repo.InsertReviewAndUpdateProduct(ctx, &review); err != nil {
		return fmt.Errorf("creating product review: %w", err)
	}
	return nil
}

// GetProductReviews returns all reviews for a product.
func (p *ProductsUC) GetProductReviews(ctx context.Context, id uuid.UUID) ([]models.Reviews, error) {
	reviews, err := p.repo.FetchReviewById(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching product reviews: %w", err)
	}

	return reviews, nil
}

// DeleteProductReview deletes a review and updates the product's ratings.
func (p *ProductsUC) DeleteProductReview(ctx context.Context, actor *models.User, productID, reviewID uuid.UUID) error {
	if actor == nil {
		return errors.New("authenticated user is required to delete a review")
	}
	err := p.repo.DeleteReviewAndUpdateProduct(ctx, productID, reviewID, actor.ID, actor.Role == "admin")
	if err != nil {
		return fmt.Errorf("deleting product review: %w", err)
	}
	return nil
}
