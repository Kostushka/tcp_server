package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
	StatusOK                  = 200
	StatusBadRequest          = 400
	StatusForbidden           = 403
	StatusNotFound            = 404
	StatusInternalServerError = 500
)

var (
	errNoRootDir      = errors.New("Не указан путь до *корневого* каталога")
	errInvalidAddr    = errors.New("Указан некорректный IP-адрес")
	errInvalidHttpReq = errors.New("incorrect request format: not HTTP")
)

var infoLog *log.Logger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
var errorLog *log.Logger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

var templateDirNames []byte

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
		log.Fatal(errNoRootDir)
	}

	// IP адрес должен быть корректным
	var addr net.IP
	if addr = net.ParseIP(listenAddress); addr == nil {
		log.Fatal(errInvalidAddr)
	}

	// парсим шаблон для отображения имен файлов
	t, err := os.ReadFile("./html/filesPage.html")
	if err != nil {
		log.Fatal(err)
	}
	templateDirNames = t

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
		go processingConn(conn, rootPath)
	}
}

type StatusData struct {
	code int
	size int64
	name string
}

// обрабатываем клиентское соединение
func processingConn(conn *net.TCPConn, rootPath string) {
	defer func() {
		// закрыть клиентское соединение
		err := conn.Close()
		if err != nil {
			errorLog.Println(err)
		} else {
			infoLog.Printf("клиентское соединение %s закрыто", conn.RemoteAddr().String())
		}
	}()

	infoLog.Printf("начинается работа с клиентским сокетом %s", conn.RemoteAddr().String())

	// получить данные запроса
	data, err := getRequestData(conn)
	// по возвращении клиентским сокетом EOF или другой ошибки логируем ошибку,
	// так как не успели вычитать все данные, а клиент уже закрыл сокет
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
		err = sendResponseHeader(conn, &StatusData{
			code: StatusBadRequest,
			size: 0,
			name: "",
		}, err)
		errorLog.Println(err)
		return
	}

	// логируем клиентские заголовки
	infoLog.Println("распарсили данные, поступившие от клиента:")

	infoLog.Printf("\"%v %v %v\" %v %v \"%v\"\n",
		q.method, q.path, q.protocol, conn.RemoteAddr().String(), reqhead["Host"], reqhead["User-Agent"])

	// отправить ответ клиенту
	err = workingWithFile(conn, rootPath, q.path)
	if err != nil {
		errorLog.Println(err)
	}
}

// парсим строку запроса в структуру, заголовки - в map
func parseQueryString(data []byte) (*QueryString, RequestHeaders, error) {
	// структура с данными строки запроса HTTP-протокола
	q := QueryString{}
	// map с заголовками запроса
	reqhead := make(RequestHeaders, 5)

	// читаем строку из буфера
	var queryBuf strings.Builder
	var i int
	// в конце строки ожидаем либо \r\n, либо \n
	for i = 0; string(data[i]) != "\r" && string(data[i]) != "\n"; i++ {
		if err := queryBuf.WriteByte(data[i]); err != nil {
			return nil, nil, err
		}
	}
	// если в конце строки \r\n - пропускаем два символа для перехода на новую строку
	if string(data[i]) == "\r" {
		i++
	}
	i++

	// парсим строку запроса
	buf := strings.Split(myTrimSpace(queryBuf.String()), " ")
	if len(buf) < 3 {
		return nil, nil, errInvalidHttpReq
	}
	// декодируем path на случай, если он не в латинице
	convertPath, err := url.QueryUnescape(buf[1])
	if err != nil {
		return nil, nil, err
	}
	q.method = buf[0]
	q.path = convertPath
	q.protocol = buf[2]

	// парсим заголовки
	headerBuf := data[i:]
	buf = strings.Split(string(headerBuf), "\r\n")
	// если в конце строки не \r\n, а \n
	if len(buf) == 1 {
		buf = strings.Split(string(headerBuf), "\n")
	}
	// в конце после заголовков ожидаем пустую строку
	for j := 0; buf[j] != ""; j++ {
		hb := strings.Split(buf[j], ":")
		reqhead[hb[0]] = strings.TrimSpace(hb[1])
	}
	return &q, reqhead, nil
}

// учитываем, что строка запроса может содержать более одного пробела, например:
// GET        /                HTTP/1.1
// удаляем лишние пробелы
func myTrimSpace(str string) string {
	var prev string
	var i int
	var res strings.Builder
	for ; i < len(str); i++ {
		if string(str[i]) == prev && prev == " " {
			continue
		}
		prev = string(str[i])
		res.WriteByte(str[i])
	}
	return res.String()
}

// получаем данные запроса
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
	name   string
}

func sendResponseHeader(w io.Writer, statusData *StatusData, mainError error) error {
	// формируем данные для ответа
	data := createResponseData(statusData)

	// отправляем заголовки клиенту
	if err := writeResponseHeader(w, data); err != nil {
		return fmt.Errorf("%w: %w", err, mainError)
	}
	return mainError
}

func workingWithFile(conn *net.TCPConn, rootPath, queryPath string) error {
	path := filepath.Join(rootPath, queryPath)

	// открываем запрашиваемый файл
	f, err := openFile(conn, path)
	if err != nil {
		return err
	}
	// закрыть файл
	defer func() {
		err := f.Close()
		if err != nil {
			errorLog.Println(err)
		}
	}()

	infoLog.Printf("определен путь до файла %s:", path)

	// получить информацию о файле
	fi, err := f.Stat()
	if err != nil {
		// файл не отправлен - 500
		err = sendResponseHeader(conn, &StatusData{
			code: StatusInternalServerError,
			size: 0,
			name: "",
		}, err)
		return fmt.Errorf("файл %s не готов к отправке: %w", path, err)
	}

	// если файл - каталог, выводим его содержимое
	if fi.IsDir() {
		infoLog.Printf("файл %s: is a directory", path)
		// выводим содержимое каталога
		if err := showDir(conn, rootPath, queryPath); err != nil {
			// содержимое каталога не отправлено - 500
			err = sendResponseHeader(conn, &StatusData{
				code: StatusInternalServerError,
				size: 0,
				name: "",
			}, err)
			return fmt.Errorf("содержимое каталога %s не готово к отправке: %w", path, err)
		}
		infoLog.Printf("клиенту отправлен html файл с содержимым %s", path)
		return nil
	}

	// отправляем клиенту заголовки
	err = sendResponseHeader(conn, &StatusData{
		code: StatusOK,
		size: fi.Size(),
		name: fi.Name(),
	}, nil)

	if err != nil {
		return err
	}

	// отправить файл клиенту
	err = sendFile(conn, f, fi.Size())
	if err != nil {
		return fmt.Errorf("файл не был отправлен клиенту: %w", err)
	}
	return nil
}

func openFile(w *net.TCPConn, path string) (*os.File, error) {
	// открываем запрашиваемый файл
	f, err := os.Open(path)

	if err != nil {
		// файл должен быть, иначе 404
		if errors.Is(err, fs.ErrNotExist) {
			// создаем ответ сервера для клиента: файл не найден
			err = sendResponseHeader(w, &StatusData{
				code: StatusNotFound,
				size: 0,
				name: ""}, err)
			return nil, err
		}
		// файл должен быть доступен, иначе 403
		if errors.Is(err, fs.ErrPermission) {
			// создаем ответ сервера для клиента: доступ к файлу запрещен
			err = sendResponseHeader(w, &StatusData{
				code: StatusForbidden,
				size: 0,
				name: ""}, err)
			return nil, err
		}
		// файл не был открыт - 500
		// создаем ответ сервера для клиента: ошибка со стороны сервера
		err = sendResponseHeader(w, &StatusData{
			code: StatusInternalServerError,
			size: 0,
			name: ""}, err)
		return nil, err
	}
	return f, nil
}

func createResponseData(data *StatusData) *ResponseData {
	// заполняем структуру данных для формированя ответа клиенту
	return &ResponseData{
		status: strconv.Itoa(data.code),
		phrase: http.StatusText(data.code),
		size:   strconv.FormatInt(data.size, 10),
		name:   data.name,
	}
}

func showDir(w io.Writer, rootPath, queryPath string) error {
	type args struct {
		RootPath string
		DirName  string
		Files    []string
	}

	// получаем файлы, находящиеся в каталоге
	files, err := os.ReadDir(filepath.Join(rootPath, queryPath))
	if err != nil {
		errorLog.Println(err)
		return err
	}

	// получаем имена файлов
	names := []string{}
	for _, v := range files {
		if strings.HasPrefix(v.Name(), ".") {
			continue
		}
		names = append(names, v.Name())
	}

	// используем шаблон для отображения имен файлов
	t, err := template.New("index").Parse(string(templateDirNames))
	if err != nil {
		errorLog.Println(err)
		return err
	}
	// если запрос идет на корень, то оставляем переменную с путем запроса пустой,
	// так как по умолчанию в html шаблоне путь запроса до директории и имена содержащихся в ней файлов/каталогов разделяет слеш
	if queryPath == "/" {
		queryPath = ""
	}

	buf := new(bytes.Buffer)
	// применяем шаблон к структуре данных, пишем выходные данные в буфер
	err = t.Execute(buf, args{
		RootPath: filepath.Join(rootPath, queryPath),
		DirName:  queryPath,
		Files:    names,
	})
	if err != nil {
		return err
	}

	// отправляем заголовки
	err = sendResponseHeader(w, &StatusData{
		code: StatusOK,
		size: int64(buf.Len()),
		name: ""}, nil)

	if err != nil {
		return err
	}

	// записать содержимое буфера в клиентский сокет
	_, err = w.Write(buf.Bytes())
	return err
}

func sendFile(w io.Writer, f *os.File, fileSize int64) error {
	// читаем файл
	fileBuf := make([]byte, fileSize) // если указать размер буфера больше размера файла, то буфер будет содержать в конце нули
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
		_, err = w.Write(fileBuf)
		if err != nil {
			return err
		}
	}
	infoLog.Printf("клиенту отправлено тело ответа")
	return nil
}

func writeResponseHeader(w io.Writer, data *ResponseData) error {
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

	// если у файла в названии есть расширение, пишем тип файла в заголовок Content-Type
	extIndex := strings.LastIndex(data.name, ".")
	if extIndex == -1 {
		respHeaders = append(respHeaders, "Content-Type: charset=utf-8")
		// пишем ответ в клиентский сокет
		return writeToConn(w, respStatus, respHeaders)
	}
	ext := data.name[extIndex:]
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		respHeaders = append(respHeaders, "Content-Type: charset=utf-8")
		// пишем ответ в клиентский сокет
		return writeToConn(w, respStatus, respHeaders)
	}
	respHeaders = append(respHeaders, "Content-Type: "+contentType)

	// пишем ответ в клиентский сокет
	return writeToConn(w, respStatus, respHeaders)
}

func writeToConn(w io.Writer, respStatus ResponseStatusLine, respHeaders ResponseHeaders) error {
	// сформировать статусную строку
	var statusString strings.Builder

	fmt.Fprintf(&statusString, "%s %s %s\n", respStatus.version, respStatus.status, respStatus.phrase)

	// записать в клиентский сокет статусную строку
	_, err := w.Write([]byte(statusString.String()))
	if err != nil {
		return err
	}
	infoLog.Println("---")
	infoLog.Printf("%s", statusString.String())

	// сформировать буфер с заголовками ответа
	var headers strings.Builder
	for _, v := range respHeaders {
		header := v + "\n"
		_, err := headers.WriteString(header)
		if err != nil {
			return err
		}
		infoLog.Println(v)
	}
	infoLog.Println("---")
	// записать в клиентский сокет заголовки ответа
	_, err = w.Write([]byte(headers.String() + "\n"))
	if err != nil {
		return err
	}

	infoLog.Printf("клиенту отправлены заголовки ответа")
	return nil
}
