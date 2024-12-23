package main

import (
	"archive/zip"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

// Константы для подключения к базе данных
const (
	dbUser     = "validator"
	dbPassword = "val1dat0r"
	dbName     = "project-sem-1"
	dbHost     = "localhost"
	dbPort     = "5432"
)

var db *sql.DB

// Структура для ответа POST
type PostResponse struct {
	TotalItems      int     `json:"total_items"`
	TotalCategories int     `json:"total_categories"`
	TotalPrice      float64 `json:"total_price"`
}

// Инициализация базы данных
func initDB() {
	var err error
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
}

// Обработчик для POST /api/v0/prices
func handlePostPrices(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Не удалось загрузить файл", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Разархивация и запись данных в БД
	var totalItems int
	var totalCategoriesMap = make(map[string]bool)
	var totalPrice float64

	reader, err := zip.NewReader(file, r.ContentLength)
	if err != nil {
		http.Error(w, "Ошибка чтения архива", http.StatusInternalServerError)
		return
	}

	for _, f := range reader.File {
		if strings.HasSuffix(f.Name, ".csv") {
			csvFile, err := f.Open()
			if err != nil {
				http.Error(w, "Ошибка открытия CSV", http.StatusInternalServerError)
				return
			}
			defer csvFile.Close()

			csvReader := csv.NewReader(csvFile)
			_, _ = csvReader.Read() // Пропускаем заголовок
			for {
				record, err := csvReader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					http.Error(w, "Ошибка чтения CSV", http.StatusInternalServerError)
					return
				}

				id := record[0]
				createdAt := record[1]
				name := record[2]
				category := record[3]
				price, _ := strconv.ParseFloat(record[4], 64)

				_, err = db.Exec("INSERT INTO prices (id, created_at, name, category, price) VALUES ($1, $2, $3, $4, $5)",
					id, createdAt, name, category, price)
				if err != nil {
					http.Error(w, "Ошибка записи в базу данных", http.StatusInternalServerError)
					return
				}

				totalItems++
				totalCategoriesMap[category] = true
				totalPrice += price
			}
		}
	}

	totalCategories := len(totalCategoriesMap)
	response := PostResponse{
		TotalItems:      totalItems,
		TotalCategories: totalCategories,
		TotalPrice:      totalPrice,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Обработчик для GET /api/v0/prices
func handleGetPrices(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, created_at, name, category, price FROM prices")
	if err != nil {
		http.Error(w, "Ошибка чтения из базы данных", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	file, err := os.Create("data.csv")
	if err != nil {
		http.Error(w, "Ошибка создания файла", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"id", "created_at", "name", "category", "price"})
	for rows.Next() {
		var id, createdAt, name, category string
		var price float64
		if err := rows.Scan(&id, &createdAt, &name, &category, &price); err != nil {
			http.Error(w, "Ошибка чтения строки", http.StatusInternalServerError)
			return
		}
		writer.Write([]string{id, createdAt, name, category, fmt.Sprintf("%.2f", price)})
	}

	archive, err := os.Create("data.zip")
	if err != nil {
		http.Error(w, "Ошибка создания архива", http.StatusInternalServerError)
		return
	}
	defer archive.Close()

	zipWriter := zip.NewWriter(archive)
	defer zipWriter.Close()

	csvFile, err := zipWriter.Create("data.csv")
	if err != nil {
		http.Error(w, "Ошибка добавления файла в архив", http.StatusInternalServerError)
		return
	}

	file.Seek(0, 0)
	io.Copy(csvFile, file)

	http.ServeFile(w, r, "data.zip")
}

func main() {
	initDB()
	defer db.Close()

	http.HandleFunc("/api/v0/prices", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			handlePostPrices(w, r)
		case "GET":
			handleGetPrices(w, r)
		default:
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		}
	})

	log.Println("Сервер запущен на порту 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
