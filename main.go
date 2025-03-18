package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/joho/godotenv"
	"github.com/xuri/excelize/v2"
)

const (
	serverAddress   = "localhost:8080"
	maxOpenConns    = 25
	maxIdleConns    = 5
	connMaxLifetime = 5 * time.Minute
)

type DATA struct {
	ConvocatoriaID  int    `json:"convocatoria_id" xlsx:"Convocatoria ID"`
	CodigoSolicitud string `json:"codigo_solicitud" xlsx:"Codigo Solicitud"`
	Nombres         string `json:"nombres" xlsx:"Nombres"`
	Apellidos       string `json:"apellidos" xlsx:"Apellidos"`
	Identificacion  string `json:"identificacion" xlsx:"Identificacion"`
	Email           string `json:"email" xlsx:"Email"`
	Telefono        string `json:"telefono" xlsx:"Telefono"`
	Institucion     string `json:"institucion" xlsx:"Institucion"`
	Edad            int    `json:"edad" xlsx:"Edad"`
	FechaNacimiento string `json:"fecha_nacimiento" xlsx:"Fecha Nacimiento"`
	Genero          string `json:"genero" xlsx:"Genero"`
	Direccion       string `json:"direccion" xlsx:"Direccion"`
	Municipio       string `json:"municipio" xlsx:"Municipio"`
	ProvinciaNombre string `json:"provincia_nombre" xlsx:"Provincia"`
	CampusNombre    string `json:"campus_nombre" xlsx:"Campus"`
	CarreraNombre   string `json:"carrera_nombre" xlsx:"Carrera"`
	AreaCarrera     string `json:"area_carrera" xlsx:"Area Carrera"`
	GradoNombre     string `json:"grado_nombre" xlsx:"Grado"`
	PaisNombre      string `json:"pais_nombre" xlsx:"Pais"`
	Modalidad       string `json:"modalidad" xlsx:"Modalidad"`
	EstadoNombre    string `json:"estado_nombre" xlsx:"Estado"`
	Puntaje         int    `json:"puntaje" xlsx:"Puntaje"`
	Comentario      string `json:"comentario" xlsx:"Comentario"`
}

type Links struct {
	First string  `json:"first"`
	Last  string  `json:"last"`
	Prev  *string `json:"prev"`
	Next  *string `json:"next"`
}

type Meta struct {
	CurrentPage int    `json:"current_page"`
	From        int    `json:"from"`
	LastPage    int    `json:"last_page"`
	Links       []Link `json:"links"`
	Path        string `json:"path"`
	PerPage     int    `json:"per_page"`
	To          int    `json:"to"`
	Total       int    `json:"total"`
}

type Link struct {
	URL    *string `json:"url"`
	Label  string  `json:"label"`
	Active bool    `json:"active"`
}

type APIError struct {
	Message string `json:"message"`
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}
}

func main() {
	server := &http.Server{
		Addr:         "localhost:8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		Handler:      http.DefaultServeMux,
	}

	http.HandleFunc("GET /download", handler)
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

	for i := range t.NumField() {
		field := t.Field(i)
		if field.Type.Kind() == reflect.Struct && field.Anonymous {
			nestedHeaders, err := getHeaders(reflect.New(field.Type).Interface())
			if err != nil {
				return nil, err
			}
			headers = append(headers, nestedHeaders...)
		} else {
			headers = append(headers, field.Tag.Get("json"))
		}
	}

	return headers, nil
}

func getRows[T any](data []T) ([][]any, error) {
	var rows [][]any

	for _, item := range data {
		var row []any
		v := reflect.ValueOf(item)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		for i := range v.NumField() {
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

func getData(convocationId string) (*[]DATA, error) {
	start, end := elapseTime("Fetching data")
	start()

	var apiResponse []DATA

	// Fetch apiResponse with API_URL and set Authorization header with Bearer token
	apiURL := os.Getenv("API_URL")
	token := os.Getenv("API_TOKEN")
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/backoffice/v1/solicitud/excel?convocatoria=%s", apiURL, convocationId), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	log.Printf("Requesting data from %s", req.URL.String())

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK {
		if contentType == "application/json" {
			var apiError APIError
			if err := json.Unmarshal(body, &apiError); err != nil {
				return nil, fmt.Errorf("error decoding error response: %w", err)
			}
			return nil, fmt.Errorf("error response from API: %s", apiError.Message)
		} else {
			return nil, fmt.Errorf("error response from API: %s", string(body))
		}
	}

	if contentType != "application/json" {
		return nil, fmt.Errorf("unexpected content type: %s", contentType)
	}

	// print json idented
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err != nil {
		return nil, fmt.Errorf("error indenting json: %w", err)
	}
	log.Printf("Response: %s", prettyJSON.String())

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	end()
	return &apiResponse, nil
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
	w.Header().Set("Content-Disposition", "attachment; filename=data.xlsx")

	file.Write(w)

	end()

	return nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Downloading data")

	convocationId := r.URL.Query().Get("convocationId")
	if convocationId == "" {
		handleError(w, nil, "Missing convocationId", http.StatusBadRequest)
		return
	}

	apiResponse, err := getData(convocationId)
	if err != nil {
		handleError(w, err, "Error fetching data", http.StatusInternalServerError)
		return
	}

	if err := downloadFile(w, *apiResponse); err != nil {
		handleError(w, err, "Error creating file", http.StatusInternalServerError)
		return
	}
}
