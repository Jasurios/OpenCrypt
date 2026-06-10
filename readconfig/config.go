package readconfig

import (
	"os"

	"github.com/joho/godotenv"
)

func Read(key string)string  {
	godotenv.Load("config.env")
	envkey := os.Getenv(key)
	return envkey
}