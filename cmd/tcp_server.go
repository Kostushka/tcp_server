package main

import (
	"html/template"
	"log"
	"net"
	"os"

	"github.com/Kostushka/tcp_server/internal/config"
	"github.com/Kostushka/tcp_server/internal/connection"
	mlog "github.com/Kostushka/tcp_server/internal/log"
)

func main() {
	// получить данные для конфигурации сервера
	configData, err := config.NewConfigData()
	if err != nil {
		log.Fatalf("сервер не может быть запущен: %v", err)
	}

	// создать логеры
	err = mlog.New(configData.Log())
	if err != nil {
		log.Fatalf("сервер не может быть запущен: %v", err)
	}

	// парсим шаблон для отображения имен файлов
	templ, err := os.ReadFile(configData.FileTemplate())
	if err != nil {
		log.Fatalf("сервер не может быть запущен: %v", err)
	}
	// используем шаблон для отображения имен файлов
	t, err := template.New("index").Parse(string(templ))
	if err != nil {
		log.Fatalf("сервер не может быть запущен: %v", err)
	}

	// объявляем структуру с данными будущего сервера
	laddr := net.TCPAddr{
		IP:   configData.ListenAddress(),
		Port: configData.Port(),
	}

	// получаем структуру с методами для работы с соединениями
	l, err := net.ListenTCP("tcp", &laddr)
	if err != nil {
		log.Fatalf("сервер не может быть запущен: %v", err)
	}
	defer connection.Close(l, "")

	mlog.Infof("Запуск сервера с адресом %v на порту %d", laddr.IP, laddr.Port)
	for {
		mlog.Infof("tcp сокет слушает соединения")
		// слушаем сокетные соединения (запросы)
		conn, err := l.AcceptTCP()
		if err != nil {
			mlog.Errorf(err)
		}
		mlog.Infof("запрос на соединение от клиента %s принят", conn.RemoteAddr().String())

		// создаем структуру с данными клиентского соединения и обрабатываем каждое клиентское соединение в отдельной горутине
		go connection.New(conn, configData.RootPath(), t).ProcessingConn()
	}
}
