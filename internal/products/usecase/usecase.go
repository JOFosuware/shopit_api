package usecase

import (
	"fmt"
	"mime/multipart"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/products"
	"github.com/jofosuware/go/shopit/pkg/cloudinary"
)

// ProductsUC is the struct type for Product UseCase
type ProductsUC struct {
	cld  cloudinary.CloudUploader
	repo products.Repo
}

// NewProductsUC is the constructor for ProductsUC
func NewProductsUC(cld cloudinary.CloudUploader, repo products.Repo) *ProductsUC {
	return &ProductsUC{
		repo: repo,
		cld:  cld,
	}
}

// CreateProduct creates a new product and uploads its images to cloudinary
func (p *ProductsUC) CreateProduct(prod models.Product, img []*multipart.FileHeader) (*models.ProdResponse, error) {
	prod, err := p.repo.InsertProduct(&prod)
	if err != nil {
		return nil, fmt.Errorf("error saving product: %v", err)
	}

	// Upload images to cloudinary and save their urls
	for _, imgHeader := range img {
		image, err := imgHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("error opening image: %v", err)
		}

		res, err := p.cld.UploadToCloud("products", image)
		if err != nil {
			return nil, fmt.Errorf("error uploading image: %v", err)
		}

		var img models.Images

		img.PublicId = res.PublicID
		img.Url = res.URL
		img.ProductId = prod.ProductId

		// saving image url
		img, err = p.repo.InsertImageUrl(&img)
		if err != nil {
			return nil, fmt.Errorf("error saving image url: %v", err)
		}

		prod.Images = append(prod.Images, img)
		image.Close()
	}

	pr := models.ProdResponse{
		Success: true,
		Product: prod,
	}

	return &pr, nil
}

// GetProducts retrieves products based on a keyword and page number
func (p *ProductsUC) GetProducts(keyword string, page int) (*models.GetProd, error) {
	prods, count, err := p.repo.FetchProductByName(keyword, page)
	if err != nil {
		return nil, fmt.Errorf("error fetching products: %v", err)
	}

	for i, prod := range prods {
		img, err := p.repo.FetchImageUrlById(prod.ProductId)
		if err != nil {
			return nil, fmt.Errorf("error fetching image url: %v", err)
		}
		prods[i].Images = img
	}

	jr := models.GetProd{
		Success:               true,
		ProductCount:          count,
		ResPerPage:            4,
		FilteredProductsCount: len(prods),
		Products:              prods,
	}

	return &jr, nil
}

// GetAdminProducts retrieves all products for admin use
func (p *ProductsUC) GetAdminProducts() ([]*models.Product, error) {
	prods, err := p.repo.FetchAllProducts()
	if err != nil {
		return nil, fmt.Errorf("error fetching products: %v", err)
	}

	return prods, nil
}

// GetSingleProduct retrieves a single product by its ID
func (p *ProductsUC) GetSingleProduct(id uuid.UUID) (*models.Product, error) {
	prod, err := p.repo.FetchProductById(id)
	if err != nil {
		return nil, fmt.Errorf("error fetching product: %v", err)
	}

	img, err := p.repo.FetchImageUrlById(prod.ProductId)
	if err != nil {
		return nil, fmt.Errorf("error fetching image url: %v", err)
	}

	review, err := p.repo.FetchReviewById(prod.ProductId)
	if err != nil {
		return nil, fmt.Errorf("error fetching review: %v", err)
	}

	prod.Images = img
	prod.Reviews = review

	return prod, nil
}

// UpdateProduct updates a product's details and images by its id
func (p *ProductsUC) UpdateProduct(id uuid.UUID, prod models.Product, img []*multipart.File) (*models.ProdResponse, error) {
	// Fetch existing images
	images, err := p.repo.FetchImageUrlById(id)
	if err != nil {
		return nil, fmt.Errorf("error fetching image url: %v", err)
	}

	if len(img) > 0 {
		// Delete existing images from cloudinary
		for _, img := range images {
			_, err := p.cld.Destroy(img.PublicId)
			if err != nil {
				return nil, fmt.Errorf("error deleting image from cloudinary: %v", err)
			}
		}

		// DeleteImageUrlById deletes all existing images of a particular product from database
		err = p.repo.DeleteImageUrlById(id)
		if err != nil {
			return nil, fmt.Errorf("error deleting images from database: %v", err)
		}

		// Upload new images to cloudinary and save their urls
		images = []models.Images{}
		for _, img := range img {
			res, err := p.cld.UploadToCloud("products", img)
			if err != nil {
				return nil, fmt.Errorf("error uploading image to cloudinary: %v", err)
			}

			var img models.Images
			img.PublicId = res.PublicID
			img.Url = res.URL
			img.ProductId = id

			// Save image url to database
			img, err = p.repo.InsertImageUrl(&img)
			if err != nil {
				return nil, fmt.Errorf("error saving image url: %v", err)
			}

			images = append(images, img)
		}
	}

	prod, err = p.repo.UpdateProduct(id, &prod)
	if err != nil {
		return nil, fmt.Errorf("error updating product: %v", err)
	}

	prod.Images = images

	res := models.ProdResponse{
		Success: true,
		Product: prod,
	}

	return &res, nil
}

// DeleteProduct deletes product from the product's table by its id
func (p *ProductsUC) DeleteProduct(id uuid.UUID) error {
	// Fetch existing images
	img, err := p.repo.FetchImageUrlById(id)
	if err != nil {
		return fmt.Errorf("error fetching image url: %v", err)
	}

	// Delete existing images from cloudinary
	for _, img := range img {
		_, err := p.cld.Destroy(img.PublicId)
		if err != nil {
			return fmt.Errorf("error deleting image from cloudinary: %v", err)
		}
	}

	// Delete the product
	err = p.repo.DeleteProductById(id)
	if err != nil {
		return fmt.Errorf("error deleting product: %v", err)
	}

	return nil
}

// CreateProductReview process product's review and save it into the database
func (p *ProductsUC) CreateProductReview(review models.Reviews) error {
	product, err := p.repo.FetchProductById(review.ProductId)
	if err != nil {
		return fmt.Errorf("error fetching product: %v", err)
	}

	reviews, err := p.repo.FetchReviewById(review.ProductId)
	if err != nil {
		return fmt.Errorf("error fetching reviews: %v", err)
	}

	reviews = append(reviews, review)
	product.NumOfReviews = len(reviews)

	err = p.repo.InsertReview(&review)
	if err != nil {
		return fmt.Errorf("error inserting reviews: %v", err)
	}

	var totalRating = 0
	for _, rv := range reviews {
		totalRating += rv.Rating
	}

	product.Ratings = totalRating / len(reviews)
	_, err = p.repo.UpdateProduct(review.ProductId, product)

	if err != nil {
		return fmt.Errorf("error updating product: %v", err)
	}

	return nil
}

// GetProductReviews fetches all reviews for a particular product
func (p *ProductsUC) GetProductReviews(id uuid.UUID) ([]models.Reviews, error) {
	reviews, err := p.repo.FetchReviewById(id)
	if err != nil {
		return nil, fmt.Errorf("error fetching reviews: %v", err)
	}

	return reviews, nil
}

// DeleteProductReview deletes a particular review for a product by its id
func (p *ProductsUC) DeleteProductReview(productId uuid.UUID, reviewId uuid.UUID) error {
	err := p.repo.DeleteReviewById(reviewId)
	if err != nil {
		return fmt.Errorf("error deleting review: %v", err)
	}

	product, err := p.repo.FetchProductById(productId)
	if err != nil {
		return fmt.Errorf("error fetching product: %v", err)
	}

	reviews, err := p.repo.FetchReviewById(productId)
	if err != nil {
		return fmt.Errorf("error fetching reviews: %v", err)
	}

	var totalRating = 0
	for _, rv := range reviews {
		totalRating += rv.Rating
	}

	product.Ratings = totalRating / len(reviews)
	product.NumOfReviews = len(reviews)

	_, err = p.repo.UpdateProduct(productId, product)
	if err != nil {
		return fmt.Errorf("error updating product: %v", err)
	}

	return nil
}
