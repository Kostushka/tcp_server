package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
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

type ResponseHeaders map[string]string

var infoLog *log.Logger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
var errorLog *log.Logger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

func main() {

	if len(os.Args) == 1 {
		log.Fatal(errors.New("Не указан путь до домашнего каталога"))
	}
	rootPath := os.Args[1]
	
	// объявляем структуру с данными будущего сервера
	laddr := net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 5000,
	}
	// получаем структуру с методами для работы с соединениями
	l, err := net.ListenTCP("tcp", &laddr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	infoLog.Printf("Запуск сервера на порту %d", laddr.Port)
	for {
		infoLog.Printf("tcp сокет слушает соединения")
		// слушаем сокетные соединения (запросы)
		conn, err := l.AcceptTCP()
		if err != nil {
			log.Fatal(err)
		}
		infoLog.Printf("запрос на соединение от клиента %s принят", conn.RemoteAddr().String())

		// обрабатываем каждое клиентское соединение в отдельной горутине
		go func(conn *net.TCPConn) {

			infoLog.Printf("начинается работа с клиентским сокетом %s", conn.RemoteAddr().String())

			// структура с данными строки запроса HTTP-протокола
			q := QueryString{}

			// map с заголовками запроса
			reqhead := make(RequestHeaders)

			// получить данные запроса
			data, err := getRequestData(conn)
			if err == nil {
				// распарсить строку запроса в структуру, заголовки - в map
				err := parseQueryString(&q, reqhead, data)
	
				// отправить ответ в клиентский сокет
				if err != nil {
					conn.Write([]byte(err.Error()))
					errorLog.Printf("клиенту отправлен ответ с ошибкой")
				} else {
					infoLog.Printf("обработан запрос от клиента %s:", conn.RemoteAddr().String())
					fmt.Printf("%s %s %s\n", q.method, q.path, q.protocol)
					for k, v := range reqhead {
						fmt.Println(k, v)
					}
					writeResponse(conn, rootPath, q.path)
				}
			}
			// закрыть клиентское соединение
			conn.Close()
			infoLog.Printf("клиентское соединение %s закрыто", conn.RemoteAddr().String())
		}(conn)
	}
}

func parseQueryString(q *QueryString, reqhead RequestHeaders, data []byte) error {
	// читаем строку из буфера
	var queryBuf string
	var i int
	for i = 0; string(data[i]) != "\n"; i++ {
		queryBuf += string(data[i])
	}
	queryBuf += string(data[i])
	i++

	// парсим строку запроса
	buf := strings.Split(queryBuf, " ")
	if len(buf) < 3 {
		errorText := "incorrect request format: not HTTP\n"
		errorLog.Printf(errorText)
		return errors.New(errorText)
	}
	q.method = buf[0]
	q.path = buf[1]
	q.protocol = buf[2]

	// парсим заголовки
	var prev byte
	var str string
	for {
		for string(data[i]) != "\n" {
			str += string(data[i])
			i++
		}
		if data[i] == prev && prev == '\n' || data[i] == '\n' && prev == '\r'{
			break
		}
		str += string(data[i])
		buf := strings.Split(str, ": ")
		reqhead[buf[0]] = buf[1]
		prev = data[i]
		i++
	}
	return nil
}

func getRequestData(conn *net.TCPConn) ([]byte, error) {
	// буфер для чтения из клиентского сокета
	buf := make([]byte, 4096)

	var data []byte
	// пока клиентский сокет пишет, читаем в буфер
	for {
		_, err := conn.Read(buf)

		// по возвращении клиентским сокетом EOF или другой ошибки логируем ошибку, так как не успели вычитать все данные, а клиент уже закрыл сокет
		if err != nil {
			if err == io.EOF {
				infoLog.Printf("Клиент закрыл соединение: %v", err)
				return nil, err
			} else {
				log.Fatal(err)
			}
		}
		// по возвращении клиентским сокетом пустой строки, перестаем читать
		if strings.Contains(string(buf), "\r\n\r\n") || strings.Contains(string(buf), "\n\n"){
			data = append(data, buf...)
			return data, nil
		}

		// если размер данных больше, чем размер буфера
		if len(buf) == cap(buf) {
			data = append(data, buf...)
			buf = make([]byte, len(buf))
		}
	}
}

type ResponseData struct {
	status string
	phrase string
	size   string
}

func writeResponse(conn *net.TCPConn, path, queryPath string) {
	respStatus := ResponseStatusLine{}
	respHeaders := make(ResponseHeaders)

	// открываем запрашиваемый файл
	f, err := os.Open(path + queryPath)

	// файл должен быть
	switch {
	// файл не существует
	case errors.Is(err, fs.ErrNotExist):
		errorLog.Println(err)
		// формируем данные для ответа
		data := ResponseData{
			status: "404",
			phrase: "Not Found",
		}
		// создаем ответ сервера для клиента
		createResponse(&respStatus, respHeaders, data)
		// пишем ответ в клиентский сокет
		err := writeToConn(conn, respStatus, respHeaders)
		if err != nil {
			log.Fatal(err)
		}
	case err != nil:
		log.Fatal(err)
	default:
		infoLog.Printf("определен путь до файла %s:", path+queryPath)

		fi, err := f.Stat()
		if err != nil {
			log.Fatal(err)
		}
		// файл не должен быть каталогом
		if fi.IsDir() {
			errorLog.Printf("%v is a directory", fi.Name())
			data := ResponseData{
				status: "400",
				phrase: "Bad Request",
			}
			// создаем ответ сервера для клиента
			createResponse(&respStatus, respHeaders, data)
			// пишем ответ в клиентский сокет
			err := writeToConn(conn, respStatus, respHeaders)
			if err != nil {
				log.Fatal(err)
			}
			return
		}
		
		data := ResponseData{
			status: "200",
			phrase: "OK",
			size:   strconv.FormatInt(fi.Size(), 10),
		}
		createResponse(&respStatus, respHeaders, data)
		err = writeToConn(conn, respStatus, respHeaders)
		if err != nil {
			log.Fatal(err)
		}
		// читаем файл
		fileBuf := make([]byte, fi.Size()) // если указать размер буфера больше размера файла, то буфер будет содержать в конце нули
										   // curl и браузер не будут ориентироваться на заголовок Content-Type: - они скажут, что это бинарный файл:
										   // Warning: Binary output can mess up your terminal. Use "--output -" to tell 
										   // Warning: curl to output it to your terminal anyway, or consider "--output 
										   // Warning: <FILE>" to save to a file.
		for {
			_, err := f.Read(fileBuf)
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}
			conn.Write(fileBuf)
		}
		infoLog.Printf("клиенту отправлено тело ответа")
	}
	defer f.Close()
}

func createResponse(respStatus *ResponseStatusLine, respHeaders ResponseHeaders, data ResponseData) {
	respStatus.version = "HTTP/1.1"
	respStatus.status = data.status
	respStatus.phrase = data.phrase

	respHeaders["Server:"] = "someserver/1.18.0"
	respHeaders["Connection:"] = "close"
	respHeaders["Date:"] = time.Now().Format(time.UnixDate)
	if data.size != "" {
		respHeaders["Size:"] = data.size
	}
	respHeaders["Content-Type:"] = "text/plain; charset=utf-8"
}

func writeToConn(conn *net.TCPConn, respStatus ResponseStatusLine, respHeaders ResponseHeaders) error {
	// записать в клиентский сокет статусную строку
	_, err := conn.Write([]byte(strings.Join([]string{respStatus.version, respStatus.status, respStatus.phrase}, " ") + "\n"))
	if err != nil {
		return err
	}
	fmt.Printf("%s %s %s\n", respStatus.version, respStatus.status, respStatus.phrase)

	// записать в клиентский сокет заголовки ответа
	for k, v := range respHeaders {
		_, err := conn.Write([]byte(k + " " + v + "\n"))
		fmt.Println(k, v)
		if err != nil {
			return err
		}
	}
	_, err = conn.Write([]byte("\n"))
	if err != nil {
		return err
	}

	infoLog.Printf("клиенту отправлены заголовки ответа")
	return nil
}
