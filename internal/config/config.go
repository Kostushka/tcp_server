// Package config - пакет для получения конфигурационных данных для запуска сервера
package config

import (
	"errors"
	"flag"
	"net"
)

var (
	// ErrNoRootDir - не указан путь до корневого каталога
	ErrNoRootDir = errors.New("не указан путь до *корневого* каталога")
	// ErrInvalidAddr - указан некорректный IP-адрес
	ErrInvalidAddr = errors.New("указан некорректный IP-адрес")
)

const portNumber = 5000

// Data - данные для конфигурации сервера
type Data struct {
	rootPath      string
	listenAddress net.IP
	port          int
	log           string
	fileTemplate  string
}

// RootPath - возвращает путь до домашнего каталога
func (c *Data) RootPath() string {
	return c.rootPath
}

// ListenAddress - возвращает адрес, на котором будет запущен сервер
func (c *Data) ListenAddress() net.IP {
	return c.listenAddress
}

// Port - возвращает порт, на которм сервер будет принимать запросы на соединение
func (c *Data) Port() int {
	return c.port
}

// Log - возвращает имя файла для записи лога в него или ”
func (c *Data) Log() string {
	return c.log
}

// FileTemplate - возвращает путь до файла шаблона с отображением имен файлов
func (c *Data) FileTemplate() string {
	return c.fileTemplate
}

// NewConfigData - функция-конструктор для получения структуры с конфигурационными данными
func NewConfigData() (*Data, error) {
	// должен быть указан путь до домашнего каталога
	var rootPath string

	flag.StringVar(&rootPath, "path", "", "a path to home directory")

	// должен быть указан адрес, на котором будет запущен сервер
	var listenAddress string

	flag.StringVar(&listenAddress, "IP", "127.0.0.1", "a listening address")

	// должен быть указан порт, на которм сервер будет принимать запросы на соединение
	var port int

	flag.IntVar(&port, "port", portNumber, "a port")

	// должно быть указано имя файла для записи лога в него, иначе вывод лога будет в stdout
	var log string

	flag.StringVar(&log, "log", "", "output log to file")

	// должен быть указан путь до файла шаблона с отображением имен файлов
	var fileTemplate string

	flag.StringVar(&fileTemplate, "templ", "./html/filesPage.html", "template for displaying file names")

	flag.Parse()

	// должен быть указан путь до домашнего каталога
	if rootPath == "" {
		return nil, ErrNoRootDir
	}

	// IP адрес должен быть корректным
	var addr net.IP
	if addr = net.ParseIP(listenAddress); addr == nil {
		return nil, ErrInvalidAddr
	}

	return &Data{
		rootPath:      rootPath,
		listenAddress: addr,
		port:          port,
		log:           log,
		fileTemplate:  fileTemplate,
	}, nil
}
