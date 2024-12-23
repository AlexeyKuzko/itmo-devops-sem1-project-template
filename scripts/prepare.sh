#!/bin/bash

# Сразу останавливается, если какая-либо команда возвращает ненулевой код.
set -e

echo "Подготовка базы данных..."
sudo apt update
sudo apt install -y postgresql postgresql-contrib

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

# Проверим подключение к PostgreSQL через сокет и по сети
socket_error=false
if ! psql -U postgres -c "\\q" &> /dev/null; then
  echo "Ошибка подключения к PostgreSQL через сокет. Пробуем подключиться по сети..."
  socket_error=true
fi

# Если сокет недоступен, проверяем конфигурацию postgresql.conf
if $socket_error; then
  echo "Проверяем конфигурацию postgresql.conf..."
  config_path=$(sudo find /etc -name postgresql.conf 2>/dev/null | head -n 1)

  if [ -z "$config_path" ]; then
    echo "Не удалось найти файл postgresql.conf. Проверьте установку PostgreSQL."
    exit 2
  fi

  echo "Файл конфигурации: $config_path"
  sudo grep "unix_socket_directories" "$config_path" || echo "unix_socket_directories параметр не найден. Добавьте его и укажите корректный путь."
  echo "Перезапускаем PostgreSQL..."
  sudo service postgresql restart

  # Повторная проверка подключения
  if ! psql -U postgres -c "\\q" &> /dev/null; then
    echo "Не удалось подключиться к PostgreSQL после исправлений. Проверьте конфигурацию вручную."
    exit 2
  fi
fi

# Подключаемся к базе данных
echo "Настройка базы данных..."
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