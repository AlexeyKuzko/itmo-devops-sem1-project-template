#!/bin/bash

# Сразу останавливается, если какая-либо команда возвращает ненулевой код.
set -e

# Проверить наличие PostgreSQL, если нет - установить
if ! command -v psql &> /dev/null; then
  echo "PostgreSQL не найден, пробуем установить..."
  sudo apt-get update
  sudo apt-get install -y postgresql-client
fi

# Подключиться к базе данных
echo "Подготовка базы данных..."
PGUSER="validator"
PGPASSWORD="val1dat0r"
DBNAME="project-sem-1"

# Создать пользователя и базу данных
psql -v ON_ERROR_STOP=1 <<-EOSQL
  CREATE USER ${PGUSER} WITH PASSWORD '${PGPASSWORD}';
  CREATE DATABASE ${DBNAME} OWNER ${PGUSER};
EOSQL

# Создать таблицу
psql -U ${PGUSER} -d ${DBNAME} -v ON_ERROR_STOP=1 <<-EOSQL
  CREATE TABLE IF NOT EXISTS prices (
    id SERIAL PRIMARY KEY,
    product_id INT NOT NULL,
    created_at DATE NOT NULL,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    price NUMERIC(10, 2) NOT NULL
  );
EOSQL

echo "Подготовка базы данных успешно завершена."