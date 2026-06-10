package main

import (
	"fmt"
	"os"

	"OpenCrypt/logo"
	"OpenCrypt/writeconfig"
)

func main() {
	// Два режима запуска:
	// go run .         — запускает сервер
	// go run . config  — открывает меню конфига
	if len(os.Args) > 1 && os.Args[1] == "config" {
		logo.Printconfig()
		writeconfig.CheckConfig()
	} else {
		logo.Printmain()
		fmt.Println("  Для настройки запустите: opencrypt config")
		writeconfig.Check()
	}
}
