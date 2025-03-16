package main

import (
	"context"
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

const (
	serverAddress   = "localhost:8080"
	maxOpenConns    = 25
	maxIdleConns    = 5
	connMaxLifetime = 5 * time.Minute
)

type Tabler interface {
	TableName() string
}

func (Salary) TableName() string {
	return "salary"
}

type Salary struct {
	EmployeeId int       `xlsx:"Employee ID"`
	Amount     float32   `xlsx:"Amount"`
	FromDate   time.Time `xlsx:"From Date"`
	ToDate     time.Time `xlsx:"To Date"`
}

func (Title) TableName() string {
	return "title"
}

type Title struct {
	EmployeeId int       `xlsx:"Employee ID"`
	Title      string    `xlsx:"Title"`
	FromDate   time.Time `xlsx:"From Date"`
	ToDate     time.Time `xlsx:"To Date"`
}

func (Employee) TableName() string {
	return "employee"
}

type Employee struct {
	Id        int       `xlsx:"ID"`
	BirthDate time.Time `xlsx:"Birth Date"`
	FirstName string    `xlsx:"First Name"`
	LastName  string    `xlsx:"Last Name"`
	Gender    string    `xlsx:"Gender"`
	HireDate  time.Time `xlsx:"Hire Date"`
}

type EmployeeSalary struct {
	Employee
	Salary
	Title
}

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
	db *gorm.DB
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

func init() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	// var err error
	// db, err = setupDatabase()
	// if err != nil {
	// 	log.Fatalf("Error setting up database: %v", err)
	// }
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

func getHeaders[T any](model T) ([]string, error) {
	var headers []string
	t := reflect.TypeOf(model)

	if t == nil {
		return nil, fmt.Errorf("nil model type")
	}

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Type.Kind() == reflect.Struct && field.Anonymous {
			nestedHeaders, err := getHeaders(reflect.New(field.Type).Interface())
			if err != nil {
				return nil, err
			}
			headers = append(headers, nestedHeaders...)
		} else {
			headers = append(headers, field.Tag.Get("xlsx"))
		}
	}

	return headers, nil
}

func getRows[T any](data []T) ([][]interface{}, error) {
	var rows [][]interface{}

	for _, item := range data {
		var row []interface{}
		v := reflect.ValueOf(item)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if !field.IsValid() {
				return nil, fmt.Errorf("invalid field at position %d", i)
			}

			row = append(row, field.Interface())
		}

		rows = append(rows, row)
	}

	return rows, nil
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

func getData(ctx context.Context, w http.ResponseWriter) ([]Salary, error) {
	start, end := elapseTime("Fetching data")
	start()

	var salaries []Salary

	if err := db.WithContext(ctx).Find(&salaries).Error; err != nil {
		handleError(w, err, "Error fetching salaries", http.StatusInternalServerError)
		end()
		return nil, err
	}

	end()
	return salaries, nil
}

func downloadFile[T any](w http.ResponseWriter, data []T) error {
	start, end := elapseTime("Creating file")
	start()

	file := excelize.NewFile()
	defer file.Close()

	sheetName := "Sheet1"
	_, err := file.NewSheet(sheetName)

	if err != nil {
		handleError(w, err, "Error creating sheet", http.StatusInternalServerError)
	}

	// Set headers
	headers, err := getHeaders(data[0])
	if err != nil {
		return fmt.Errorf("error getting field tags: %w", err)
	}
	file.SetSheetRow(sheetName, "A1", &headers)

	// Add rows
	rows, err := getRows(data)
	if err != nil {
		return fmt.Errorf("error getting rows: %w", err)
	}
	for i, row := range rows {
		cell := fmt.Sprintf("A%d", i+2)
		file.SetSheetRow(sheetName, cell, &row)
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=salaries.xlsx")

	file.Write(w)

	end()

	return nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	// ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	// defer cancel()

	// data, err := getData(ctx, w)
	// if err != nil {
	// 	return
	// }

	data := []Salary{
		{EmployeeId: 1, Amount: 1000, FromDate: time.Now(), ToDate: time.Now()},
		{EmployeeId: 2, Amount: 2000, FromDate: time.Now(), ToDate: time.Now()},
		{EmployeeId: 3, Amount: 3000, FromDate: time.Now(), ToDate: time.Now()},
	}

	if err := downloadFile(w, data); err != nil {
		handleError(w, err, "Error streaming salaries", http.StatusInternalServerError)
		return
	}
}
