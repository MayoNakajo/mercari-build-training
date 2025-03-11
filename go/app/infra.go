package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	// STEP 5-1: uncomment this line
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

var errImageNotFound = errors.New("image not found")

type Item struct {
	ID         int    `db:"id" json:"-"`
	Name       string `db:"name" json:"name"`
	CategoryID int    `db:"category_id" json:"category_id"`
	Image      string `db:"image_name" json:"image_name"`
}

// struct for category table in database
type Category struct {
	ID   int    `db:"id" json:"-"`
	Name string `db:"name" json:"name"`
}

type ItemName struct {
	ID       int    `db:"id" json:"-"`
	Name     string `db:"name" json:"name"`
	Category string `db:"category" json:"category"`
	Image    string `db:"image_name" json:"image_name"`
}

// Please run `go generate ./...` to generate the mock implementation
// ItemRepository is an interface to manage items.
//
//go:generate go run go.uber.org/mock/mockgen -source=$GOFILE -package=${GOPACKAGE} -destination=./mock_$GOFILE
type ItemRepository interface {
	Insert(ctx context.Context, item *Item) error
	GetAllItems(ctx context.Context) ([]Item, error)
	GetItem(ctx context.Context, itemID int) (*Item, error)
	GetCategoryID(ctx context.Context, keyword string) (int, error)
	Search(ctx context.Context, keyword string) (*sql.Rows, error)
}

// itemRepository is an implementation of ItemRepository
type itemRepository struct {
	db *sql.DB
}

// NewItemRepository creates a new itemRepository.
func NewItemRepository(database *sql.DB) ItemRepository {
	return &itemRepository{db: database}
}

// Insert inserts an item into the repository.
func (i *itemRepository) Insert(ctx context.Context, item *Item) error {
	// STEP 4-2: add an implementation to store an item
	// Check if file exists
	// _, err := os.Stat(i.fileName)
	// if os.IsNotExist(err) {
	// 	// Create file when it does not exist
	// 	newFile, createErr := os.Create(i.fileName)
	// 	// Handle error
	// 	if createErr != nil {
	// 		return errors.New("error creating file")
	// 	}
	// 	defer newFile.Close()

	// 	// Initialize new slice
	// 	newItem := []*Item{item}
	// 	// Convert to JSON
	// 	newItemJSON, _ := json.Marshal(newItem)

	// 	_, err := newFile.Write(newItemJSON)
	// 	if err != nil {
	// 		return errors.New("error writing as JSON")
	// 	}
	// } else {
	// 	var items []*Item
	// 	file, openErr := os.OpenFile(i.fileName, os.O_RDWR, 0644)

	// 	if openErr != nil {
	// 		return errors.New("error opening file")
	// 	}
	// 	defer file.Close()

	// 	items, err := i.GetAllItems(ctx)
	// 	if err != nil {
	// 		return errors.New("error retrieving items")
	// 	}

	// 	// Convert to JSON with new item added to existing
	// 	itemJSON, _ := json.Marshal(append(items, item))

	// 	_, err = file.Write(itemJSON)
	// 	if err != nil {
	// 		return errors.New("error writing JSON to file")
	// 	}

	// }
	// return nil

	_, err := i.db.ExecContext(ctx, "INSERT INTO items (name, category_id, image_name) VALUES (?, ?, ?)", item.Name, item.CategoryID, item.Image)
	if err != nil {
		return fmt.Errorf("error inserting item: %w", err)

	}
	return err
}

func (i *itemRepository) GetAllItems(ctx context.Context) ([]Item, error) {
	// var items []*Item
	// file, err := os.Open(i.fileName)
	// if err != nil {
	// 	return nil, errors.New("error opening file")
	// }

	// defer file.Close()
	// // Decode json into items array
	// err = json.NewDecoder(file).Decode(&items)
	// if err != nil {
	// 	return nil, errors.New("error decoding file")
	// }
	// return items, nil
	rows, err := i.db.QueryContext(ctx, `SELECT items.id, items.name, categories.categoryname AS category, items.image_name
	FROM items JOIN categories ON items.category_id = categories.id`)

	if err != nil {
		return nil, fmt.Errorf("error retrieving all items: %w", err)
	}
	defer rows.Close()

	var items []Item

	for rows.Next() {
		var item Item
		err := rows.Scan(ctx, `SELECT items.id, items.name, categories.name
		FROM items JOIN categories ON items.category_id = categories.id;`)
		if err != nil {
			return nil, errors.New("error scanning items")
		}
		items = append(items, item)

	}
	return items, nil
}

func (i *itemRepository) GetItem(ctx context.Context, itemID int) (*Item, error) {
	// items, err := i.GetAllItems(ctx)
	// if err != nil {
	// 	return nil, errors.New("no items found")
	// }
	// returnItem := items[itemID]

	// return returnItem, nil

	// query := `SELECT items.id, items.name, categories.name, items.image_name
	// 			FROM items
	// 			JOIN categories ON items.category_id = categories.id
	// 			WHERE items.id = ?`
	// rows := i.db.QueryRowContext(ctx, query)

	var item Item
	rows := i.db.QueryRowContext(ctx, "SELECT id, name, category, image_name FROM items WHERE id = ?", itemID)

	err := rows.Scan(`SELECT items.id, items.name, categories.name
			FROM items JOIN categories ON items.category_id = categories.id;`)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("error finding item")
		}
		return nil, errors.New("error finding item")
	}
	return &item, nil
}

func (i *itemRepository) GetCategoryID(ctx context.Context, categoryName string) (int, error) {
	var categoryID int

	row := i.db.QueryRowContext(ctx, "SELECT id FROM categories WHERE name = ?", categoryName)

	err := row.Scan(&categoryID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			res, err := i.db.ExecContext(ctx, "INSERT INTO categories (name) VALUES (?)", categoryName)
			if err != nil {
				return 0, fmt.Errorf("error inserting category: %w", err)
			}
			id, err := res.LastInsertId()
			if err != nil {
				return 0, fmt.Errorf("error retrieving category id: %w", err)
			}
			categoryID = int(id)
		} else {
			return 0, fmt.Errorf("error retrieving category id: %w", err)
		}
	}
	return categoryID, nil
}

// StoreImage stores an image and returns an error if any.
// This package doesn't have a related interface for simplicity.
func StoreImage(fileName string, image []byte) error {
	// STEP 4-4: add an implementation to store an image
	err := os.WriteFile(fileName, image, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (i *itemRepository) Search(ctx context.Context, keyword string) (*sql.Rows, error) {
	rows, err := i.db.QueryContext(ctx, `SELECT items.id, items.name, categories.name, items.image_name
				FROM items JOIN categories ON items.category_id = categories.id
				WHERE items.name LIKE ?`, "%"+keyword+"%")

	// if err != nil {
	// 	return nil, errors.New("error searching item")
	// }
	// defer rows.Close()

	// var items []Item
	// for rows.Next() {
	// 	var item Item
	// 	if err := rows.Scan(&item.ID, &item.Name, &item.CategoryID, &item.Image); err != nil {
	// 		return nil, fmt.Errorf("failed to scan item: %w", err)
	// 	}
	// 	items = append(items, item)
	// }

	// err = rows.Err()
	// if err != nil {
	// 	return nil, err
	// }

	return rows, err
}

func InitDB(dbPath string) (*sql.DB, error) {
	database, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, errors.New("error opening database")
	}

	tables := `
	CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS items (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		category_id INTEGER NOT NULL,
		image_name TEXT NOT NULL,
		FOREIGN KEY (category_id) REFERENCES categories(id)
	);`

	_, err = database.Exec(tables)
	if err != nil {
		return nil, errors.New("error creating database table")
	}

	return database, nil
}
