package app

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Server struct {
	// Port is the port number to listen on.
	Port string
	// ImageDirPath is the path to the directory storing images.
	ImageDirPath string
}

// Run is a method to start the server.
// This method returns 0 if the server started successfully, and 1 otherwise.
func (s Server) Run() int {
	// set up logger
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(logger)
	// STEP 4-6: set the log level to DEBUG
	slog.SetLogLoggerLevel(slog.LevelInfo)

	// set up CORS settings
	frontURL, found := os.LookupEnv("FRONT_URL")
	if !found {
		frontURL = "http://localhost:3000"
	}

	// STEP 5-1: set up the database connection
	database, err := InitDB("db/mercari.sqlite3")
	// set up handlers
	itemRepo := NewItemRepository(database)
	h := &Handlers{imgDirPath: s.ImageDirPath, itemRepo: itemRepo}

	// set up routes
	mux := http.NewServeMux()
	// mux.HandleFunc("GET /", h.GetItems)
	mux.HandleFunc("POST /items", h.AddItem)
	mux.HandleFunc("GET /items", h.GetItems)
	mux.HandleFunc("GET /items/{item_id}", h.GetItem)
	mux.HandleFunc("GET /images/{filename}", h.GetImage)
	mux.HandleFunc("GET /search", h.SearchItems)

	// start the server
	slog.Info("http server started on", "port", s.Port)
	err = http.ListenAndServe(":"+s.Port, simpleCORSMiddleware(simpleLoggerMiddleware(mux), frontURL, []string{"GET", "HEAD", "POST", "OPTIONS"}))
	if err != nil {
		slog.Error("failed to start server: ", "error", err)
		return 1
	}

	return 0
}

type Handlers struct {
	// imgDirPath is the path to the directory storing images.
	imgDirPath string
	itemRepo   ItemRepository
}

type HelloResponse struct {
	Message string `json:"message"`
}

// STEP 4-3
type GetAllItemsResponse struct {
	Items []Item `json:"items"`
}

// STEP 4-5
type GetSingleItemResponse struct {
	Item *Item `json:"item"`
}

// Hello is a handler to return a Hello, world! message for GET / .
func (s *Handlers) Hello(w http.ResponseWriter, r *http.Request) {
	resp := HelloResponse{Message: "Hello, world!"}
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// STEP 4-3
func (s *Handlers) GetItems(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// DEBUG
	slog.Info("Fetching items...")

	allItems, err := s.itemRepo.GetAllItems(ctx)
	if err != nil {
		slog.Error("Error retrieving all items", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get response
	resp := GetAllItemsResponse{Items: allItems}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Handlers) GetItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	itemID := r.PathValue("item_id")
	if itemID == "" {
		http.Error(w, "item ID required", http.StatusBadRequest)
		return
	}

	// Convert string to int
	idNum, err := strconv.Atoi(itemID)
	if err != nil {
		http.Error(w, "invalid item ID", http.StatusBadRequest)
	} else {
		item, err := s.itemRepo.GetItem(ctx, idNum)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Get response
		resp := GetSingleItemResponse{Item: item}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	}
}

type AddItemRequest struct {
	Name     string `form:"name"`
	Category string `form:"category"` // STEP 4-2: add a category field
	Image    []byte `form:"image"`    // STEP 4-4: add an image field
}

type AddItemResponse struct {
	Message string `json:"message"`
}

// parseAddItemRequest parses and validates the request to add an item.
func parseAddItemRequest(r *http.Request) (*AddItemRequest, error) {
	req := &AddItemRequest{
		Name: r.FormValue("name"),
		// STEP 4-2: add a category field
		Category: r.FormValue("category"),
	}

	// STEP 4-4: add an image field
	var f multipart.File
	// FOrm image file
	f, _, err := r.FormFile("image")
	if err != nil {
		return nil, errors.New("error reading image file")
	}
	defer f.Close()
	// Convert image to byte array
	byteArray, err := io.ReadAll(f)
	if err != nil {
		return nil, errors.New("error reading image")
	}
	req.Image = byteArray

	// validate the request
	if req.Name == "" {
		return nil, errors.New("name is required")
	}

	// STEP 4-2: validate the category field
	if req.Category == "" {
		return nil, errors.New("category is required")
	}
	// STEP 4-4: validate the image field
	if len(req.Image) == 0 {
		return nil, errors.New("image is required")
	}
	return req, nil
}

// AddItem is a handler to add a new item for POST /items .
func (s *Handlers) AddItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	req, err := parseAddItemRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// STEP 4-4: uncomment on adding an implementation to store an image
	fileName, err := s.storeImage(req.Image)
	if err != nil {
		slog.Error("failed to store image: ", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	categoryID, err := s.itemRepo.GetCategoryID(ctx, req.Category)
	if err != nil {
		errors.New("error retrieving category ID")
		return
	}
	item := &Item{
		Name: req.Name,
		// STEP 4-2: add a category field
		CategoryID: categoryID,
		// STEP 4-4: add an image field
		Image: fileName,
	}
	message := fmt.Sprintf("item received: %s", item.Name)
	slog.Info(message)

	// STEP 4-2: add an implementation to store an item
	err = s.itemRepo.Insert(ctx, item)
	if err != nil {
		slog.Error("failed to store item: ", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := AddItemResponse{Message: message}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// storeImage stores an image and returns the file path and an error if any.
// this method calculates the hash sum of the image as a file name to avoid the duplication of a same file
// and stores it in the image directory.
func (s *Handlers) storeImage(image []byte) (filePath string, err error) {
	// STEP 4-4: add an implementation to store an image

	// - calc hash sum
	hash := sha256.Sum256(image)

	// - build image file path
	filePath = fmt.Sprintf("%x.jpg", hash)
	hashedFilePath := filepath.Join(s.imgDirPath, filePath)

	// - check if the image already exists
	_, err = os.Stat(hashedFilePath)
	if err == nil {
		return filePath, nil
	}

	// Store the image
	err = StoreImage(hashedFilePath, image)
	if err != nil {
		return "", err
	}

	// - return the image file path
	return filePath, nil
}

type GetImageRequest struct {
	FileName string // path value
}

// parseGetImageRequest parses and validates the request to get an image.
func parseGetImageRequest(r *http.Request) (*GetImageRequest, error) {
	req := &GetImageRequest{
		FileName: r.PathValue("filename"), // from path parameter
	}

	// validate the request
	if req.FileName == "" {
		return nil, errors.New("filename is required")
	}

	return req, nil
}

// GetImage is a handler to return an image for GET /images/{filename} .
// If the specified image is not found, it returns the default image.
func (s *Handlers) GetImage(w http.ResponseWriter, r *http.Request) {
	req, err := parseGetImageRequest(r)
	if err != nil {
		slog.Warn("failed to parse get image request: ", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	imgPath, err := s.buildImagePath(req.FileName)
	if err != nil {
		if !errors.Is(err, errImageNotFound) {
			slog.Warn("failed to build image path: ", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// when the image is not found, it returns the default image without an error.
		slog.Debug("image not found", "filename", imgPath)
		imgPath = filepath.Join(s.imgDirPath, "default.jpg")
	}

	slog.Info("returned image", "path", imgPath)
	http.ServeFile(w, r, imgPath)
}

// buildImagePath builds the image path and validates it.
func (s *Handlers) buildImagePath(imageFileName string) (string, error) {
	imgPath := filepath.Join(s.imgDirPath, filepath.Clean(imageFileName))

	// to prevent directory traversal attacks
	rel, err := filepath.Rel(s.imgDirPath, imgPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid image path: %s", imgPath)
	}

	// validate the image suffix
	if !strings.HasSuffix(imgPath, ".jpg") && !strings.HasSuffix(imgPath, ".jpeg") {
		return "", fmt.Errorf("image path does not end with .jpg or .jpeg: %s", imgPath)
	}

	// check if the image exists
	_, err = os.Stat(imgPath)
	if err != nil {
		return imgPath, errImageNotFound
	}

	return imgPath, nil
}

func (s *Handlers) SearchItems(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	keyword := r.URL.Query().Get("keyword")

	if keyword == "" {
		http.Error(w, "keyword is required", http.StatusBadRequest)
		return
	}

	// items, err := s.itemRepo.Search(ctx, keyword)
	// if err != nil {
	// 	http.Error(w, "failed to search items", http.StatusInternalServerError)
	// 	return
	// }

	// // Send JSON response
	// resp := struct {
	// 	Items []Item `json:"items"`
	// }{Items: items}

	// w.Header().Set("Content-Type", "application/json")
	// err = json.NewEncoder(w).Encode(resp)
	// if err != nil {
	// 	http.Error(w, "failed to encode response", http.StatusInternalServerError)
	// }

	rows, err := s.itemRepo.Search(ctx, keyword)
	if err != nil {
		http.Error(w, "failed to search items", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type itemResponse struct {
		ID       int    `json:"-"`
		Name     string `json:"name"`
		Category string `json:"category"`
		Image    string `json:"image_name"`
	}

	var responseItems []itemResponse
	for rows.Next() {
		var item itemResponse
		if err := rows.Scan(&item.ID, &item.Name, &item.Category, &item.Image); err != nil {
			http.Error(w, "failed to scan item", http.StatusInternalServerError)
			return
		}
		responseItems = append(responseItems, item)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "error iterating through items", http.StatusInternalServerError)
		return
	}

	resp := struct {
		Items []itemResponse `json:"items"`
	}{Items: responseItems}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
