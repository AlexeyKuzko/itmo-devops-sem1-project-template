#!/bin/bash

# Сразу останавливается, если какая-либо команда возвращает ненулевой код.
set -e

echo "Подготовка базы данных..."

# Проверить наличие PostgreSQL, если нет - установить
if ! command -v psql &> /dev/null; then
  echo "PostgreSQL не найден, пробуем установить..."
  sudo apt-get update
  sudo apt-get install -y postgresql postgresql-client
fi

# Проверить, запущен ли сервис PostgreSQL
if ! sudo service postgresql status &> /dev/null; then
  echo "PostgreSQL не запущен, запускаем..."
  sudo service postgresql start
fi

# Проверим подключение к PostgreSQL
if ! psql -U postgres -c "\\q" &> /dev/null; then
  echo "Ошибка подключения к PostgreSQL, перезапускаем сервис..."

  # Пробуем перезапустить сервис
  sudo service postgresql restart

  # Еще раз проверим подключение
  if ! psql -U postgres -c "\\q" &> /dev/null; then
    echo "Не удалось подключиться к PostgreSQL на перезапуске, нужно проверить конфигурацию."
    exit 2
  fi
fi

# Подключаемся к базе данных
echo "Подготовка базы данных..."
PGUSER="validator"
PGPASSWORD="val1dat0r"
DBNAME="project-sem-1"

# Создать пользователя и базу данных
psql -v ON_ERROR_STOP=1 <<-EOSQL
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