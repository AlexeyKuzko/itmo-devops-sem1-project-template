# Финальный проект 1 семестра

REST API сервис для загрузки и выгрузки данных о ценах.

## Требования к системе

Ubuntu последней стабильной версии + PostgreSQL 15.

## Установка и запуск

1. Установите PostgreSQL (если не установлен):
   ```bash
   sudo apt update
   sudo apt install -y postgresql
   ```

2.	Настройте базу данных:
   ```bash
   psql -U postgres
   CREATE USER validator WITH PASSWORD    'val1dat0r';
   CREATE DATABASE "project-sem-1";
   \c "project-sem-1"
   CREATE TABLE prices (
       id VARCHAR PRIMARY KEY,
       created_at DATE,
       name VARCHAR,
       category VARCHAR,
       price NUMERIC
   );
   ```

3.	Установите зависимости:
   ```bash
   ./scripts/prepare.sh
   ```

4.  Запустите сервер:
   ```bash   
   ./scripts/run.sh
   ```
5.  Запустите тесты:
   ```bash
   ./scripts/tests.sh
   ```

## Тестирование

Директория `sample_data` - это пример директории, которая является разархивированной версией файла `sample_data.zip`

Проверка ручек:
### POST /api/v0/prices
Пример запроса для загрузки данных в формате ZIP:
   ```bash
curl -X POST -F "file=@data.zip" http://localhost:8080/api/v0/prices
   ```

### GET /api/v0/prices
Пример запроса для выгрузки данных в формате ZIP:
   ```bash
curl -X GET http://localhost:8080/api/v0/prices -o response.zip
   ```

## Контакт

В случае вопросов, можно обращаться ко [мне](https://github.com/AlexeyKuzko).
