package app

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"
)

func TestParseAddItemRequest(t *testing.T) {
	t.Parallel()

	type wants struct {
		req *AddItemRequest
		err bool
	}

	// STEP 6-1: define test cases
	cases := map[string]struct {
		args  map[string]string
		image []byte
		wants
	}{
		"ok: valid request": {
			args: map[string]string{
				"name":     "TestName",     // fill here
				"category": "TestCategory", // fill here
			},
			image: []byte("default.jpg"),
			wants: wants{
				req: &AddItemRequest{
					Name:     "TestName",     // fill here
					Category: "TestCategory", // fill here
					Image:    []byte("default.jpg"),
				},
				err: false,
			},
		},
		"ng: empty request": {
			args:  map[string]string{},
			image: nil,
			wants: wants{
				req: nil,
				err: true,
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// prepare request body
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			for k, v := range tt.args {
				err := writer.WriteField(k, v)
				if err != nil {
					t.Fatalf("failed to write field: %v", err)
				}
			}

			if len(tt.image) > 0 {
				file, err := writer.CreateFormFile("image", "default.jpg")
				if err != nil {
					t.Fatalf("Failed to create file: %v", err)
				}

				_, err = file.Write(tt.image)
				if err != nil {
					t.Fatalf("Failed to write image: %v", err)
				}
				writer.Close()
			}

			// prepare HTTP request
			req, err := http.NewRequest("POST", "http://localhost:9000/items", body)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", writer.FormDataContentType())

			// execute test target
			got, err := parseAddItemRequest(req)

			// confirm the result
			if err != nil {
				if !tt.err {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if diff := cmp.Diff(tt.wants.req, got); diff != "" {
				t.Errorf("unexpected request (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHelloHandler(t *testing.T) {
	t.Parallel()

	// Please comment out for STEP 6-2
	// predefine what we want
	type wants struct {
		code int               // desired HTTP status code
		body map[string]string // desired body
	}
	want := wants{
		code: http.StatusOK,
		body: map[string]string{"message": "Hello, world!"},
	}

	// set up test
	req := httptest.NewRequest("GET", "/hello", nil)
	res := httptest.NewRecorder()

	h := &Handlers{}
	h.Hello(res, req)

	// STEP 6-2: confirm the status code
	if res.Code != want.code {
		t.Fatalf("Status code failure: response code was: %d, want code was: %d", res.Code, want.code)
	}

	// STEP 6-2: confirm response body
	var actual map[string]string
	err := json.NewDecoder(res.Body).Decode(&actual)
	if err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}
	compare := cmp.Diff(want.body, actual)
	if compare != "" {
		t.Errorf("Unexpected response body: %s", compare)
	}
}

func TestAddItem(t *testing.T) {
	t.Parallel()

	type wants struct {
		code int
	}
	cases := map[string]struct {
		args     map[string]string
		injector func(m *MockItemRepository)
		wants
	}{
		"ok: correctly inserted": {
			args: map[string]string{
				"name":     "used iPhone 16e",
				"category": "phone",
				"image":    "default.png",
			},
			injector: func(m *MockItemRepository) {
				// STEP 6-3: define mock expectation
				// succeeded to insert
				m.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wants: wants{
				code: http.StatusOK,
			},
		},
		"ng: failed to insert": {
			args: map[string]string{
				"name":     "used iPhone 16e",
				"category": "phone",
				"image":    "default.png",
			},
			injector: func(m *MockItemRepository) {
				// STEP 6-3: define mock expectation
				// failed to insert
				m.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(errors.New("database error"))
			},
			wants: wants{
				code: http.StatusInternalServerError,
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			mockIR := NewMockItemRepository(ctrl)
			tt.injector(mockIR)
			h := &Handlers{itemRepo: mockIR}

			// values := url.Values{}
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			for k, v := range tt.args {
				// values.Set(k, v)
				_ = writer.WriteField(k, v)
			}

			file, err := writer.CreateFormFile("image", "test.jpg")
			if err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}
			_, err = file.Write([]byte("test image"))
			if err != nil {
				t.Fatalf("Failed to write image: %v", err)
			}
			writer.Close()

			req := httptest.NewRequest("POST", "/items", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			rr := httptest.NewRecorder()
			h.AddItem(rr, req)

			if tt.wants.code != rr.Code {
				t.Errorf("expected status code %d, got %d", tt.wants.code, rr.Code)
			}
			if tt.wants.code >= 400 {
				return
			}

			for _, v := range tt.args {
				if !strings.Contains(rr.Body.String(), v) {
					t.Errorf("response body does not contain %s, got: %s", v, rr.Body.String())
				}
			}
		})
	}
}

// STEP 6-4: uncomment this test
func TestAddItemE2e(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	db, closers, err := setupDB(t)
	if err != nil {
		t.Fatalf("failed to set up database: %v", err)
	}
	t.Cleanup(func() {
		for _, c := range closers {
			c()
		}
	})

	type wants struct {
		code     int
		name     string
		category string
		image    []byte
	}
	cases := map[string]struct {
		args  map[string]string
		image []byte
		wants
	}{
		"ok: correctly inserted": {
			args: map[string]string{
				"name":     "used iPhone 16e",
				"category": "phone",
			},
			image: []byte("default.jpg"),
			wants: wants{
				code: http.StatusOK,
			},
		},
		"ng: failed to insert": {
			args: map[string]string{
				"name":     "",
				"category": "phone",
			},
			image: nil,
			wants: wants{
				code: http.StatusBadRequest,
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			h := &Handlers{itemRepo: &itemRepository{db: db}}

			values := url.Values{}
			for k, v := range tt.args {
				values.Set(k, v)
			}
			req := httptest.NewRequest("POST", "/items", strings.NewReader(values.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			rr := httptest.NewRecorder()
			h.AddItem(rr, req)

			// check response
			if tt.wants.code != rr.Code {
				t.Errorf("expected status code %d, got %d", tt.wants.code, rr.Code)
			}
			if tt.wants.code >= 400 {
				return
			}
			for _, v := range tt.args {
				if !strings.Contains(rr.Body.String(), v) {
					t.Errorf("response body does not contain %s, got: %s", v, rr.Body.String())
				}
			}

			// STEP 6-4: check inserted data
			if rr.Code == http.StatusOK {
				var item AddItemRequest
				if err := json.NewDecoder(rr.Body).Decode(&item); err != nil {
					t.Errorf("failed to decode response body: %v", err)
				}
				if item.Name != tt.wants.name || item.Category != tt.wants.category || !bytes.Equal(item.Image, tt.wants.image) {
					t.Errorf("expected name, category,image was: %s, %s,%v, but recieved: %s, %s, %v", tt.wants.name, tt.wants.category, tt.wants.image, item.Name, item.Category, item.Image)
				}
			}
		})
	}
}

func setupDB(t *testing.T) (db *sql.DB, closers []func(), e error) {
	t.Helper()

	defer func() {
		if e != nil {
			for _, c := range closers {
				c()
			}
		}
	}()

	// create a temporary file for e2e testing
	f, err := os.CreateTemp(".", "*.sqlite3")
	if err != nil {
		return nil, nil, err
	}
	closers = append(closers, func() {
		f.Close()
		os.Remove(f.Name())
	})

	// set up tables
	db, err = sql.Open("sqlite3", f.Name())
	if err != nil {
		return nil, nil, err
	}
	closers = append(closers, func() {
		db.Close()
	})

	file, err := os.ReadFile(".../db/items.sql")
	if err != nil {
		t.Errorf("error reading SQL file: %v", err)
		return nil, nil, err
	}
	_, err = db.Exec(string(file))
	if err != nil {
		return nil, nil, err
	}

	return db, closers, nil
}
