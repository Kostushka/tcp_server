package config

import (
	"errors"
	"flag"
	"net"
)

var (
	ErrNoRootDir   = errors.New("не указан путь до *корневого* каталога")
	ErrInvalidAddr = errors.New("указан некорректный IP-адрес")
)

const portNumber = 5000

// данные для конфигурации сервера
type configData struct {
	rootPath      string
	listenAddress net.IP
	port          int
	log           string
	fileTemplate  string
}

func (c *configData) RootPath() string {
	return c.rootPath
}
func (c *configData) ListenAddress() net.IP {
	return c.listenAddress
}
func (c *configData) Port() int {
	return c.port
}
func (c *configData) Log() string {
	return c.log
}
func (c *configData) FileTemplate() string {
	return c.fileTemplate
}

// функция-конструктор для получения структуры с конфигурационными данными
func NewConfigData() (*configData, error) {
	// должен быть указан путь до домашнего каталога
	var rootPath string
	flag.StringVar(&rootPath, "path", "", "a path to home directory")

	// должен быть указан адрес, на котором будет запущен сервер
	var listenAddress string
	flag.StringVar(&listenAddress, "IP", "127.0.0.1", "a listening address")

	// должен быть указан порт, на которм сервер будет принимаь запросы на соединение
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

	return &configData{
		rootPath:      rootPath,
		listenAddress: addr,
		port:          port,
		log:           log,
		fileTemplate:  fileTemplate,
	}, nil
}
