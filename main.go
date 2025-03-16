package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"sync"
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

	var err error
	db, err = setupDatabase()
	if err != nil {
		log.Fatalf("Error setting up database: %v", err)
	}
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

func getFieldTags(model any, tagName string) ([]string, error) {
	var tags []string
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
			nestedTags, err := getFieldTags(reflect.New(field.Type).Interface(), tagName)
			if err != nil {
				return nil, err
			}
			tags = append(tags, nestedTags...)
		} else {
			if tagValue := field.Tag.Get(tagName); tagValue != "" {
				tags = append(tags, tagValue)
			}
		}
	}
	return tags, nil
}

func addHeaders(sheet *xlsx.Sheet, tags []string) {
	row := sheet.AddRow()
	for _, tag := range tags {
		cell := row.AddCell()
		cell.Value = tag
	}
}

func addRows[T any](sheet *xlsx.Sheet, data []T) error {
	for _, item := range data {
		row := sheet.AddRow()
		v := reflect.ValueOf(item)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if !field.IsValid() {
				return fmt.Errorf("invalid field at position %d", i)
			}

			// Handle nested structs
			if field.Kind() == reflect.Struct && field.CanInterface() {
				nested := field.Interface()
				nv := reflect.ValueOf(nested)
				for j := 0; j < nv.NumField(); j++ {
					nestedField := nv.Field(j)
					if nestedField.CanInterface() {
						cell := row.AddCell()
						cell.Value = fmt.Sprintf("%v", nestedField.Interface())
					}
				}
			} else {
				if field.CanInterface() {
					cell := row.AddCell()
					cell.Value = fmt.Sprintf("%v", field.Interface())
				}
			}
		}
	}
	return nil
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

var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 4*1024*1024)) // 4MB buffer
	},
}

func handler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	var salaries []Salary
	if err := db.WithContext(ctx).Find(&salaries).Error; err != nil {
		handleError(w, err, "Error fetching salaries", http.StatusInternalServerError)
		return
	}

	file := xlsx.NewFile()
	sheet, err := file.AddSheet("Salaries")
	if err != nil {
		handleError(w, err, "Error creating sheet", http.StatusInternalServerError)
		return
	}

	tags, err := getFieldTags(Salary{}, "xlsx")
	if err != nil {
		handleError(w, err, "Error getting field tags", http.StatusInternalServerError)
		return
	}

	addHeaders(sheet, tags)

	start, end := elapseTime("Adding rows")
	start()
	if err := addRows(sheet, salaries); err != nil {
		handleError(w, err, "Error generating rows", http.StatusInternalServerError)
		return
	}
	end()

	// Get buffer from pool
	buf := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buf)
	buf.Reset()

	// Set headers before writing data
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=salaries.xlsx")

	// Stream directly to response with compression
	startWriting, endWriting := elapseTime("Writing file")
	startWriting()

	// Write to buffer first
	if err := file.Write(buf); err != nil {
		handleError(w, err, "Error writing file", http.StatusInternalServerError)
		return
	}

	// Set content length and copy from buffer
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	if _, err := buf.WriteTo(w); err != nil {
		log.Printf("Error writing response: %v", err)
	}

	endWriting()

}
