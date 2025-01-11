package main

import (
	"flag"
	"github.com/Kostushka/tcp_server/internal/netf"
	"github.com/Kostushka/tcp_server/internal/types"
	"log"
	"net"
	"os"
)

func main() {

	// должен быть указан путь до домашнего каталога
	var rootPath string
	flag.StringVar(&rootPath, "path", "", "a path to home directory")
	// должен быть указан адрес, на котором будет запущен сервер
	var listenAddress string
	flag.StringVar(&listenAddress, "IP", "127.0.0.1", "a listening address")
	// должен быть указан порт, на которм сервер будет принимаь запросы на соединение
	var port int
	flag.IntVar(&port, "port", 5000, "a port")

	flag.Parse()

	// должен быть указан путь до домашнего каталога
	if rootPath == "" {
		log.Fatal(types.ErrNoRootDir)
	}

	// IP адрес должен быть корректным
	var addr net.IP
	if addr = net.ParseIP(listenAddress); addr == nil {
		log.Fatal(types.ErrInvalidAddr)
	}

	// парсим шаблон для отображения имен файлов
	t, err := os.ReadFile("./html/filesPage.html")
	if err != nil {
		log.Fatal(err)
	}
	types.TemplateDirNames = t

	// объявляем структуру с данными будущего сервера
	laddr := net.TCPAddr{
		IP:   addr,
		Port: port,
	}

	// получаем структуру с методами для работы с соединениями
	l, err := net.ListenTCP("tcp", &laddr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	types.InfoLog.Printf("Запуск сервера с адресом %v на порту %d", laddr.IP, laddr.Port)
	for {
		types.InfoLog.Printf("tcp сокет слушает соединения")
		// слушаем сокетные соединения (запросы)
		conn, err := l.AcceptTCP()
		if err != nil {
			types.ErrorLog.Println(err)
		}
		types.InfoLog.Printf("запрос на соединение от клиента %s принят", conn.RemoteAddr().String())

		// обрабатываем каждое клиентское соединение в отдельной горутине
		go netf.ProcessingConn(conn, rootPath)
	}
}
