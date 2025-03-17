# Go Excel Service

This is a Go-based service that provides functionality to download Excel files from a database using GORM.

## Project Structure

```plaintext
.env
.env.example
.gitignore
Dockerfile
go.mod
go.sum
main.go
```

## Getting Started

### Prerequisites

- Go 1.24.1 or later
- Docker (optional, for containerization)

### Installation

1. Clone the repository:

    ```sh
    git clone https://github.com/yourusername/go-excel-service.git
    cd go-excel-service
    ```

2. Copy the example environment file and update it with your database URL:

    ```sh
    cp .env.example .env
    ```

3. Install dependencies:

    ```sh
    go mod download
    ```

### Running the Application

1. Run the application:

    ```sh
    go run main.go
    ```

2. The server will start on `localhost:8080`. You can access the download endpoint at `http://localhost:8080/download`.

### Building with Docker

1. Build the Docker image:

    ```sh
    docker build -t go-excel-service .
    ```

2. Run the Docker container:

    ```sh
    docker run -p 8080:8080 --env-file .env go-excel-service
    ```

## Endpoints

- `GET /download`: Downloads an Excel file containing salary data.

## Environment Variables

- `DATABASE_URL`: The URL of the PostgreSQL database.

## License

This project is licensed under the MIT License.
