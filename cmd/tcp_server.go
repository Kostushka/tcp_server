package main

import (
	"errors"
	"flag"
	"log"
	"net"
	"os"

	mlog "github.com/Kostushka/tcp_server/internal/log"
	"github.com/Kostushka/tcp_server/internal/netf"
)

var (
	errNoRootDir   = errors.New("Не указан путь до *корневого* каталога")
	errInvalidAddr = errors.New("Указан некорректный IP-адрес")
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
	// должен быть указан путь до файла шаблона
	var fileTemplate string
	flag.StringVar(&fileTemplate, "templ", "./html/filesPage.html", "template for displaying file names")

	flag.Parse()

	// должен быть указан путь до домашнего каталога
	if rootPath == "" {
		log.Fatal(errNoRootDir)
	}

	// IP адрес должен быть корректным
	var addr net.IP
	if addr = net.ParseIP(listenAddress); addr == nil {
		log.Fatal(errInvalidAddr)
	}

	// парсим шаблон для отображения имен файлов
	template, err := os.ReadFile(fileTemplate)
	if err != nil {
		log.Fatal(err)
	}

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

	mlog.InfoLog.Printf("Запуск сервера с адресом %v на порту %d", laddr.IP, laddr.Port)
	for {
		mlog.InfoLog.Printf("tcp сокет слушает соединения")
		// слушаем сокетные соединения (запросы)
		conn, err := l.AcceptTCP()
		if err != nil {
			mlog.ErrorLog.Println(err)
		}
		mlog.InfoLog.Printf("запрос на соединение от клиента %s принят", conn.RemoteAddr().String())

		// обрабатываем каждое клиентское соединение в отдельной горутине
		go netf.ProcessingConn(conn, rootPath, &template)
	}
}
