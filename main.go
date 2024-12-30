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
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName,
	)
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
}

// Обработчик для POST /api/v0/prices
func handlePostPrices(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		log.Printf("Ошибка загрузки файла: %v", err)
		http.Error(w, "Не удалось загрузить файл", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tempFile, err := os.CreateTemp("", "uploaded-*.zip")
	if err != nil {
		log.Printf("Ошибка сохранения файла: %v", err)
		http.Error(w, "Ошибка сохранения файла", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())

	if _, err := io.Copy(tempFile, file); err != nil {
		log.Printf("Ошибка копирования файла: %v", err)
		http.Error(w, "Ошибка копирования файла", http.StatusInternalServerError)
		return
	}

	zipReader, err := zip.OpenReader(tempFile.Name())
	if err != nil {
		log.Printf("Ошибка открытия архива: %v", err)
		http.Error(w, "Ошибка чтения архива", http.StatusBadRequest)
		return
	}
	defer zipReader.Close()

	// Читаем сразу все CSV-файлы из архива
	var csvRecords [][]string
	for _, f := range zipReader.File {
		if strings.HasSuffix(f.Name, ".csv") {
			csvFile, err := f.Open()
			if err != nil {
				log.Printf("Ошибка открытия CSV: %v", err)
				http.Error(w, "Ошибка открытия CSV", http.StatusInternalServerError)
				return
			}
			defer csvFile.Close()

			reader := csv.NewReader(csvFile)
			records, err := reader.ReadAll()
			if err != nil {
				log.Printf("Ошибка чтения CSV: %v", err)
				http.Error(w, "Ошибка чтения CSV", http.StatusInternalServerError)
				return
			}
			csvRecords = append(csvRecords, records...)
		}
	}

	// Открываем транзакцию один раз на всю пачку
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Ошибка начала транзакции: %v", err)
		http.Error(w, "Ошибка начала транзакции", http.StatusInternalServerError)
		return
	}
	// Если что-то пойдёт не так, откатим все изменения
	defer tx.Rollback()

	for _, record := range csvRecords {
		// Проверка и обработка данных
		if len(record) < 5 {
			// Недостаточно данных для вставки — просто пропускаем запись
			continue
		}

		id := strings.TrimSpace(record[0])
		created_at := strings.TrimSpace(record[1])
		name := strings.TrimSpace(record[2])
		category := strings.TrimSpace(record[3])
		price, err := strconv.ParseFloat(strings.TrimSpace(record[4]), 64)
		if err != nil {
			// Некорректная цена — пропускаем запись
			log.Printf("Ошибка преобразования цены '%s': %v", record[4], err)
			continue
		}

		// Проверка формата даты
		if _, err := time.Parse("2006-01-02", created_at); err != nil {
			// Некорректный формат даты — пропускаем запись
			log.Printf("Некорректный формат даты '%s': %v", created_at, err)
			continue
		}

		// Вставка в базу в рамках одной транзакции
		_, err = tx.Exec(`
            INSERT INTO prices (id, created_at, name, category, price)
            VALUES ($1, $2, $3, $4, $5)
        `, id, created_at, name, category, price)
		if err != nil {
			log.Printf("Ошибка записи в базу данных для ID '%s': %v", id, err)
			// Если не получается вставить конкретную строку, пропускаем её
			continue
		}
	}

	// После того, как все данные вставлены, делаем запросы для подсчёта итогов
	var totalItems int
	var totalCategories int
	var totalPrice float64

	err = tx.QueryRow("SELECT COUNT(*) FROM prices").Scan(&totalItems)
	if err != nil {
		log.Printf("Ошибка получения total_items: %v", err)
		http.Error(w, "Ошибка чтения из базы данных", http.StatusInternalServerError)
		return
	}

	err = tx.QueryRow("SELECT COUNT(DISTINCT category) FROM prices").Scan(&totalCategories)
	if err != nil {
		log.Printf("Ошибка получения total_categories: %v", err)
		http.Error(w, "Ошибка чтения из базы данных", http.StatusInternalServerError)
		return
	}

	// Чтобы избежать NULL (если вдруг база пустая), используем COALESCE
	err = tx.QueryRow("SELECT COALESCE(SUM(price), 0) FROM prices").Scan(&totalPrice)
	if err != nil {
		log.Printf("Ошибка получения total_price: %v", err)
		http.Error(w, "Ошибка чтения из базы данных", http.StatusInternalServerError)
		return
	}

	// Завершаем транзакцию (коммитим все вставленные данные и подсчёты)
	if err := tx.Commit(); err != nil {
		log.Printf("Ошибка подтверждения транзакции: %v", err)
		http.Error(w, "Ошибка подтверждения транзакции", http.StatusInternalServerError)
		return
	}

	// Формируем и отправляем ответ
	response := map[string]interface{}{
		"total_items":      totalItems,
		"total_categories": totalCategories,
		"total_price":      totalPrice,
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

	// Создаем временную директорию для файлов
	tempDir := os.TempDir()
	csvFilePath := filepath.Join(tempDir, "data.csv")
	zipFilePath := filepath.Join(tempDir, "data.zip")

	// Создаём CSV-файл
	file, err := os.Create(csvFilePath)
	if err != nil {
		http.Error(w, "Ошибка создания файла CSV", http.StatusInternalServerError)
		return
	}
	defer os.Remove(csvFilePath)
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"id", "created_at", "name", "category", "price"})
	for rows.Next() {
		var id, created_at, name, category string
		var price float64
		if err := rows.Scan(&id, &created_at, &name, &category, &price); err != nil {
			http.Error(w, "Ошибка чтения строки", http.StatusInternalServerError)
			return
		}
		writer.Write([]string{id, created_at, name, category, fmt.Sprintf("%.2f", price)})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		http.Error(w, "Ошибка записи CSV", http.StatusInternalServerError)
		return
	}

	// Создаём архив
	archive, err := os.Create(zipFilePath)
	if err != nil {
		http.Error(w, "Ошибка создания архива", http.StatusInternalServerError)
		return
	}
	defer os.Remove(zipFilePath)
	defer archive.Close()

	zipWriter := zip.NewWriter(archive)
	defer zipWriter.Close()

	csvFile, err := zipWriter.Create("data.csv")
	if err != nil {
		http.Error(w, "Ошибка добавления файла в архив", http.StatusInternalServerError)
		return
	}

	file.Seek(0, 0)
	if _, err := io.Copy(csvFile, file); err != nil {
		http.Error(w, "Ошибка копирования данных в архив", http.StatusInternalServerError)
		return
	}

	// Проверяем, что завершили запись в архив
	if err := zipWriter.Close(); err != nil {
		http.Error(w, "Ошибка завершения архива", http.StatusInternalServerError)
		return
	}

	// Отдаём файл пользователю
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"data.zip\"")
	http.ServeFile(w, r, zipFilePath)
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

	log.Println("Сервер успешно запущен на порту 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
