package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/xuri/excelize/v2"
)

type NetflixShow struct {
	ShowID      string
	Type        string
	Title       string
	Director    *string
	Cast        *string
	Country     *string
	DateAdded   *time.Time
	ReleaseYear int
	Rating      *string
	Duration    *string
	ListedIn    *string
	Description *string
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	if os.Getenv("DATABASE_URL") == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
}

func main() {
	http.HandleFunc("/export", exportHandler)
	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func exportHandler(w http.ResponseWriter, r *http.Request) {
	shows, err := fetchNetflixShows()
	if err != nil {
		log.Printf("Error fetching data: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	file, err := createExcelFile(shows)
	if err != nil {
		log.Printf("Error creating excel file: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=netflix_shows.xlsx")

	if err := file.Write(w); err != nil {
		log.Printf("Error writing response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func fetchNetflixShows() ([]NetflixShow, error) {
	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, fmt.Errorf("database connection error: %w", err)
	}
	defer conn.Close(context.Background())

	rows, err := conn.Query(context.Background(), "SELECT * FROM netflix_shows")
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	var shows []NetflixShow
	for rows.Next() {
		var show NetflixShow
		if err := rows.Scan(
			&show.ShowID,
			&show.Type,
			&show.Title,
			&show.Director,
			&show.Cast,
			&show.Country,
			&show.DateAdded,
			&show.ReleaseYear,
			&show.Rating,
			&show.Duration,
			&show.ListedIn,
			&show.Description,
		); err != nil {
			return nil, fmt.Errorf("row scan error: %w", err)
		}
		shows = append(shows, show)
	}

	return shows, nil
}

func createExcelFile(shows []NetflixShow) (*excelize.File, error) {
	f := excelize.NewFile()

	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return nil, fmt.Errorf("stream writer error: %w", err)
	}

	headers := []any{"Show ID", "Type", "Title", "Director", "Cast", "Country",
		"Date Added", "Release Year", "Rating", "Duration", "Listed In", "Description"}
	if err := sw.SetRow("A1", headers); err != nil {
		return nil, fmt.Errorf("header write error: %w", err)
	}

	for i, show := range shows {

		row := []any{
			show.ShowID,
			show.Type,
			show.Title,
			show.Director,
			show.Cast,
			show.Country,
			show.DateAdded,
			show.ReleaseYear,
			show.Rating,
			show.Duration,
			show.ListedIn,
			show.Description,
		}
		if err := sw.SetRow(fmt.Sprintf("A%d", i+2), row); err != nil {
			return nil, fmt.Errorf("row write error: %w", err)
		}
	}

	if err := sw.Flush(); err != nil {
		return nil, fmt.Errorf("stream flush error: %w", err)
	}

	return f, nil
}
