#!/bin/bash

# Прекращать выполнение при ошибках
set -e

echo "Подготовка базы данных..."

# Переменные для подключения
PGHOST="localhost"
PGUSER="validator"
PGPASSWORD="val1dat0r"
DBNAME="project-sem-1"
PORT=5432

# Проверка доступности PostgreSQL
echo "Проверяем доступность PostgreSQL..."
if ! psql -U postgres -h $PGHOST -p $PORT -c "\\q" &> /dev/null; then
  echo "PostgreSQL недоступен на $PGHOST:$PORT. Проверяем окружение GitHub Actions..."
  
  # Проверить, запущен ли контейнер PostgreSQL
  if [ -n "$(which docker)" ] && [ "$(docker ps -q -f name=postgres)" ]; then
    echo "Контейнер PostgreSQL запущен. Проверяем его статус..."
    if ! docker exec postgres pg_isready -U postgres; then
      echo "PostgreSQL в контейнере недоступен. Проверьте конфигурацию Docker."
      exit 1
    fi
  else
    echo "PostgreSQL не найден ни как служба, ни как контейнер. Проверьте окружение."
    exit 2
  fi
else
  echo "PostgreSQL доступен. Продолжаем настройку..."
fi

# Создать пользователя и базу данных
echo "Создаём пользователя и базу данных..."
psql -v ON_ERROR_STOP=1 -h $PGHOST -p $PORT <<-EOSQL
  DO \$\$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_user WHERE usename = '${PGUSER}') THEN
      CREATE USER ${PGUSER} WITH PASSWORD '${PGPASSWORD}';
    END IF;
  END \$\$;

  DO \$\$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_database WHERE datname = '${DBNAME}') THEN
      CREATE DATABASE ${DBNAME} OWNER ${PGUSER};
    END IF;
  END \$\$;
EOSQL

# Создать таблицу
echo "Создаём таблицу..."
psql -U ${PGUSER} -d ${DBNAME} -h $PGHOST -p $PORT -v ON_ERROR_STOP=1 <<-EOSQL
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