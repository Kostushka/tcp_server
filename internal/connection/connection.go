// Package connection - пакет с функциями, которые работают с клиентским соединением
package connection

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"

	"github.com/Kostushka/tcp_server/internal/connection/consts"
	"github.com/Kostushka/tcp_server/internal/connection/headerdata"
	"github.com/Kostushka/tcp_server/internal/connection/types"
	"github.com/Kostushka/tcp_server/internal/dir"
	"github.com/Kostushka/tcp_server/internal/file"
	"github.com/Kostushka/tcp_server/internal/log"
	"github.com/Kostushka/tcp_server/internal/querydata"
)

// Connection - структура с данными обрабатываемого соединения
type Connection struct {
	conn     *net.TCPConn
	rootPath string
	template *template.Template
}

// New - создать структуру с данными обрабатываемого соединения
func New(conn *net.TCPConn, rootPath string, template *template.Template) *Connection {
	return &Connection{
		conn:     conn,
		rootPath: rootPath,
		template: template,
	}
}

// ProcessingConn - обрабатываем клиентское соединение
func (c *Connection) ProcessingConn() {
	// закрыть клиентское соединение
	defer Close(c.conn, fmt.Sprintf("клиентское соединение %s закрыто", c.conn.RemoteAddr().String()))

	log.Infof("начинается работа с клиентским сокетом %s", c.conn.RemoteAddr().String())

	// получить данные запроса
	data, err := c.readConn()
	if err != nil {
		// по возвращении клиентским сокетом EOF или другой ошибки логируем ошибку,
		// так как не успели вычитать все данные, а клиент уже закрыл сокет
		log.Errorf(err)

		return
	}

	// создать структуру с данными запроса
	query, err := querydata.NewParseQueryData(data)
	if err != nil {
		// некорректный запрос
		if errors.Is(err, querydata.ErrInvalidHTTPReq) {
			err = c.sendResponseHeader(&types.StatusData{
				Code: consts.StatusBadRequest,
			}, err)
		}

		log.Errorf(err)

		return
	}

	// логируем клиентские заголовки
	log.Infof("распарсили данные, поступившие от клиента:")

	log.Infof("\"%v %v %v\" %v %v \"%v\"\n",
		query.Method(), query.Path(), query.Protocol(), c.conn.RemoteAddr().String(),
		query.Header("Host"), query.Header("User-Agent"))

	// работаем с путем до файла, взятым из строки запроса
	path := filepath.Join(c.rootPath, query.Path())

	// открываем запрашиваемый файл
	f, fi, err := c.openFile(path)
	if err != nil {
		log.Errorf(err)

		return
	}

	// закрыть файл
	defer Close(f, "")

	log.Infof("определен путь до файла: %q", path)

	// если файл - каталог, выводим его содержимое
	if fi.IsDir() {
		c.workingWithCatalog(query.Path())

		return
	}

	// отправить клиенту заголовки и файл
	err = c.SendFile(f, fi)
	if err != nil {
		log.Errorf(err)
	}
}

// прочитать из клиентского сокета данные в буфер
func (c *Connection) readConn() ([]byte, error) {
	// буфер для чтения из клиентского сокета
	buf := make([]byte, consts.BufSize)

	var data []byte
	// пока клиентский сокет пишет, читаем в буфер
	for {
		n, err := c.conn.Read(buf)
		// обрабатываем ошибку при чтении
		if err != nil {
			// не успели вычитать все данные, клиент закрыл сокет
			if errors.Is(err, io.EOF) {
				err = fmt.Errorf("клиент преждевременно закрыл соединение: %w", err)
			}

			return nil, err
		}
		// добавляем к итоговому срезу считанные в буфер данные
		data = append(data, buf[:n]...)

		// по возвращении клиентским сокетом пустой строки, перестаем читать
		if bytes.Contains(data, []byte("\r\n\r\n")) || bytes.Contains(data, []byte("\n\n")) {
			break
		}
	}

	return data, nil
}

// SendFile - отправить клиенту заголовки и файл
func (c *Connection) SendFile(f *os.File, fi os.FileInfo) error {
	// отправляем клиенту заголовки
	err := c.sendResponseHeader(&types.StatusData{
		Code: consts.StatusOK,
		Size: fi.Size(),
		Name: fi.Name(),
	}, nil)
	if err != nil {
		log.Errorf(err)

		return err
	}
	// отправить файл клиенту
	if err = file.Send(c.conn, f); err != nil {
		return fmt.Errorf("файл не был отправлен клиенту: %w", err)
	}

	return nil
}

// работаем с каталогом
func (c *Connection) workingWithCatalog(queryPath string) {
	log.Infof("файл %q: is a directory", filepath.Join(c.rootPath, queryPath))

	// выводим содержимое каталога
	buf, err := dir.ShowDir(c.rootPath, queryPath, c.template)
	if err != nil {
		// содержимое каталога не готово к отправке - 500
		err = c.sendInternalServerError(err)
		log.Errorf("содержимое каталога %q не готово к отправке: %v", filepath.Join(c.rootPath, queryPath), err)

		return
	}
	// отправляем заголовки
	err = c.sendResponseHeader(&types.StatusData{
		Code:        consts.StatusOK,
		Size:        int64(buf.Len()),
		ContentType: "text/html"}, nil)

	if err != nil {
		log.Errorf("не удалось отправить заголовки: %w", err)

		return
	}
	// записать содержимое буфера в клиентский сокет
	_, err = c.conn.Write(buf.Bytes())
	if err != nil {
		log.Errorf("содержимое каталога %q не готово к отправке: %v", filepath.Join(c.rootPath, queryPath), err)

		return
	}

	log.Infof("клиенту отправлен html файл с содержимым каталога %q", filepath.Join(c.rootPath, queryPath))
}

// получаем дескриптор открытого файла
func (c *Connection) openFile(path string) (*os.File, os.FileInfo, error) {
	var respdata *types.StatusData

	file, err := file.Open(path)
	if err != nil {
		switch {
		// файл должен быть, иначе 404
		case errors.Is(err, fs.ErrNotExist):
			// создаем ответ сервера для клиента: файл не найден
			respdata = &types.StatusData{
				Code: consts.StatusNotFound,
			}
		// файл должен быть доступен, иначе 403
		case errors.Is(err, fs.ErrPermission):
			// создаем ответ сервера для клиента: доступ к файлу запрещен
			respdata = &types.StatusData{
				Code: consts.StatusForbidden,
			}
		// файл не был открыт - 500
		default:
			// создаем ответ сервера для клиента: ошибка со стороны сервера
			respdata = &types.StatusData{
				Code: consts.StatusInternalServerError,
			}
		}
		// отправляем клиенту: ошибка при открытии файла
		err = c.sendResponseHeader(respdata, err)

		return nil, nil, err
	}
	// получить информацию о файле
	fi, err := file.Stat()
	if err != nil {
		// файл не отправлен - 500
		err = c.sendInternalServerError(err)

		return nil, nil, err
	}

	return file, fi, nil
}

// отправляем заголоки с ошибкой 500
func (c *Connection) sendInternalServerError(mainError error) error {
	err := c.sendResponseHeader(&types.StatusData{
		Code: consts.StatusInternalServerError,
	}, mainError)

	return err
}

// отправляем клиенту заголовки ответа
func (c *Connection) sendResponseHeader(statusData *types.StatusData, mainError error) error {
	// формируем данные для ответа
	data := headerdata.HeaderData{}
	data.SetResponseData(statusData)

	// отправляем заголовки клиенту
	if err := data.WriteResponseHeader(c.conn); err != nil {
		return fmt.Errorf("%w: %w", err, mainError)
	}

	return mainError
}

// Close - закрытие файла или соединения
func Close(c io.Closer, m string) {
	err := c.Close()
	if err != nil {
		log.Errorf(err)

		return
	}

	if m != "" {
		log.Infof(m)
	}
}
