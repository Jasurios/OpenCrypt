package users

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/argon2"
)

// User — структура пользователя
// MasterKey хранится зашифрованным паролем пользователя
// Сам мастер ключ никогда не хранится в открытом виде
type User struct {
	Name            string `json:"name"`
	Password        string `json:"password"`         // хеш пароля через argon2
	Super           string `json:"super"`            // yes/no — супер пользователь
	MasterKey       string `json:"master_key"`       // мастер ключ зашифрованный паролем (hex)
	MasterKeySalt   string `json:"master_key_salt"`  // соль для шифрования мастер ключа (hex)
	PasswordSalt    string `json:"password_salt"`    // соль для хеша пароля (hex)
}

// Main — главное меню управления пользователями
// Вызывается из режима config
func Main() {
	var num int

	fmt.Println("\n  Управление пользователями\n")
	fmt.Println("  1. Создать пользователя")
	fmt.Println("  2. Изменить пароль")
	fmt.Println("  3. Изменить права (super)")
	fmt.Println("  4. Удалить пользователя")
	fmt.Println("  0. Выйти")
	fmt.Printf("\n  Выбор: ")
	fmt.Scanln(&num)

	switch num {
	case 1:
		Newuser()
	case 2:
		changeuser()
	case 3:
		changesuper()
	case 4:
		deliteuser()
	}
}

// Newuser создаёт нового пользователя
// Генерирует мастер ключ и шифрует его паролем пользователя
func Newuser() {
	var username, password, super string

	fmt.Printf("  Имя пользователя: ")
	fmt.Scanln(&username)
	fmt.Printf("  Пароль: ")
	fmt.Scanln(&password)
	fmt.Printf("  Супер пользователь? (yes/no): ")
	fmt.Scanln(&super)
	if super != "yes" {
		super = "no"
	}

	// Генерируем соль для хеша пароля
	passwordSalt := make([]byte, 16)
	rand.Read(passwordSalt)

	// Хешируем пароль через argon2id — надёжный алгоритм для паролей
	// Параметры: time=1, memory=64MB, threads=4, keyLen=32
	passwordHash := argon2.IDKey([]byte(password), passwordSalt, 1, 64*1024, 4, 32)

	// Генерируем случайный мастер ключ (32 байта = AES-256)
	// Этот ключ будет шифровать все архивы пользователя
	masterKey := make([]byte, 32)
	rand.Read(masterKey)

	// Генерируем соль для шифрования мастер ключа
	masterKeySalt := make([]byte, 16)
	rand.Read(masterKeySalt)

	// Из пароля выводим ключ для шифрования мастер ключа
	// Используем argon2 с другой солью чтобы ключ был уникальным
	encKey := argon2.IDKey([]byte(password), masterKeySalt, 1, 64*1024, 4, 32)

	// Шифруем мастер ключ через AES-256-GCM
	encryptedMasterKey, err := encryptAES(masterKey, encKey)
	if err != nil {
		fmt.Println("  Ошибка шифрования мастер ключа:", err)
		return
	}

	user := User{
		Name:          username,
		Password:      hex.EncodeToString(passwordHash),
		Super:         super,
		MasterKey:     hex.EncodeToString(encryptedMasterKey),
		MasterKeySalt: hex.EncodeToString(masterKeySalt),
		PasswordSalt:  hex.EncodeToString(passwordSalt),
	}

	// Загружаем существующих пользователей и добавляем нового
	fileData, _ := os.ReadFile("users.json")
	var users []User
	json.Unmarshal(fileData, &users)
	users = append(users, user)

	jsondata, _ := json.MarshalIndent(users, "", "  ")
	os.WriteFile("users.json", jsondata, 0644)

	fmt.Printf("  ✓ Пользователь '%s' создан\n", username)
}

// changeuser меняет пароль пользователя
// При смене пароля нужно перешифровать мастер ключ новым паролем
func changeuser() {
	var username, oldPassword, newPassword string

	users, err := Load()
	if err != nil {
		fmt.Println("  Ошибка загрузки пользователей:", err)
		return
	}

	// Показываем список пользователей
	fmt.Println("\n  Пользователи:")
	for _, u := range users {
		fmt.Printf("  - %s\n", u.Name)
	}

	fmt.Printf("\n  Имя пользователя: ")
	fmt.Scanln(&username)
	fmt.Printf("  Старый пароль: ")
	fmt.Scanln(&oldPassword)
	fmt.Printf("  Новый пароль: ")
	fmt.Scanln(&newPassword)

	found := false
	for i, u := range users {
		if u.Name != username {
			continue
		}
		found = true

		// Проверяем старый пароль
		oldSalt, _ := hex.DecodeString(u.PasswordSalt)
		oldHash := argon2.IDKey([]byte(oldPassword), oldSalt, 1, 64*1024, 4, 32)
		if hex.EncodeToString(oldHash) != u.Password {
			fmt.Println("  Неверный старый пароль")
			return
		}

		// Расшифровываем мастер ключ старым паролем
		masterKeySalt, _ := hex.DecodeString(u.MasterKeySalt)
		oldEncKey := argon2.IDKey([]byte(oldPassword), masterKeySalt, 1, 64*1024, 4, 32)
		encryptedMasterKey, _ := hex.DecodeString(u.MasterKey)
		masterKey, err := decryptAES(encryptedMasterKey, oldEncKey)
		if err != nil {
			fmt.Println("  Ошибка расшифровки мастер ключа:", err)
			return
		}

		// Генерируем новые соли
		newPasswordSalt := make([]byte, 16)
		rand.Read(newPasswordSalt)
		newMasterKeySalt := make([]byte, 16)
		rand.Read(newMasterKeySalt)

		// Хешируем новый пароль
		newPasswordHash := argon2.IDKey([]byte(newPassword), newPasswordSalt, 1, 64*1024, 4, 32)

		// Перешифровываем мастер ключ новым паролем
		newEncKey := argon2.IDKey([]byte(newPassword), newMasterKeySalt, 1, 64*1024, 4, 32)
		newEncryptedMasterKey, err := encryptAES(masterKey, newEncKey)
		if err != nil {
			fmt.Println("  Ошибка перешифровки мастер ключа:", err)
			return
		}

		users[i].Password = hex.EncodeToString(newPasswordHash)
		users[i].PasswordSalt = hex.EncodeToString(newPasswordSalt)
		users[i].MasterKey = hex.EncodeToString(newEncryptedMasterKey)
		users[i].MasterKeySalt = hex.EncodeToString(newMasterKeySalt)

		break
	}

	if !found {
		fmt.Printf("  Пользователь '%s' не найден\n", username)
		return
	}

	jsondata, _ := json.MarshalIndent(users, "", "  ")
	os.WriteFile("users.json", jsondata, 0644)
	fmt.Printf("  ✓ Пароль пользователя '%s' изменён\n", username)
}

// changesuper меняет права пользователя
func changesuper() {
	var username, super string

	users, err := Load()
	if err != nil {
		fmt.Println("  Ошибка загрузки пользователей:", err)
		return
	}

	fmt.Println("\n  Пользователи:")
	for _, u := range users {
		fmt.Printf("  - %s [super: %s]\n", u.Name, u.Super)
	}

	fmt.Printf("\n  Имя пользователя: ")
	fmt.Scanln(&username)
	fmt.Printf("  Сделать супер? (yes/no): ")
	fmt.Scanln(&super)
	if super != "yes" {
		super = "no"
	}

	found := false
	for i, u := range users {
		if u.Name == username {
			users[i].Super = super
			found = true
			break
		}
	}

	if !found {
		fmt.Printf("  Пользователь '%s' не найден\n", username)
		return
	}

	jsondata, _ := json.MarshalIndent(users, "", "  ")
	os.WriteFile("users.json", jsondata, 0644)
	fmt.Printf("  ✓ Права пользователя '%s' изменены\n", username)
}

// deliteuser удаляет пользователя
func deliteuser() {
	var username string

	users, err := Load()
	if err != nil {
		fmt.Println("  Ошибка загрузки пользователей:", err)
		return
	}

	fmt.Println("\n  Пользователи:")
	for _, u := range users {
		fmt.Printf("  - %s\n", u.Name)
	}

	fmt.Printf("\n  Имя пользователя для удаления: ")
	fmt.Scanln(&username)

	newUsers := make([]User, 0, len(users))
	found := false
	for _, u := range users {
		if u.Name == username {
			found = true
			continue
		}
		newUsers = append(newUsers, u)
	}

	if !found {
		fmt.Printf("  Пользователь '%s' не найден\n", username)
		return
	}

	jsondata, _ := json.MarshalIndent(newUsers, "", "  ")
	os.WriteFile("users.json", jsondata, 0644)
	fmt.Printf("  ✓ Пользователь '%s' удалён\n", username)
}

// Load загружает список пользователей из users.json
func Load() ([]User, error) {
	data, err := os.ReadFile("users.json")
	if err != nil {
		return nil, err
	}
	var users []User
	err = json.Unmarshal(data, &users)
	return users, err
}

// Find ищет пользователя по имени
func Find(username string, list []User) *User {
	for _, u := range list {
		if u.Name == username {
			return &u
		}
	}
	return nil
}

// VerifyPassword проверяет пароль пользователя
func VerifyPassword(user *User, password string) bool {
	salt, err := hex.DecodeString(user.PasswordSalt)
	if err != nil {
		return false
	}
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return hex.EncodeToString(hash) == user.Password
}

// GetMasterKey расшифровывает и возвращает мастер ключ пользователя
// Вызывается при подключении — ключ нужен для шифрования/расшифровки архивов
func GetMasterKey(user *User, password string) ([]byte, error) {
	masterKeySalt, err := hex.DecodeString(user.MasterKeySalt)
	if err != nil {
		return nil, fmt.Errorf("ошибка декодирования соли: %w", err)
	}

	// Выводим ключ шифрования из пароля
	encKey := argon2.IDKey([]byte(password), masterKeySalt, 1, 64*1024, 4, 32)

	// Расшифровываем мастер ключ
	encryptedMasterKey, err := hex.DecodeString(user.MasterKey)
	if err != nil {
		return nil, fmt.Errorf("ошибка декодирования мастер ключа: %w", err)
	}

	masterKey, err := decryptAES(encryptedMasterKey, encKey)
	if err != nil {
		return nil, fmt.Errorf("ошибка расшифровки мастер ключа: %w", err)
	}

	return masterKey, nil
}

// encryptAES шифрует данные через AES-256-GCM
// GCM обеспечивает и шифрование и аутентификацию данных
func encryptAES(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Генерируем случайный nonce — уникален для каждого шифрования
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Шифруем: nonce + зашифрованные данные
	encrypted := gcm.Seal(nonce, nonce, data, nil)
	return encrypted, nil
}

// decryptAES расшифровывает данные через AES-256-GCM
func decryptAES(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("данные слишком короткие")
	}

	// Разделяем nonce и зашифрованные данные
	nonce, encrypted := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, encrypted, nil)
}
