package delivery

import (
	"net/http"

	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/pkg/utils"
)

func (h *AuthHandlers) writeError(w http.ResponseWriter, status int, message string) {
	_ = h.writeJSON(w, status, models.Response{
		Success: false,
		Message: message,
	})
}

func (h *AuthHandlers) writeBadRequest(w http.ResponseWriter, operation string, err error) {
	h.logger.Errorf("%s: %v", operation, err)
	h.writeError(w, http.StatusBadRequest, "invalid request")
}

func (h *AuthHandlers) writeValidationError(w http.ResponseWriter, r *http.Request, operation string, validationErrors map[string]string) {
	h.logger.Errorf("%s: %v", operation, validationErrors)
	if err := utils.FailedValidation(w, r, validationErrors); err != nil {
		h.logger.Errorf("writing validation response: %v", err)
	}
}

func (h *AuthHandlers) writeJSON(w http.ResponseWriter, status int, payload any) error {
	if err := utils.WriteJSON(w, status, payload); err != nil {
		h.logger.Errorf("writing HTTP response: %v", err)
		return err
	}
	return nil
}

func (h *AuthHandlers) writeInternalError(w http.ResponseWriter, operation string, err error) {
	h.logger.Errorf("%s: %v", operation, err)
	h.writeError(w, http.StatusInternalServerError, "internal server error")
}
