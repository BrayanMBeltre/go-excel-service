package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"sync"
	"time"

	"context"

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

var (
	db           *gorm.DB
	excelHeaders []string
	fieldIndexes []int
)

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

func initFieldMetadata() {
	t := reflect.TypeOf(NetflixShow{})

	excelHeaders = make([]string, t.NumField())
	fieldIndexes = make([]int, t.NumField())

	for i := range t.NumField() {
		field := t.Field(i)
		if tag := field.Tag.Get("xlsx"); tag != "" {
			excelHeaders[i] = tag
		} else {
			excelHeaders[i] = field.Name
		}
		fieldIndexes[i] = i
	}
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

	initFieldMetadata()
}

func main() {
	server := &http.Server{
		Addr:         "localhost:8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		Handler:      http.DefaultServeMux,
	}

	http.HandleFunc("/download", handler)
	log.Printf("Starting server on %s", serverAddress)
	log.Fatal(server.ListenAndServe())
}

func addHeaders(sheet *xlsx.Sheet, tags []string) {
	row := sheet.AddRow()
	for _, tag := range tags {
		cell := row.AddCell()
		cell.Value = tag
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

func addRowsWithGorutines(sheet *xlsx.Sheet, shows []NetflixShow) {
	var wg sync.WaitGroup
	for _, show := range shows {
		wg.Add(1)
		go func(show NetflixShow) {
			defer wg.Done()
			row := sheet.AddRow()
			v := reflect.ValueOf(show)
			for i := range v.NumField() {
				cell := row.AddCell()
				cell.Value = fmt.Sprintf("%v", v.Field(i).Interface())
			}
		}(show)
	}
	wg.Wait()
}

func handleError(w http.ResponseWriter, err error, message string, code int) {
	log.Printf("%s: %v", message, err)
	http.Error(w, message, code)
}

func elapseTime(message string) (start, end func()) {
	var startTime, endTime time.Time

	start = func() {
		startTime = time.Now()
	}

	end = func() {
		endTime = time.Now()
		fmt.Printf("Elapsed time for %s: %v \n", message, endTime.Sub(startTime))
	}

	return start, end
}

func handler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var shows []NetflixShow
	if err := db.WithContext(ctx).Find(&shows).Error; err != nil {
		handleError(w, err, "Error fetching Netflix shows", http.StatusInternalServerError)
		return
	}

	file := xlsx.NewFile()
	sheet, err := file.AddSheet("Netflix Shows")
	if err != nil {
		handleError(w, err, "Error adding sheet", http.StatusInternalServerError)
		return
	}

	addHeaders(sheet, excelHeaders)

	start, end := elapseTime("Adding rows with goroutines")
	start()
	addRows(sheet, shows)
	end()

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=netflix_shows.xlsx")

	if err := file.Write(w); err != nil {
		handleError(w, err, "Error writing response", http.StatusInternalServerError)
		return
	}
}
