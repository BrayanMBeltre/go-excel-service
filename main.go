package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/joho/godotenv"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type NetflixShow struct {
	ShowID      string
	Type        string
	Title       string
	Director    string
	CastMembers string
	Country     string
	DateAdded   time.Time
	ReleaseYear int
	Rating      string
	Duration    string
	ListedIn    string
	Description string
}

var db *gorm.DB

func init() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	if os.Getenv("DATABASE_URL") == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	var err error
	db, err = gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")), &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Error getting database instance: %v", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
}

func main() {
	http.HandleFunc("/export", handler)
	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}

func GetStructFieldNames(input any) []string {
	t := reflect.TypeOf(input)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil
	}

	var fields []string
	for i := range t.NumField() {
		field := t.Field(i)
		fields = append(fields, field.Name)
	}
	return fields
}

func handler(w http.ResponseWriter, r *http.Request) {

	var shows []NetflixShow
	if err := db.Find(&shows).Error; err != nil {
		log.Printf("Error fetching netflix shows: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	columns := GetStructFieldNames(NetflixShow{})

	f := excelize.NewFile()

	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		log.Printf("Error creating excel file: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	headers := []any{}

	for _, column := range columns {
		headers = append(headers, column)
	}

	if err := sw.SetRow("A1", headers); err != nil {
		log.Printf("Error writing headers: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	for i, show := range shows {
		row := []any{}
		val := reflect.ValueOf(show)
		for _, column := range columns {
			fieldVal := val.FieldByName(column)
			row = append(row, fieldVal.Interface())
		}
		if err := sw.SetRow(fmt.Sprintf("A%d", i+2), row); err != nil {
			log.Printf("Error writing row: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	if err := sw.Flush(); err != nil {
		log.Printf("Error flushing stream: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=netflix_shows.xlsx")

	if err := f.Write(w); err != nil {
		log.Printf("Error writing response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

}
