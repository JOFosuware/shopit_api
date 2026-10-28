package delivery

import (
	"errors"
	"net/http"

	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/internal/products"
	"github.com/jofosuware/go/shopit/pkg/utils"
)

func (h *ProdHandlers) writeError(w http.ResponseWriter, status int, message string) {
	_ = h.writeJSON(w, status, models.Response{
		Success: false,
		Message: message,
	})
}

func (h *ProdHandlers) writeBadRequest(w http.ResponseWriter, operation string, err error) {
	h.logger.Errorf("%s: %v", operation, err)
	h.writeError(w, http.StatusBadRequest, "invalid request")
}

func (h *ProdHandlers) writeValidationError(w http.ResponseWriter, r *http.Request, operation string, validationErrors map[string]string) {
	h.logger.Errorf("%s: %v", operation, validationErrors)
	if err := utils.FailedValidation(w, r, validationErrors); err != nil {
		h.logger.Errorf("writing validation response: %v", err)
	}
}

func (h *ProdHandlers) writeJSON(w http.ResponseWriter, status int, payload any) error {
	if err := utils.WriteJSON(w, status, payload); err != nil {
		h.logger.Errorf("writing HTTP response: %v", err)
		return err
	}
	return nil
}

func (h *ProdHandlers) writeProductError(w http.ResponseWriter, operation string, err error) {
	h.logger.Errorf("%s: %v", operation, err)
	switch {
	case errors.Is(err, products.ErrProductNotFound):
		h.writeError(w, http.StatusNotFound, "product not found")
	case errors.Is(err, products.ErrReviewNotFound):
		h.writeError(w, http.StatusNotFound, "product review not found")
	case errors.Is(err, products.ErrInvalidRating):
		h.writeError(w, http.StatusBadRequest, products.ErrInvalidRating.Error())
	case errors.Is(err, products.ErrInvalidPage):
		h.writeError(w, http.StatusBadRequest, products.ErrInvalidPage.Error())
	case errors.Is(err, products.ErrImageUploadFailed):
		h.writeError(w, http.StatusBadGateway, products.ErrImageUploadFailed.Error())
	default:
		h.writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
