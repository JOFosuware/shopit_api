package products

import "errors"

var (
	ErrProductNotFound   = errors.New("product not found")
	ErrReviewNotFound    = errors.New("product review not found")
	ErrInvalidRating     = errors.New("review rating must be between 1 and 5")
	ErrInvalidPage       = errors.New("page is outside the valid range")
	ErrImageUploadFailed = errors.New("failed to upload product image")
)

const ProductsPerPage = 12

func IsValidProductPage(page int) bool {
	return page > 0 && page-1 <= int(^uint(0)>>1)/ProductsPerPage
}
