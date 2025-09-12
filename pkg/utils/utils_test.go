package utils

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestWriteJSON(t *testing.T) {
	// Create a mock HTTP response writer
	w := httptest.NewRecorder()

	// Call the WriteJSON function
	err := WriteJSON(w, http.StatusOK, map[string]interface{}{"message": "Hello, World!"})

	assert.NoError(t, err)
	assert.Equal(t, w.Code, http.StatusOK)
}

func TestReadJSON(t *testing.T) {
	// Create a mock HTTP request with JSON body
	r, err := http.NewRequest(http.MethodPost, "/api", bytes.NewBuffer([]byte(`{"name":"John Doe"}`)))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	// Create a mock HTTP response writer
	w := httptest.NewRecorder()

	// Call the ReadJSON function
	var data struct {
		Name string `json:"name"`
	}
	err = ReadJSON(w, r, &data)
	assert.NoError(t, err)

	// Check the parsed JSON data
	expectedName := "John Doe"
	assert.Equal(t, data.Name, expectedName)
}

func TestBadRequest(t *testing.T) {
	// Create a mock HTTP response writer
	w := httptest.NewRecorder()

	// Create a mock HTTP request
	r := httptest.NewRequest(http.MethodGet, "/api", nil)

	// Call the BadRequest function
	err := BadRequest(w, r, errors.New("Bad request"))

	// Check if there was an error
	assert.NoError(t, err)

	// Check the response status code
	assert.Equal(t, w.Code, http.StatusBadRequest)
}

func TestInvalidCredentials(t *testing.T) {
	// Create a mock HTTP response writer
	w := httptest.NewRecorder()

	// Call the InvalidCredentials function
	err := InvalidCredentials(w)

	// Check if there was an error
	assert.NoError(t, err)

	// Check the response status code
	assert.Equal(t, w.Code, http.StatusUnauthorized)
}

func TestPasswordMatches(t *testing.T) {
	// Create a mock password hash
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	assert.NoError(t, err)

	// Call the PasswordMatches function
	matches, err := PasswordMatches(string(hash), "password")

	// Check if there was an error
	assert.NoError(t, err)

	// Check the return value
	assert.True(t, matches)
}

func TestFailedValidation(t *testing.T) {
	// Create a mock HTTP response writer
	w := httptest.NewRecorder()

	// Create a mock HTTP request
	r := httptest.NewRequest(http.MethodGet, "/api", nil)

	// Call the FailedValidation function
	FailedValidation(w, r, map[string]string{"name": "Name is required"})

	// Check the response status code
	assert.Equal(t, w.Code, http.StatusUnprocessableEntity)
}

func TestProcessImage(t *testing.T) {
	// Create a mock image of 100x100
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	cyan := color.RGBA{100, 200, 200, 0xff}

	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.Set(x, y, cyan)
		}
	}

	// Encode the image to PNG
	buf := new(bytes.Buffer)
	err := png.Encode(buf, img)
	assert.NoError(t, err)

	imgBytes := buf.Bytes()

	// Create a new multipart writer
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Create a new form-file
	partHeader := make(textproto.MIMEHeader)
	partHeader.Add("Content-Type", "image/png")
	partHeader.Add("Content-Disposition", `form-data; name="image"; filename="img.png"`)
	part, err := writer.CreatePart(partHeader)
	assert.NoError(t, err)

	// Write image bytes to the part
	_, err = part.Write(imgBytes)
	assert.NoError(t, err)

	// Close the writer
	err = writer.Close()
	assert.NoError(t, err)

	// Create a new multipart file
	mr := multipart.NewReader(body, writer.Boundary())
	form, err := mr.ReadForm(10 << 20)
	assert.NoError(t, err)
	file := form.File["image"][0]

	// Open the file
	f, err := file.Open()
	assert.NoError(t, err)

	// Process the image
	_, err = ProcessImage(f, 200, 200)
	assert.NoError(t, err)
}

//func TestIsAuthenticated(t *testing.T) {
//	// Create mock repository
//	repo := mockRepo.NewRepo(t)
//
//	// Create a test handler that will be wrapped by the middleware
//	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		w.WriteHeader(http.StatusOK)
//		w.Write([]byte("handler called"))
//	})
//
//	// Apply the middleware to the test handler
//	middleware := IsAuthenticated(testHandler)
//
//	t.Run("no cookie present", func(t *testing.T) {
//		// Create a request with no cookie
//		req := httptest.NewRequest("GET", "/", nil)
//		rr := httptest.NewRecorder()
//
//		// Call the middleware
//		middleware.ServeHTTP(rr, req)
//
//		// Check response
//		assert.Equal(t, http.StatusUnauthorized, rr.Code)
//		assert.NotContains(t, rr.Body.String(), "handler called")
//	})
//
//	t.Run("wrong cookie name", func(t *testing.T) {
//		// Create a request with wrong cookie name
//		req := httptest.NewRequest("GET", "/", nil)
//		req.AddCookie(&http.Cookie{
//			Name:  "wrongname",
//			Value: "MQUYLLXB2PHU5PE6PG3HGG2AXI",
//		})
//		rr := httptest.NewRecorder()
//
//		// Call the middleware
//		middleware.ServeHTTP(rr, req)
//
//		// Check response
//		assert.Equal(t, http.StatusUnauthorized, rr.Code)
//		assert.NotContains(t, rr.Body.String(), "handler called")
//	})
//
//	t.Run("token with invalid length", func(t *testing.T) {
//		// Create a request with token of invalid length
//		req := httptest.NewRequest("GET", "/", nil)
//		req.AddCookie(&http.Cookie{
//			Name:  "token",
//			Value: "tooshort",
//		})
//		rr := httptest.NewRecorder()
//
//		// Call the middleware
//		middleware.ServeHTTP(rr, req)
//
//		// Check response
//		assert.Equal(t, http.StatusUnauthorized, rr.Code)
//		assert.NotContains(t, rr.Body.String(), "handler called")
//	})
//
//	t.Run("token not found in database", func(t *testing.T) {
//		// Setup mock to return error
//		validToken := "MQUYLLXB2PHU5PE6PG3HGG2AXI"
//		repo.On("FetchUserByToken", validToken).Return(nil, errors.New("token not found"))
//
//		// Create a request with valid token format but not in DB
//		req := httptest.NewRequest("GET", "/", nil)
//		req.AddCookie(&http.Cookie{
//			Name:  "token",
//			Value: validToken,
//		})
//		rr := httptest.NewRecorder()
//
//		// Call the middleware
//		middleware.ServeHTTP(rr, req)
//
//		// Check response
//		assert.Equal(t, http.StatusUnauthorized, rr.Code)
//		assert.NotContains(t, rr.Body.String(), "handler called")
//	})
//
//	t.Run("valid authentication", func(t *testing.T) {
//		// Setup mock to return success
//		validToken := "MQUYLLXB2PHU5PE6PG3HGG2AXI"
//		repo.On("FetchUserByToken", validToken).Return(struct{}{}, nil)
//
//		// Create a request with valid token
//		req := httptest.NewRequest("GET", "/", nil)
//		req.AddCookie(&http.Cookie{
//			Name:  "token",
//			Value: validToken,
//		})
//		rr := httptest.NewRecorder()
//
//		// Call the middleware
//		middleware.ServeHTTP(rr, req)
//
//		// Check that the handler was called
//		assert.Equal(t, http.StatusOK, rr.Code)
//		assert.Contains(t, rr.Body.String(), "handler called")
//	})
//}
