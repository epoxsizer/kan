# kan

[![Проверка](https://github.com/epoxsizer/kan/actions/workflows/ci.yml/badge.svg)](https://github.com/epoxsizer/kan/actions/workflows/ci.yml)
[![Релиз](https://github.com/epoxsizer/kan/actions/workflows/release.yml/badge.svg)](https://github.com/epoxsizer/kan/actions/workflows/release.yml)
[![Лицензия: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

`kan` - локальный трекер задач с интерфейсом в терминале. Он хранит данные в
SQLite и не требует сервера, регистрации или подключения к интернету.

Структура данных:

```text
Проект -> Доска -> Колонка -> Карточка
```

В приложении есть полнотекстовый поиск, теги, приоритеты, сроки, комментарии,
чек-листы, дополнительные поля, связи между карточками, JSON-импорт и экспорт,
а также автоматические резервные копии.

## Интерфейс

![Экран доски kan с цветными колонками и карточками](docs/kan-board.svg)

Интерфейс адаптируется к размеру терминала. Активная колонка и выбранная
карточка выделяются цветом, а доступные сочетания клавиш показаны в нижней
строке.

## Быстрый запуск из исходников

Требуется Go 1.22 или новее.

```sh
git clone https://github.com/epoxsizer/kan.git
cd kan
make build
./bin/kan seed
./bin/kan
```

Команда `seed` создаёт демонстрационный проект, доску, колонки и карточки. Она
идемпотентна, поэтому её можно выполнить повторно без дублирования данных.

Для запуска без демонстрационных данных:

```sh
make build
./bin/kan migrate
./bin/kan
```

## Установка готового релиза

Скачайте архив для Linux, macOS или Windows из раздела
[релизов GitHub](https://github.com/epoxsizer/kan/releases),
проверьте файл по `checksums.txt` и поместите бинарник `kan` в каталог из
переменной `PATH`.

Также приложение можно установить через Go:

```sh
go install github.com/epoxsizer/kan/cmd/kan@latest
```

## Основные клавиши

| Клавиша | Действие |
|---|---|
| `h j k l`, стрелки | Навигация |
| `Enter`, `e` | Открыть или изменить объект |
| `a` | Добавить карточку или объект текущего экрана |
| `D` | Удалить с подтверждением |
| `d` | Показать краткую информацию |
| `H`, `L` | Переместить карточку в соседнюю колонку |
| `Shift-Tab`, `Tab` | Переместить карточку между колонками |
| `J`, `K` | Изменить порядок карточек |
| `/` | Поиск по текущей доске |
| `:` | Командная строка и общий нечёткий поиск |
| `:layout table` | Показать проекты и доски таблицей |
| `:layout cards` | Показать проекты и доски списком карточек |
| `?` | Полная справка |
| `Esc` | Назад или отмена |
| `q`, `Ctrl-C` | Выход |

В формах используйте `Tab` для перехода между полями и `Ctrl-S` для сохранения.

## Команды CLI

Приложение можно использовать без терминального интерфейса из командных
сценариев, автоматических проверок и агентов. Успешные команды управления
данными возвращают JSON в стандартный вывод. Названия и заголовки с пробелами
необходимо заключать в кавычки.

```sh
kan project list
kan project create --name "Новый проект" --comment "Описание проекта"
kan board list --project PROJECT_ID
kan board create --project PROJECT_ID --name "Разработка"
kan column create --board BOARD_ID --name "В работе"
kan card create --board BOARD_ID --column COLUMN_ID --title "Подготовить релиз"
kan card search --board BOARD_ID --query "релиз"
kan card update CARD_ID --priority high
kan card delete CARD_ID --yes
```

Полный список параметров:

```sh
kan --help
kan card --help
kan card create --help
```

Не запускайте команды записи одновременно с терминальным интерфейсом: блокировка
базы защищает её от параллельных изменений.

## Импорт, экспорт и резервные копии

```sh
kan backup
kan backup before-upgrade
kan export --out kan-export.json
kan import kan-export.json
```

Ручные и автоматические копии сохраняются в каталоге `backup/` относительно
текущего рабочего каталога. Во время работы терминального интерфейса
автоматическая копия создаётся примерно раз в шесть часов.

## Пути данных

По умолчанию используются XDG-каталоги:

- конфигурация: `${XDG_CONFIG_HOME:-~/.config}/kan/config.toml`;
- база: `${XDG_DATA_HOME:-~/.local/share}/kan/kan.db`;
- журнал: `${XDG_STATE_HOME:-~/.local/state}/kan/kan.log`.

Пути можно изменить флагами `--config`, `--db`, `--log` или переменными
`KAN_CONFIG`, `KAN_DB`, `KAN_LOG`.

Пример конфигурации находится в
[`docs/config.example.toml`](docs/config.example.toml).

## Разработка

```sh
make fmt       # форматирование
make test      # тесты
make check     # форматирование, go vet, тесты и сборка
make build     # bin/kan
make cross-build # Linux, macOS и Windows для amd64/arm64
```

Сборка выполняется с `CGO_ENABLED=0` и использует SQLite-драйвер на чистом Go.

Проект распространяется по лицензии [MIT](LICENSE). Правила участия описаны в
[CONTRIBUTING.md](CONTRIBUTING.md), сообщения об уязвимостях - в
[SECURITY.md](SECURITY.md).
