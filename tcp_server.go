package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type QueryString struct {
	method   string
	path     string
	protocol string
}

type RequestHeaders map[string]string

type ResponseStatusLine struct {
	version string
	status  string
	phrase  string
}

type ResponseHeaders []string

const (
	StatusBadRequest          = 400
	StatusForbidden           = 403
	StatusNotFound            = 404
	StatusInternalServerError = 500
)

var infoLog *log.Logger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
var errorLog *log.Logger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

func main() {

	// должен быть указан путь до домашнего каталога
	rootPath := flag.String("path", "", "a path to home directory")
	// должен быть указан адрес, на котором будет запущен сервер
	listenAddress := flag.String("IP", "127.0.0.1", "a listening address")
	// должен быть указан порт, на которм сервер будет принимаь запросы на соединение
	port := flag.Int("port", 5000, "a port")

	flag.Parse()

	// должен быть указан путь до домашнего каталога
	if *rootPath == "" {
		log.Fatal(errors.New("Не указан путь до домашнего каталога"))
	}

	// IP адрес должен быть корректным
	var addr net.IP
	if addr = net.ParseIP(*listenAddress); addr == nil {
		log.Fatal(errors.New("Указан некорректный IP-адрес"))
	}

	// объявляем структуру с данными будущего сервера
	laddr := net.TCPAddr{
		IP:   addr,
		Port: *port,
	}

	// получаем структуру с методами для работы с соединениями
	l, err := net.ListenTCP("tcp", &laddr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	infoLog.Printf("Запуск сервера с адресом %v на порту %d", laddr.IP, laddr.Port)
	for {
		infoLog.Printf("tcp сокет слушает соединения")
		// слушаем сокетные соединения (запросы)
		conn, err := l.AcceptTCP()
		if err != nil {
			errorLog.Println(err)
		}
		infoLog.Printf("запрос на соединение от клиента %s принят", conn.RemoteAddr().String())

		// обрабатываем каждое клиентское соединение в отдельной горутине
		go func(conn *net.TCPConn) {
			processingConn(conn, *rootPath)
		}(conn)
	}
}

func processingConn(conn *net.TCPConn, rootPath string) {

	defer func() {
		// закрыть клиентское соединение
		conn.Close()
		infoLog.Printf("клиентское соединение %s закрыто", conn.RemoteAddr().String())
	}()

	infoLog.Printf("начинается работа с клиентским сокетом %s", conn.RemoteAddr().String())

	// получить данные запроса
	data, err := getRequestData(conn)
	// по возвращении клиентским сокетом EOF или другой ошибки логируем ошибку, так как не успели вычитать все данные, а клиент уже закрыл сокет
	if err != nil {
		errorLog.Println(err)
		return
	}

	// распарсить строку запроса в структуру, заголовки - в map
	q, reqhead, err := parseQueryString(data)
	// отправить в клиентский сокет ошибку
	if err != nil {
		errorLog.Println(err)
		// некорректный запрос
		data := createResponseData(StatusBadRequest, "")
		// создаем ответ сервера для клиента
		err := writeResponseHeader(conn, data)
		if err != nil {
			errorLog.Println(err)
		}
		return
	}

	// логируем клиентские заголовки
	infoLog.Println("обработан запрос от клиента:")

	infoLog.Printf("\"%v %v %v\" %v %v \"%v\"\n",
		q.method, q.path, q.protocol, conn.RemoteAddr().String(), reqhead["Host"], reqhead["User-Agent"])

	// отправить ответ клиенту
	err = writeResponse(conn, rootPath, q.path)
	if err != nil {
		errorLog.Println(err)
	}
}

func parseQueryString(data []byte) (*QueryString, RequestHeaders, error) {
	// структура с данными строки запроса HTTP-протокола
	q := QueryString{}
	// map с заголовками запроса
	reqhead := make(RequestHeaders)

	// читаем строку из буфера
	var queryBuf string
	var i int
	// в конце строки ожидаем либо \r\n, либо \n
	for i = 0; string(data[i]) != "\r" && string(data[i]) != "\n"; i++ {
		queryBuf += string(data[i])
	}
	if string(data[i]) == "\r" {
		i++
	}
	i++

	// парсим строку запроса
	buf := strings.Split(queryBuf, " ")
	if len(buf) < 3 {
		return &q, reqhead, errors.New("incorrect request format: not HTTP")
	}
	q.method = buf[0]
	q.path = buf[1]
	q.protocol = buf[2]

	// парсим заголовки
	headerString := data[i:]
	buf = strings.Split(string(headerString), "\r\n")
	// если в конце строки не \r\n, а \n
	if len(buf) == 1 {
		buf = strings.Split(string(headerString), "\n")
	}
	// в конце после заголовков ожидаем пустую строку
	for j := 0; buf[j] != ""; j++ {
		hb := strings.Split(buf[j], ": ")
		reqhead[hb[0]] = hb[1]
	}
	return &q, reqhead, nil
}

func getRequestData(conn *net.TCPConn) ([]byte, error) {
	// буфер для чтения из клиентского сокета
	buf := make([]byte, 4096)

	var data []byte
	// пока клиентский сокет пишет, читаем в буфер
	for {
		n, err := conn.Read(buf)
		// обрабатываем ошибку при чтении
		if err != nil {
			if err == io.EOF {
				err = fmt.Errorf("Клиент преждевременно закрыл соединение: %w", err)
			}
			return nil, err
		}
		// добавляем к итоговому срезу считанные в буфер данные
		data = append(data, buf[:n]...)

		// по возвращении клиентским сокетом пустой строки, перестаем читать
		if bytes.Contains(data, []byte("\r\n\r\n")) || bytes.Contains(data, []byte("\n\n")) {
			return data, nil
		}
	}
}

type ResponseData struct {
	status string
	phrase string
	size   string
}

func writeResponse(conn *net.TCPConn, path, queryPath string) error {
	// открываем запрашиваемый файл
	f, err := os.Open(path + queryPath)

	if err != nil {
		// файл должен быть, иначе 404
		if errors.Is(err, fs.ErrNotExist) {
			errorLog.Println(err)
			// формируем данные для ответа: файл не найден
			data := createResponseData(StatusNotFound, "")
			// создаем ответ сервера для клиента
			return writeResponseHeader(conn, data)
		}
		// файл не был открыт - 500
		data := createResponseData(StatusInternalServerError, "")
		// создаем ответ сервера для клиента
		return writeResponseHeader(conn, data)
	}
	defer f.Close()

	infoLog.Printf("определен путь до файла %s:", path+queryPath)
	// отправить файл клиенту
	return sendFile(conn, f)
}

func createResponseData(code int, size string) *ResponseData {
	// заполняем структуру данных для формированя ответа клиенту
	return &ResponseData{
		status: strconv.Itoa(code),
		phrase: http.StatusText(code),
		size:   size,
	}
}

func sendFile(conn *net.TCPConn, f *os.File) error {
	fi, err := f.Stat()
	if err != nil {
		// файл не отправлен - 500
		data := createResponseData(StatusInternalServerError, "")
		return writeResponseHeader(conn, data)
	}
	// файл не должен быть каталогом, иначе 403
	if fi.IsDir() {
		errIsDir := fmt.Errorf("%v is a directory", fi.Name())

		data := createResponseData(StatusForbidden, "")
		// создаем ответ сервера для клиента
		err := writeResponseHeader(conn, data)
		if err != nil {
			return fmt.Errorf("файл не был отправлен клиенту: %v: %v", err, errIsDir)
		}
		return fmt.Errorf("файл не был отправлен клиенту: %v", errIsDir)
	}
	// файл готов к отправке
	data := createResponseData(200, strconv.FormatInt(fi.Size(), 10))
	// создаем ответ сервера для клиента
	if err = writeResponseHeader(conn, data); err != nil {
		return err
	}

	// читаем файл
	fileBuf := make([]byte, fi.Size()) // если указать размер буфера больше размера файла, то буфер будет содержать в конце нули
	// curl и браузер не будут ориентироваться на заголовок Content-Type: - они скажут, что это бинарный файл:
	// Warning: Binary output can mess up your terminal. Use "--output -" to tell
	// Warning: curl to output it to your terminal anyway, or consider "--output
	// Warning: <FILE>" to save to a file.
	for {
		_, err := f.Read(fileBuf)
		// читаем файл, пока не встретим EOF
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// записать содержимое буфера в клиентский сокет
		n, err := conn.Write(fileBuf)
		if n != len(fileBuf) {
			return fmt.Errorf("ошибка записи: буфер содержит %v байт, а в клиентский сокет записано %v байт", len(fileBuf), n)
		}
		if err != nil {
			return err
		}
	}
	infoLog.Printf("клиенту отправлено тело ответа")
	return nil
}

func writeResponseHeader(conn *net.TCPConn, data *ResponseData) error {
	respStatus := ResponseStatusLine{}
	respHeaders := ResponseHeaders{}

	respStatus.version = "HTTP/1.1"
	respStatus.status = data.status
	respStatus.phrase = data.phrase

	respHeaders = append(respHeaders, "Server: someserver/1.18.0")
	respHeaders = append(respHeaders, "Connection: close")
	respHeaders = append(respHeaders, "Date: "+time.Now().Format(time.UnixDate))
	if data.size != "" {
		respHeaders = append(respHeaders, "Size: "+data.size)
	}
	respHeaders = append(respHeaders, "Content-Type:: text/plain; charset=utf-8")

	// пишем ответ в клиентский сокет
	return writeToConn(conn, respStatus, respHeaders)
}

func writeToConn(conn *net.TCPConn, respStatus ResponseStatusLine, respHeaders ResponseHeaders) error {
	// записать в клиентский сокет статусную строку
	statusString := strings.Join([]string{respStatus.version, respStatus.status, respStatus.phrase}, " ") + "\n"
	n, err := conn.Write([]byte(statusString))
	if n != len(statusString) {
		return fmt.Errorf("ошибка записи: строка содержит %v байт, а в клиентский сокет записано %v байт", len(statusString), n)
	}
	if err != nil {
		return err
	}
	fmt.Printf("%s %s %s\n", respStatus.version, respStatus.status, respStatus.phrase)

	// записать в клиентский сокет заголовки ответа
	for _, v := range respHeaders {
		header := v + "\n"
		n, err := conn.Write([]byte(header))
		if n != len(header) {
			return fmt.Errorf("ошибка записи: заголовок содержит %v байт, а в клиентский сокет записано %v байт", len(header), n)
		}
		fmt.Println(v)
		if err != nil {
			return err
		}
	}
	n, err = conn.Write([]byte("\n"))
	if n != len("\n") {
		return fmt.Errorf("ошибка записи: строка содержит %v байт, а в клиентский сокет записано %v байт", len("\n"), n)
	}
	if err != nil {
		return err
	}

	infoLog.Printf("клиенту отправлены заголовки ответа")
	return nil
}
