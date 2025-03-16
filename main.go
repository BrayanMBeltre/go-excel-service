package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/joho/godotenv"
	"github.com/tealeg/xlsx"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	serverAddress   = "localhost:8080"
	maxOpenConns    = 25
	maxIdleConns    = 5
	connMaxLifetime = 5 * time.Minute
)

type NetflixShow struct {
	ShowID      string    `xlsx:"Id"`
	Type        string    `xlsx:"Type"`
	Title       string    `xlsx:"Title"`
	Director    string    `xlsx:"Director"`
	CastMembers string    `xlsx:"Cast Members"`
	Country     string    `xlsx:"Country"`
	DateAdded   time.Time `xlsx:"Date Added"`
	ReleaseYear int       `xlsx:"Release Year"`
	Rating      string    `xlsx:"Rating"`
	Duration    string    `xlsx:"Duration"`
	ListedIn    string    `xlsx:"Listed In"`
	Description string    `xlsx:"Description"`
}

var db *gorm.DB

func setupDatabase() (*gorm.DB, error) {
	if os.Getenv("DATABASE_URL") == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	db, err := gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")), &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("error getting database instance: %w", err)
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	return db, nil
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	var err error
	db, err = setupDatabase()
	if err != nil {
		log.Fatalf("Error setting up database: %v", err)
	}
}

func main() {
	http.HandleFunc("/export", handler)
	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(serverAddress, nil))
}

func addHeaders(sheet *xlsx.Sheet, model any) {
	t := reflect.TypeOf(model)
	row := sheet.AddRow()

	for i := range t.NumField() {
		field := t.Field(i)
		cell := row.AddCell()
		cell.Value = field.Tag.Get("xlsx")
	}
}

func addRows(sheet *xlsx.Sheet, shows []NetflixShow) {
	for _, show := range shows {
		row := sheet.AddRow()
		v := reflect.ValueOf(show)
		for i := range v.NumField() {
			cell := row.AddCell()
			cell.Value = fmt.Sprintf("%v", v.Field(i).Interface())
		}
	}
}

func handleError(w http.ResponseWriter, err error, message string, code int) {
	log.Printf("%s: %v", message, err)
	http.Error(w, message, code)
}

func handler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var shows []NetflixShow
	if err := db.WithContext(ctx).Limit(10).Find(&shows).Error; err != nil {
		handleError(w, err, "Error fetching Netflix shows", http.StatusInternalServerError)
		return
	}

	file := xlsx.NewFile()
	sheet, err := file.AddSheet("Netflix Shows")
	if err != nil {
		handleError(w, err, "Error adding sheet", http.StatusInternalServerError)
		return
	}

	addHeaders(sheet, NetflixShow{})
	addRows(sheet, shows)

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=netflix_shows.xlsx")

	if err := file.Write(w); err != nil {
		handleError(w, err, "Error writing response", http.StatusInternalServerError)
		return
	}
}
