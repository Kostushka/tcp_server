package netf

import (
	"bytes"
	"fmt"
	"github.com/Kostushka/tcp_server/internal/types"
	"html/template"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// получаем данные запроса
func GetRequestData(conn *net.TCPConn) ([]byte, error) {
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

// отправляем клиенту заголовки ответа
func SendResponseHeader(w io.Writer, statusData *types.StatusData, mainError error) error {
	// формируем данные для ответа
	data := createResponseData(statusData)

	// отправляем заголовки клиенту
	if err := writeResponseHeader(w, data); err != nil {
		return fmt.Errorf("%w: %w", err, mainError)
	}
	return mainError
}

// формируем данные для ответа клиенту
func createResponseData(data *types.StatusData) *types.ResponseData {
	// заполняем структуру данных для формирования ответа клиенту
	return &types.ResponseData{
		Status: strconv.Itoa(data.Code),
		Phrase: http.StatusText(data.Code),
		Size:   strconv.FormatInt(data.Size, 10),
		Name:   data.Name,
	}
}

// отправляем клиенту содержимое каталога
func ShowDir(w io.Writer, rootPath, queryPath string) error {
	type args struct {
		RootPath string
		DirName  string
		Files    []string
	}

	// получаем файлы, находящиеся в каталоге
	files, err := os.ReadDir(filepath.Join(rootPath, queryPath))
	if err != nil {
		types.ErrorLog.Println(err)
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
	t, err := template.New("index").Parse(string(types.TemplateDirNames))
	if err != nil {
		types.ErrorLog.Println(err)
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
	err = SendResponseHeader(w, &types.StatusData{
		Code: types.StatusOK,
		Size: int64(buf.Len()),
		Name: ""}, nil)

	if err != nil {
		return err
	}

	// записать содержимое буфера в клиентский сокет
	_, err = w.Write(buf.Bytes())
	return err
}

// отправляем клиенту файл
func SendFile(w io.Writer, f *os.File, fileSize int64) error {
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
	types.InfoLog.Printf("клиенту отправлено тело ответа")
	return nil
}

// формируем и отправляем клиенту заголовки ответа
func writeResponseHeader(w io.Writer, data *types.ResponseData) error {
	respStatus := types.ResponseStatusLine{}
	respHeaders := types.ResponseHeaders{}

	respStatus.Version = "HTTP/1.1"
	respStatus.Status = data.Status
	respStatus.Phrase = data.Phrase

	respHeaders = append(respHeaders, "Server: someserver/1.18.0")
	respHeaders = append(respHeaders, "Connection: close")
	respHeaders = append(respHeaders, "Date: "+time.Now().Format(time.UnixDate))
	if data.Size != "" {
		respHeaders = append(respHeaders, "Size: "+data.Size)
	}

	// если у файла в названии есть расширение, пишем тип файла в заголовок Content-Type
	extIndex := strings.LastIndex(data.Name, ".")
	if extIndex == -1 {
		respHeaders = append(respHeaders, "Content-Type: charset=utf-8")
		// пишем ответ в клиентский сокет
		return writeToConn(w, respStatus, respHeaders)
	}
	ext := data.Name[extIndex:]
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

// пишем заголовки в клиентский сокет
func writeToConn(w io.Writer, respStatus types.ResponseStatusLine, respHeaders types.ResponseHeaders) error {
	// сформировать статусную строку
	var statusString strings.Builder

	fmt.Fprintf(&statusString, "%s %s %s\n", respStatus.Version, respStatus.Status, respStatus.Phrase)

	// записать в клиентский сокет статусную строку
	_, err := w.Write([]byte(statusString.String()))
	if err != nil {
		return err
	}
	types.InfoLog.Println("---")
	types.InfoLog.Printf("%s", statusString.String())

	// сформировать буфер с заголовками ответа
	var headers strings.Builder
	for _, v := range respHeaders {
		header := v + "\n"
		_, err := headers.WriteString(header)
		if err != nil {
			return err
		}
		types.InfoLog.Println(v)
	}
	types.InfoLog.Println("---")
	// записать в клиентский сокет заголовки ответа
	_, err = w.Write([]byte(headers.String() + "\n"))
	if err != nil {
		return err
	}

	types.InfoLog.Printf("клиенту отправлены заголовки ответа")
	return nil
}
