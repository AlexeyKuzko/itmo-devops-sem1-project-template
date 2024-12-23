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

// Подключение к базе данных
var db *sql.DB

func initDB() {
	var err error
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("База данных недоступна: %v", err)
	}
	log.Println("Успешно подключилсь к базе данных.")
}

func main() {
	initDB()
	defer db.Close()

	http.HandleFunc("/api/v0/prices", pricesHandler)
	log.Println("Запускаем сервер на :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func pricesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		handlePostPrices(w, r)
	case http.MethodGet:
		handleGetPrices(w, r)
	default:
		http.Error(w, "Неизвестный метод", http.StatusMethodNotAllowed)
	}
}

func handlePostPrices(w http.ResponseWriter, r *http.Request) {
	// Распарсить файл CSV
	r.ParseMultipartForm(10 << 20) // Максимальный размер: 10MB
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Ошибка чтения файла", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Сохранить файл во временном файле
	tmpFilePath := "uploaded.zip"
	tmpFile, err := os.Create(tmpFilePath)
	if err != nil {
		http.Error(w, "Ошибка записи файла", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpFilePath)
	defer tmpFile.Close()
	io.Copy(tmpFile, file)

	// Извлечь и обработать CSV из архива
	archive, err := zip.OpenReader(tmpFilePath)
	if err != nil {
		http.Error(w, "Ошибка открытия архива", http.StatusInternalServerError)
		return
	}
	defer archive.Close()

	totalItems, totalCategories, totalPrice := processArchive(archive)

	// Возврат JSON с информацией о внесённых данных
	response := map[string]interface{}{
		"total_items":      totalItems,
		"total_categories": totalCategories,
		"total_price":      totalPrice,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func processArchive(archive *zip.ReadCloser) (int, int, float64) {
	totalItems := 0
	totalCategories := make(map[string]struct{})
	totalPrice := 0.0

	for _, file := range archive.File {
		if strings.HasSuffix(file.Name, ".csv") {
			f, err := file.Open()
			if err != nil {
				log.Printf("Ошибка открытия файла в архиве: %v", err)
				continue
			}
			defer f.Close()

			reader := csv.NewReader(f)
			for {
				record, err := reader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Printf("Ошибка чтения файла CSV: %v", err)
					continue
				}

				productID, _ := strconv.Atoi(record[0])
				createdAt, _ := time.Parse("2006-01-02", record[1])
				name := record[2]
				category := record[3]
				price, _ := strconv.ParseFloat(record[4], 64)

				// Вставить запись в базу данных
				_, err = db.Exec(`
					INSERT INTO prices (product_id, created_at, name, category, price)
					VALUES ($1, $2, $3, $4, $5)
				`, productID, createdAt, name, category, price)
				if err != nil {
					log.Printf("Ошибка добавления записи в БД: %v", err)
					continue
				}

				totalItems++
				totalCategories[category] = struct{}{}
				totalPrice += price
			}
		}
	}
	return totalItems, len(totalCategories), totalPrice
}

func handleGetPrices(w http.ResponseWriter, r *http.Request) {
	// Получить данные из БД
	rows, err := db.Query(`SELECT product_id, created_at, name, category, price FROM prices`)
	if err != nil {
		http.Error(w, "Ошибка при запросе данных из БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Создать CSV файл
	csvFilePath := "data.csv"
	csvFile, err := os.Create(csvFilePath)
	if err != nil {
		http.Error(w, "Не удается создать CSV файл", http.StatusInternalServerError)
		return
	}
	defer os.Remove(csvFilePath)
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)
	defer writer.Flush()

	// Записать данные в CSV
	for rows.Next() {
		var productID int
		var createdAt time.Time
		var name, category string
		var price float64

		err := rows.Scan(&productID, &createdAt, &name, &category, &price)
		if err != nil {
			http.Error(w, "Ошибка считывания данных", http.StatusInternalServerError)
			return
		}

		record := []string{
			strconv.Itoa(productID),
			createdAt.Format("2006-01-02"),
			name,
			category,
			fmt.Sprintf("%.2f", price),
		}
		writer.Write(record)
	}

	// Создать архив
	zipFilePath := "output.zip"
	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		http.Error(w, "Ошибка создания архива", http.StatusInternalServerError)
		return
	}
	defer os.Remove(zipFilePath)
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	csvFileInZip, err := zipWriter.Create("data.csv")
	if err != nil {
		http.Error(w, "Ошибка добавления CSV в zip", http.StatusInternalServerError)
		return
	}
	csvFile.Seek(0, io.SeekStart)
	io.Copy(csvFileInZip, csvFile)

	// Вернуть архив
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=output.zip")
	http.ServeFile(w, r, zipFilePath)
}
