package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/cavke/go-distributed-object-storage/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type MockStorage struct {
	objects map[string]*storage.Object
	err     error
}

func (ms *MockStorage) Init(ctx context.Context) error {
	return nil
}

func (ms *MockStorage) Get(ctx context.Context, id string) (*storage.Object, error) {
	if ms.err != nil {
		return nil, ms.err
	}
	return ms.objects[id], nil
}

func (ms *MockStorage) Put(ctx context.Context, object *storage.Object) error {
	if ms.err != nil {
		return ms.err
	}
	ms.objects[object.ID] = object
	return nil
}

func TestGetObject(t *testing.T) {
	tests := []struct {
		name           string
		objectID       string
		mockStorage    *MockStorage
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "invalid object ID",
			objectID:       "invalid@ID",
			mockStorage:    &MockStorage{},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid objectID. Must be alfa-numeric string between 1 and 32 characters.",
		},
		{
			name:     "internal server error",
			objectID: "validID",
			mockStorage: &MockStorage{
				err: errors.New("test error"),
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Error retrieving object: validID",
		},
		{
			name:     "object not found",
			objectID: "missingID",
			mockStorage: &MockStorage{
				objects: make(map[string]*storage.Object),
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Object doesn't exist: missingID",
		},
		{
			name:     "success",
			objectID: "validID",
			mockStorage: &MockStorage{
				objects: map[string]*storage.Object{
					"validID": {
						Content:     []byte("test content"),
						ContentType: "text/plain",
					},
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "test content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new Echo instance
			e := echo.New()

			// Register the route to allow Echo to understand the :id parameter
			e.GET("/object/:id", func(c echo.Context) error {
				return getObject(tt.mockStorage, c)
			})

			// Set up the request and response recorder
			req := httptest.NewRequest(http.MethodGet, "/object/"+tt.objectID, nil)
			rec := httptest.NewRecorder()

			// Start the Echo router
			e.ServeHTTP(rec, req)

			// Assert the outcomes
			assert.Equal(t, tt.expectedStatus, rec.Code)

			if rec.Code != 200 {
				// Parse the error message
				var resp Response
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				if err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				assert.Equal(t, tt.expectedBody, resp.Message)
			}
		})
	}
}

func TestPutObject(t *testing.T) {
	tests := []struct {
		name           string
		objectID       string
		body           io.Reader
		contentType    string
		mockStorage    *MockStorage
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "invalid object ID",
			objectID:       "invalid@ID",
			body:           strings.NewReader("test content"),
			contentType:    "text/plain",
			mockStorage:    &MockStorage{},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid objectID. Must be alfa-numeric string between 1 and 32 characters.",
		},
		{
			name:           "error reading request body",
			objectID:       "validID",
			body:           &errorReader{}, // custom errorReader type to simulate reading error
			contentType:    "text/plain",
			mockStorage:    &MockStorage{},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Cannot read request body",
		},
		{
			name:           "error storing object",
			objectID:       "validID",
			body:           strings.NewReader("test content"),
			contentType:    "text/plain",
			mockStorage:    &MockStorage{err: errors.New("test error")},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Cannot store object: validID",
		},
		{
			name:        "success",
			objectID:    "validID",
			body:        strings.NewReader("test content"),
			contentType: "text/plain",
			mockStorage: &MockStorage{
				objects: make(map[string]*storage.Object),
				err:     nil,
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "Object was successfully stored with ID: validID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new Echo instance
			e := echo.New()

			// Register the route to allow Echo to understand the :id parameter
			e.PUT("/object/:id", func(c echo.Context) error {
				return putObject(tt.mockStorage, c)
			})

			// Setup the request and response recorder
			req := httptest.NewRequest(http.MethodPut, "/object/"+tt.objectID, tt.body)
			req.Header.Set(echo.HeaderContentType, tt.contentType)
			rec := httptest.NewRecorder()

			// Start the Echo router
			e.ServeHTTP(rec, req)

			// Parse the response message
			var resp Response
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			if err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			// Assert the outcomes
			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Equal(t, tt.expectedBody, resp.Message)
		})
	}
}

// Custom error reader to simulate error when reading request body
type errorReader struct{}

func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}
