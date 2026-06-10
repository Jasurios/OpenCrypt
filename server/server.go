package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"OpenCrypt/backup"
	"OpenCrypt/readconfig"
	"OpenCrypt/users"
	"OpenCrypt/vfs"
)

var (
	watchedDirs = make(map[string]bool)
	watchedMu   sync.Mutex
)

func Start() {
	port := readconfig.Read("PORT")
	baseDir := readconfig.Read("USERSFILES")

	userList, err := users.Load()
	if err != nil {
		log.Fatal("Не могу загрузить пользователей:", err)
	}

	privateBytes, err := os.ReadFile("keys/server_key")
	if err != nil {
		log.Fatal("Не могу прочитать ключ сервера:", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Не могу парсить ключ:", err)
	}

	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			user := users.Find(c.User(), userList)
			if user == nil || !users.VerifyPassword(user, string(pass)) {
				return nil, fmt.Errorf("неверный логин или пароль")
			}

			var rootDir string
			if user.Super == "yes" {
				rootDir = baseDir
			} else {
				rootDir = filepath.Join(baseDir, user.Name)
			}

			os.MkdirAll(rootDir, 0755)

			watchedMu.Lock()
			if !watchedDirs[rootDir] {
				watchedDirs[rootDir] = true
				go backup.StartWatcher(rootDir)
			}
			watchedMu.Unlock()

			return &ssh.Permissions{
				Extensions: map[string]string{
					"root_dir": rootDir,
				},
			}, nil
		},
	}
	config.AddHostKey(private)

	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", port))
	if err != nil {
		log.Fatal("Не могу запустить сервер:", err)
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("Завершение работы...")
		listener.Close()
		os.Exit(0)
	}()

	log.Printf("OpenCrypt запущен на порту %s", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Сервер остановлен")
			return
		}
		go handleConnection(conn, config)
	}
}

func handleConnection(conn net.Conn, config *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		log.Println("SSH handshake ошибка:", err)
		return
	}
	defer sshConn.Close()

	rootDir := sshConn.Permissions.Extensions["root_dir"]
	log.Printf("Подключился: %s -> %s", sshConn.User(), rootDir)

	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			newChan.Reject(ssh.UnknownChannelType, "неизвестный тип")
			continue
		}

		channel, requests, err := newChan.Accept()
		if err != nil {
			log.Println("Ошибка канала:", err)
			continue
		}

		go func(in <-chan *ssh.Request) {
			for req := range in {
				ok := req.Type == "subsystem" && string(req.Payload[4:]) == "sftp"
				req.Reply(ok, nil)
			}
		}(requests)

		handlers := sftp.NewRequestServer(channel, newFS(rootDir))
		if err := handlers.Serve(); err != nil && err != io.EOF {
			log.Println("SFTP serve ошибка:", err)
		}
	}
}

func newFS(root string) sftp.Handlers {
	return vfs.New(root)
}