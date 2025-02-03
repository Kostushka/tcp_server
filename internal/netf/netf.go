package netf

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Kostushka/tcp_server/internal/dirf"
	"github.com/Kostushka/tcp_server/internal/filef"
	"github.com/Kostushka/tcp_server/internal/log"
	"github.com/Kostushka/tcp_server/internal/parsequeryf"
)

const (
	statusOK                  = 200
	statusBadRequest          = 400
	statusForbidden           = 403
	statusNotFound            = 404
	statusInternalServerError = 500
)

// строка статуса ответа
type responseStatusLine struct {
	version string
	status  string
	phrase  string
}

func (r *responseStatusLine) Version() string {
	return r.version
}
func (r *responseStatusLine) Status() string {
	return r.status
}
func (r *responseStatusLine) Phrase() string {
	return r.phrase
}

// заголовки ответа
type responseHeaders []string

// собираемые данные для строки статуса и заголовков ответа
type statusData struct {
	code        int
	size        int64
	name        string
	contentType string
}

func (s *statusData) Code() int {
	return s.code
}
func (s *statusData) Size() int64 {
	return s.size
}
func (s *statusData) Name() string {
	return s.name
}
func (s *statusData) ContentType() string {
	return s.contentType
}

// сформированные данные для строки статуса и заголовков ответа
type responseData struct {
	status      string
	phrase      string
	size        string
	name        string
	contentType string
}

func (r *responseData) Status() string {
	return r.status
}
func (r *responseData) Phrase() string {
	return r.phrase
}
func (r *responseData) Size() string {
	return r.size
}
func (r *responseData) Name() string {
	return r.name
}
func (r *responseData) ContentType() string {
	return r.contentType
}

// структура с сформированными данными для строки статуса и заголовков ответа
type HeaderData struct {
	responseData *responseData
}

func (h *HeaderData) ResponseData() *responseData {
	return h.responseData
}

// формируем данные заголовков для ответа клиенту
func (h *HeaderData) SetResponseData(data *statusData) {
	// заполняем структуру данных для формирования ответа клиенту
	h.responseData = &responseData{
		status:      strconv.Itoa(data.Code()),
		phrase:      http.StatusText(data.Code()),
		size:        strconv.FormatInt(data.Size(), 10),
		name:        data.Name(),
		contentType: data.ContentType(),
	}
}
// формируем и отправляем клиенту заголовки ответа
func (h *HeaderData) WriteResponseHeader(w io.Writer) error {
	respStatus := responseStatusLine{
		version: "HTTP/1.1",
		status:  h.responseData.Status(),
		phrase:  h.responseData.Phrase(),
	}
	respHeaders := responseHeaders{}

	respHeaders = append(respHeaders, "Server: someserver/1.18.0")
	respHeaders = append(respHeaders, "Connection: close")
	respHeaders = append(respHeaders, "Date: "+time.Now().Format(time.UnixDate))
	if h.responseData.Size() != "" {
		respHeaders = append(respHeaders, "Size: "+h.responseData.Size())
	}
	// не пишем Content-Type, если ошибка
	if h.responseData.Status() != strconv.Itoa(statusOK) {
		return writeToConn(w, respStatus, respHeaders)
	}
	if h.responseData.ContentType() != "" {
		respHeaders = append(respHeaders, "Content-Type: "+h.responseData.ContentType())
		return writeToConn(w, respStatus, respHeaders)
	}

	// если у файла в названии есть расширение, пишем тип файла в заголовок Content-Type
	extIndex := strings.LastIndex(h.responseData.Name(), ".")
	if extIndex == -1 {
		respHeaders = append(respHeaders, "Content-Type: application/octet-stream")
		// пишем ответ в клиентский сокет
		return writeToConn(w, respStatus, respHeaders)
	}
	ext := h.responseData.Name()[extIndex:]
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		respHeaders = append(respHeaders, "Content-Type: application/octet-stream")
		// пишем ответ в клиентский сокет
		return writeToConn(w, respStatus, respHeaders)
	}
	respHeaders = append(respHeaders, "Content-Type: "+contentType)

	// пишем ответ в клиентский сокет
	return writeToConn(w, respStatus, respHeaders)
}

// данные запроса
type queryData struct {
	data []byte
	parsedQueryString *parsequeryf.QueryString
	parsedReqHeaders parsequeryf.RequestHeaders
}

func (q *queryData) Data() []byte {
	return q.data
}
func (q *queryData) ParsedQueryString() *parsequeryf.QueryString {
	return q.parsedQueryString
}
func (q *queryData) ParsedReqHeaders() parsequeryf.RequestHeaders {
	return q.parsedReqHeaders
}
func (q *queryData) SetData(conn *net.TCPConn) (error) {
	// получить данные запроса
	// буфер для чтения из клиентского сокета
	buf := make([]byte, 4096)

	var data []byte
	// пока клиентский сокет пишет, читаем в буфер
	for {
		n, err := conn.Read(buf)
		// обрабатываем ошибку при чтении
		if err != nil {
			// не успели вычитать все данные, клиент закрыл сокет
			if err == io.EOF {
				err = fmt.Errorf("Клиент преждевременно закрыл соединение: %w", err)
			}
			return err
		}
		// добавляем к итоговому срезу считанные в буфер данные
		data = append(data, buf[:n]...)

		// по возвращении клиентским сокетом пустой строки, перестаем читать
		if bytes.Contains(data, []byte("\r\n\r\n")) || bytes.Contains(data, []byte("\n\n")) {
			break
		}
	}
	// записать данные запроса в буфер структуры
	q.data = data
	return nil
}

func (q *queryData) SetParsedQuery(data []byte) (error) {
	// распарсить строку запроса в структуру, заголовки - в map
	queryLine, reqhead, err := parsequeryf.ParseQueryString(data)
	// отправить в клиентский сокет ошибку
	if err != nil {
		return err
	}
	// записать распарсенные данные запросы в соответсвующие поля структуры
	q.parsedQueryString = queryLine
	q.parsedReqHeaders = reqhead
	return nil
}

// файл и информация о нем
type fileData struct {
	file *os.File
	fileInfo fs.FileInfo
}

func (f *fileData) File() *os.File {
	return f.file
}

func (f *fileData) FileInfo() fs.FileInfo {
	return f.fileInfo
}

func (f *fileData) SetFile(w io.Writer, path string) error {
	var respdata *statusData

	file, err := filef.OpenFile(w, path)
	if err != nil {
		switch {
		// файл должен быть, иначе 404
		case errors.Is(err, fs.ErrNotExist):
			// создаем ответ сервера для клиента: файл не найден
			respdata = &statusData{
				code: statusNotFound,
			}
		// файл должен быть доступен, иначе 403
		case errors.Is(err, fs.ErrPermission):
			// создаем ответ сервера для клиента: доступ к файлу запрещен
			respdata = &statusData{
				code: statusForbidden,
			}
		// файл не был открыт - 500
		default:
			// создаем ответ сервера для клиента: ошибка со стороны сервера
			respdata = &statusData{
				code: statusInternalServerError,
			}
		}
		// отправляем клиенту: ошибка при открытии файла
		err = sendResponseHeader(w, respdata, err)
		return err
	}
	
	f.file = file
	return nil	
}

func (f *fileData) SetFileInfo(w io.Writer) error {
	// получить информацию о файле
	fi, err := f.File().Stat()
	if err != nil {
		// файл не отправлен - 500
		err = sendInternalServerError(w, err)
		return err
	}
	f.fileInfo = fi
	return nil
}

// обрабатываем клиентское соединение
func ProcessingConn(conn *net.TCPConn, rootPath string, template *template.Template) {
	defer func() {
		// закрыть клиентское соединение
		err := conn.Close()
		if err != nil {
			log.ErrorLog.Println(err)
		} else {
			log.InfoLog.Printf("клиентское соединение %s закрыто", conn.RemoteAddr().String())
		}
	}()

	log.InfoLog.Printf("начинается работа с клиентским сокетом %s", conn.RemoteAddr().String())

	// данные запроса
	query := queryData{}
	
	// записать данные запроса
	err := query.SetData(conn)
	if err != nil {
		// по возвращении клиентским сокетом EOF или другой ошибки логируем ошибку,
		// так как не успели вычитать все данные, а клиент уже закрыл сокет
		log.ErrorLog.Println(err)
		return
	}

	// записать распарсенные строку запроса и заголовки
	err = query.SetParsedQuery(query.Data())
	if err != nil {
		log.ErrorLog.Println(err)
		// некорректный запрос
		err = sendResponseHeader(conn, &statusData{
			code: statusBadRequest,
		}, err)
		log.ErrorLog.Println(err)
	}

	// логируем клиентские заголовки
	log.InfoLog.Println("распарсили данные, поступившие от клиента:")

	log.InfoLog.Printf("\"%v %v %v\" %v %v \"%v\"\n",
		query.ParsedQueryString().Method(), query.ParsedQueryString().Path(), query.ParsedQueryString().Protocol(), conn.RemoteAddr().String(), 
		query.ParsedReqHeaders()["Host"], query.ParsedReqHeaders()["User-Agent"])

	// работаем с путем до файла, взятым из строки запроса
	path := filepath.Join(rootPath, query.ParsedQueryString().Path())

	// файл и его данные
	f := fileData{}

	// записываем запрашиваемый файл
	err = f.SetFile(conn, path)
	if err != nil {
		log.ErrorLog.Println(err)
		return
	}

	// закрыть файл
	defer func() {
		err := f.File().Close()
		if err != nil {
			log.ErrorLog.Println(err)
		}
	}()

	log.InfoLog.Printf("определен путь до файла %s:", path)

	// записать информацию о файле
	err = f.SetFileInfo(conn)
	if err != nil {
		log.ErrorLog.Printf("файл %s не готов к отправке: %w", path, err)
		return
	}

	// если файл - каталог, выводим его содержимое
	if f.FileInfo().IsDir() {
		workingWithCatalog(conn, rootPath, query.ParsedQueryString().Path(), template)
		return
	}

	// отправляем клиенту заголовки
	err = sendResponseHeader(conn, &statusData{
		code: statusOK,
		size: f.FileInfo().Size(),
		name: f.FileInfo().Name(),
	}, nil)
	if err != nil {
		log.ErrorLog.Println(err)
		return
	}

	// отправить файл клиенту
	if err = filef.SendFile(conn, f.File(), f.FileInfo().Size()); err != nil {
		log.ErrorLog.Printf("файл не был отправлен клиенту: %w", err)
	}
}

// отправляем заголоки с ошибкой 500
func sendInternalServerError(w io.Writer, mainError error) error {
	err := sendResponseHeader(w, &statusData{
		code: statusInternalServerError,
	}, mainError)
	return err
}

func workingWithCatalog(w *net.TCPConn, rootPath, queryPath string, template *template.Template) {
	log.InfoLog.Printf("файл %s: is a directory", filepath.Join(rootPath, queryPath))

	// выводим содержимое каталога
	buf, err := dirf.ShowDir(w, rootPath, queryPath, template)
	if err != nil {
		// содержимое каталога не готово к отправке - 500
		err = sendInternalServerError(w, err)
		log.ErrorLog.Printf("содержимое каталога %s не готово к отправке: %w", filepath.Join(rootPath, queryPath), err)
		return
	}
	// отправляем заголовки
	err = sendResponseHeader(w, &statusData{
		code:        statusOK,
		size:        int64(buf.Len()),
		contentType: "text/html"}, nil)

	if err != nil {
		// заголовки не отправлены - 500
		err = sendInternalServerError(w, err)
		log.ErrorLog.Printf("содержимое каталога %s не готово к отправке: %w", filepath.Join(rootPath, queryPath), err)
		return
	}
	// записать содержимое буфера в клиентский сокет
	_, err = w.Write(buf.Bytes())
	if err != nil {
		log.ErrorLog.Println(err)
		return
	}
	log.InfoLog.Printf("клиенту отправлен html файл с содержимым %s", filepath.Join(rootPath, queryPath))
	return
}

// отправляем клиенту заголовки ответа
func sendResponseHeader(w io.Writer, statusData *statusData, mainError error) error {
	// формируем данные для ответа
	data := HeaderData{}
	data.SetResponseData(statusData)

	// отправляем заголовки клиенту
	if err := data.WriteResponseHeader(w); err != nil {
		return fmt.Errorf("%w: %w", err, mainError)
	}
	return mainError
}

// пишем заголовки в клиентский сокет
func writeToConn(w io.Writer, respStatus responseStatusLine, respHeaders responseHeaders) error {
	// сформировать статусную строку
	var statusString strings.Builder

	fmt.Fprintf(&statusString, "%s %s %s\n", respStatus.Version(), respStatus.Status(), respStatus.Phrase())

	// записать в клиентский сокет статусную строку
	_, err := w.Write([]byte(statusString.String()))
	if err != nil {
		return err
	}
	log.InfoLog.Println("---")
	log.InfoLog.Printf("%s", statusString.String())

	// сформировать буфер с заголовками ответа
	var headers strings.Builder
	for _, v := range respHeaders {
		header := v + "\n"
		_, err := headers.WriteString(header)
		if err != nil {
			return err
		}
		log.InfoLog.Println(v)
	}
	log.InfoLog.Println("---")
	// записать в клиентский сокет заголовки ответа
	_, err = w.Write([]byte(headers.String() + "\n"))
	if err != nil {
		return err
	}

	log.InfoLog.Printf("клиенту отправлены заголовки ответа")
	return nil
}
