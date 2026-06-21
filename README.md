# Go Web Template

A lightweight, modern template for building web applications with Go, PostgreSQL, and templ.

## Features

- **Go Backend**: Uses Gin for routing and API endpoints
- **PostgreSQL Database**: With Goose migrations for schema management
- **Type-safe Database Access**: Generated with sqlc
- **HTML Templating**: Using the modern templ templating engine
- **Docker Integration**: Containerized for easy deployment
- **API Ready**: Pre-configured REST API structure

## Quick Start

1. Clone this repository:
   ```
   git clone https://github.com/dbut2/web-template.git
   ```

2. Run the application with Docker:
   ```
   docker compose up
   ```

3. Visit `http://localhost:8080` to see the application

## Project Structure

- `/go` - Backend Go code
  - `/database` - Database access with sqlc-generated code
  - `/service` - HTTP service with handlers and API endpoints
- `/db` - Database schema and migrations
  - `/migrations` - Goose SQL migrations
  - `/queries` - SQL queries for sqlc generation
- `/web` - Frontend templates using templ

## Development

### Prerequisites

- Go 1.26+
- Docker

### Build Commands

- Generate templ templates:
  ```
  go generate ./web
  ```

- Generate database code:
  ```
  go generate ./db
  ```

- Build the application:
  ```
  go build -o server ./go
  ```
