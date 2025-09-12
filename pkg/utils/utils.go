package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/jofosuware/go/shopit/internal/auth/repository"
	"github.com/nfnt/resize"
	"golang.org/x/crypto/bcrypt"
)

// contextKey is a custom type for context keys used in request contexts.
type contextKey string

// UserContextKey is the key used to store/retrieve the user from context.
const UserContextKey contextKey = "user"

var Repo *repository.AuthRepository

// WriteJSON writes arbitrary data out as JSON
func WriteJSON(w http.ResponseWriter, status int, data interface{}, headers ...http.Header) error {
	out, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	if len(headers) > 0 {
		for k, v := range headers[0] {
			w.Header()[k] = v
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(out)

	return nil
}

// ReadJSON reads json from request body into data. We only accept a single json value in the body
func ReadJSON(w http.ResponseWriter, r *http.Request, data interface{}) error {
	maxBytes := 1048576 // max one megabyte in request body
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	err := dec.Decode(data)
	if err != nil {
		return err
	}

	// we only allow one entry in the json file
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only have a single JSON value")
	}

	return nil
}

// BadRequest sends a JSON response with status http.StatusBadRequest, describing the error
func BadRequest(w http.ResponseWriter, r *http.Request, err error) error {
	var payload struct {
		Success   bool   `json:"success"`
		Message string `json:"message"`
	}

	payload.Success = true
	payload.Message = err.Error()

	out, err := json.MarshalIndent(payload, "", "\t")
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	w.Write(out)
	return nil
}

func InvalidCredentials(w http.ResponseWriter) error {
	var payload struct {
		Success   bool   `json:"success"`
		Message string `json:"message"`
	}

	payload.Success = true
	payload.Message = "invalid authentication credentials"

	err := WriteJSON(w, http.StatusUnauthorized, payload)
	if err != nil {
		return err
	}
	return nil
}

func TooManyRequests(w http.ResponseWriter) error {
	var payload struct {
		Success   bool   `json:"success"`
		Message string `json:"message"`
	}

	payload.Success = true
	payload.Message = "Too many requests"

	err := WriteJSON(w, http.StatusTooManyRequests, payload)
	if err != nil {
		return err
	}
	return nil
}

func PasswordMatches(hash, password string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

func FailedValidation(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	var payload struct {
		Success   bool              `json:"success"`
		Message string            `json:"message"`
		Errors  map[string]string `json:"errors"`
	}

	payload.Success = true
	payload.Message = "failed validation"
	payload.Errors = errors
	WriteJSON(w, http.StatusUnprocessableEntity, payload)
}

func ProcessImage(file multipart.File, width, height uint) ([]byte, error) {
	//Decode the file into an image.Image type
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	//Resize image
	resizedImg := resize.Resize(width, height, img, resize.Lanczos3)

	buf := new(bytes.Buffer)
	err = png.Encode(buf, resizedImg)
	if err != nil {
		return nil, err
	}
	imgData := buf.Bytes()

	return imgData, nil
}

// IsAuthenticated checks whether a user is authenticated
func IsAuthenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorizationHeader := r.Header.Get("Authorization")
		if authorizationHeader == "" {
			_ = InvalidCredentials(w)
			fmt.Println("no authorization header received")
			return
		}

		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			_ = InvalidCredentials(w)
			fmt.Println("no authorization header received")
			return
		}

		token := headerParts[1]

		if len(token) != 26 {
			_ = InvalidCredentials(w)
			fmt.Println("error verifying token length")
			return
		}

		user, err := Repo.FetchUserByToken(token)
		if err != nil {
			_ = InvalidCredentials(w)
			fmt.Println("error retrieving token from database: ", err)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// CreateMultipartForm takes url.Values map and returns the multipart form data and the content type
func CreateMultipartForm(fields url.Values) (*bytes.Buffer, string, error) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Add form fields to the multipart writer
	for key, values := range fields {
		for _, value := range values {
			if err := addFormField(w, key, value); err != nil {
				return nil, "", err
			}
		}
	}

	// Close the multipart writer to finalize the form
	if err := w.Close(); err != nil {
		return nil, "", err
	}

	return &b, w.FormDataContentType(), nil
}

// addFormField adds a single form field to the multipart writer
func addFormField(w *multipart.Writer, key, value string) error {
	fw, err := w.CreateFormField(key)
	if err != nil {
		return fmt.Errorf("error creating form field: %v", err)
	}
	if _, err := io.WriteString(fw, value); err != nil {
		return fmt.Errorf("error writing to form field: %v", err)
	}
	return nil
}

// ExtractImages extracts the images from the multipart form data
func ExtractImages(img []*multipart.FileHeader) ([]*multipart.File, error) {
	var files []*multipart.File

	for _, img := range img {
		file, err := img.Open()
		if err != nil {
			return nil, err
		}

		files = append(files, &file)

		file.Close()
	}

	return files, nil
}
