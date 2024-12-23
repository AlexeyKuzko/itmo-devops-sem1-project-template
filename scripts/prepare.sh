#!/bin/bash

# Прекращать выполнение при ошибках
set -e

echo "Подготовка базы данных..."

# Переменные для подключения
PGHOST="localhost"
PGPORT=5432
PGUSER="validator"
PGPASSWORD="val1dat0r" 
DBUSER="validator"
DBPASS="val1dat0r"
DBNAME="project-sem-1"

export PGPASSWORD

# Проверка доступности PostgreSQL
echo "Проверяем доступность PostgreSQL..."
if ! psql -U $PGUSER -h $PGHOST -p $PGPORT -c "\\q" &> /dev/null; then
  echo "PostgreSQL недоступен на $PGHOST:$PGPORT. Проверяем окружение GitHub Actions..."
  
  # Определяем контейнер PostgreSQL
  POSTGRES_CONTAINER=$(docker ps --filter "ancestor=postgres:15" --format "{{.ID}}")
  
  if [ -z "$POSTGRES_CONTAINER" ]; then
    echo "Контейнер PostgreSQL не найден. Проверьте конфигурацию Docker."
    exit 2
  fi

  echo "Контейнер PostgreSQL найден: $POSTGRES_CONTAINER. Проверяем статус..."
  if ! docker exec $POSTGRES_CONTAINER pg_isready -U postgres &> /dev/null; then
    echo "PostgreSQL в контейнере недоступен. Проверьте конфигурацию Docker."
    exit 3
  fi

  # Обновляем хост на 127.0.0.1
  PGHOST="127.0.0.1"
  echo "Подключение настроено на контейнер PostgreSQL."
fi

# Создать пользователя и базу данных
echo "Создаём пользователя и базу данных..."
psql -U $PGUSER -h $PGHOST -p $PGPORT <<-EOSQL
  DO \$\$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_user WHERE usename = '${DBUSER}') THEN
      CREATE USER ${DBUSER} WITH PASSWORD '${DBPASS}';
    END IF;
  END \$\$;

  DO \$\$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_database WHERE datname = '${DBNAME}') THEN
      CREATE DATABASE ${DBNAME} OWNER ${DBUSER};
    END IF;
  END \$\$;
EOSQL

# Создать таблицу
echo "Создаём таблицу..."
psql -U ${DBUSER} -h $PGHOST -p $PGPORT -d ${DBNAME} <<-EOSQL
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