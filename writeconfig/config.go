package writeconfig

import (
	"fmt"
	"log"
	"os"

	"OpenCrypt/backup"
	"OpenCrypt/logo"
	"OpenCrypt/readconfig"
	"OpenCrypt/server"
	"OpenCrypt/users"

	"github.com/joho/godotenv"
)

// Check проверяет есть ли конфиг и запускает нужный режим
func Check() {
	_, err := os.Stat("config.env")
	if os.IsNotExist(err) {
		log.Println("Первый запуск — создаём конфиг")
		Firsttime()
	} else {
		server.Start()
	}
}

// CheckConfig открывает меню конфига
// Вызывается через: go run . config
func CheckConfig() {
	_, err := os.Stat("config.env")
	if os.IsNotExist(err) {
		fmt.Println("Сначала запустите сервер для первоначальной настройки")
		os.Exit(1)
	}
	showConfigMenu()
}

// Firsttime — первоначальная настройка при первом запуске
func Firsttime() {
	var userspath, port, backupDir, backupInterval string

	fmt.Printf("  Путь до папки пользователей: ")
	fmt.Scanln(&userspath)
	fmt.Printf("  Порт сервера (например 2022): ")
	fmt.Scanln(&port)
	fmt.Printf("  Папка для бэкапов: ")
	fmt.Scanln(&backupDir)
	fmt.Printf("  Интервал бэкапа (minute/day/month): ")
	fmt.Scanln(&backupInterval)

	// Создаём нужные папки
	os.MkdirAll(userspath, 0755)
	if backupDir != "" {
		os.MkdirAll(backupDir, 0755)
	}

	env := map[string]string{
		"USERSFILES":     userspath,
		"PORT":           port,
		"BACKUPDIR":      backupDir,
		"BACKUPINTERVAL": backupInterval,
	}
	godotenv.Write(env, "config.env")

	fmt.Println("\n  Создаём первого пользователя:")
	users.Newuser()

	fmt.Println("\n  Конфиг создан! Запускаем сервер...")
	server.Start()
}

// showConfigMenu — меню конфига
func showConfigMenu() {
	for {
		logo.Printconfig()

		fmt.Println("\n  Что хотите изменить?\n")
		fmt.Println("  [1] Настройки сервера")
		fmt.Println("  [2] Пользователи")
		fmt.Println("  [3] Настройки бэкапа")
		fmt.Println("  [4] Сделать бэкап сейчас")
		fmt.Println("  [0] Выход")
		fmt.Print("\n  Выбор: ")

		var choice int
		fmt.Scanln(&choice)

		switch choice {
		case 1:
			editServerConfig()
		case 2:
			users.Main()
		case 3:
			editBackupConfig()
		case 4:
			doBackupNow()
		case 0:
			return
		}
	}
}

// editServerConfig — меню изменения настроек сервера
func editServerConfig() {
	fmt.Println("\n  Текущие настройки:")
	fmt.Printf("  PORT:       %s\n", readconfig.Read("PORT"))
	fmt.Printf("  USERSFILES: %s\n", readconfig.Read("USERSFILES"))

	var port, userspath string
	fmt.Printf("\n  Новый порт (Enter = оставить): ")
	fmt.Scanln(&port)
	fmt.Printf("  Новый путь до файлов (Enter = оставить): ")
	fmt.Scanln(&userspath)

	env := map[string]string{
		"USERSFILES":     readconfig.Read("USERSFILES"),
		"PORT":           readconfig.Read("PORT"),
		"BACKUPDIR":      readconfig.Read("BACKUPDIR"),
		"BACKUPINTERVAL": readconfig.Read("BACKUPINTERVAL"),
	}
	if port != "" {
		env["PORT"] = port
	}
	if userspath != "" {
		env["USERSFILES"] = userspath
	}
	godotenv.Write(env, "config.env")
	fmt.Println("  ✓ Настройки сохранены. Перезапустите сервер.")
}

// editBackupConfig — меню изменения настроек бэкапа
func editBackupConfig() {
	fmt.Println("\n  Текущие настройки бэкапа:")
	fmt.Printf("  BACKUPDIR:      %s\n", readconfig.Read("BACKUPDIR"))
	fmt.Printf("  BACKUPINTERVAL: %s\n", readconfig.Read("BACKUPINTERVAL"))

	var backupDir, backupInterval string
	fmt.Printf("\n  Новая папка бэкапов (Enter = оставить): ")
	fmt.Scanln(&backupDir)
	fmt.Printf("  Новый интервал (minute/day/month, Enter = оставить): ")
	fmt.Scanln(&backupInterval)

	env := map[string]string{
		"USERSFILES":     readconfig.Read("USERSFILES"),
		"PORT":           readconfig.Read("PORT"),
		"BACKUPDIR":      readconfig.Read("BACKUPDIR"),
		"BACKUPINTERVAL": readconfig.Read("BACKUPINTERVAL"),
	}
	if backupDir != "" {
		env["BACKUPDIR"] = backupDir
		os.MkdirAll(backupDir, 0755)
	}
	if backupInterval != "" {
		if backupInterval == "minute" || backupInterval == "day" || backupInterval == "month" {
			env["BACKUPINTERVAL"] = backupInterval
		} else {
			fmt.Println("  Неверный интервал. Доступны: minute, day, month")
			return
		}
	}
	godotenv.Write(env, "config.env")
	fmt.Println("  ✓ Настройки бэкапа сохранены. Перезапустите сервер.")
}

// doBackupNow делает бэкап прямо сейчас
func doBackupNow() {
	sourceDir := readconfig.Read("USERSFILES")

	if sourceDir == "" {
		fmt.Println("  Путь до файлов не настроен.")
		return
	}

	fmt.Println("  Делаю бэкап...")
	if err := backup.Do(sourceDir); err != nil {
		fmt.Println("  Ошибка бэкапа:", err)
	} else {
		fmt.Println("  ✓ Бэкап успешно создан")
	}
}
