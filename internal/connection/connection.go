package connection

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"bytes"

	"github.com/Kostushka/tcp_server/internal/connection/constants"
	"github.com/Kostushka/tcp_server/internal/connection/headerdata"
	"github.com/Kostushka/tcp_server/internal/connection/types"
	"github.com/Kostushka/tcp_server/internal/dirf"
	"github.com/Kostushka/tcp_server/internal/filef"
	"github.com/Kostushka/tcp_server/internal/log"
	"github.com/Kostushka/tcp_server/internal/querydata"
	"github.com/Kostushka/tcp_server/internal/querydata/parsequeryf"
)

// структура с данными обрабатываемого соединения
type Connection struct {
	conn     *net.TCPConn
	rootPath string
	template *template.Template
}

// создать структуру с данными обрабатываемого соединения
func New(conn *net.TCPConn, rootPath string, template *template.Template) *Connection {
	return &Connection{
		conn:     conn,
		rootPath: rootPath,
		template: template,
	}
}

// прочитать из клиентского сокета данные в буфер
func (c *Connection) ReadConn() ([]byte, error) {
	// буфер для чтения из клиентского сокета
	buf := make([]byte, 4096)

	var data []byte
	// пока клиентский сокет пишет, читаем в буфер
	for {
		n, err := c.conn.Read(buf)
		// обрабатываем ошибку при чтении
		if err != nil {
			// не успели вычитать все данные, клиент закрыл сокет
			if err == io.EOF {
				err = fmt.Errorf("Клиент преждевременно закрыл соединение: %w", err)
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

// обрабатываем клиентское соединение
func (c *Connection) ProcessingConn() {
	// закрыть клиентское соединение
	defer Close(c.conn, fmt.Sprintf("клиентское соединение %s закрыто", c.conn.RemoteAddr().String()))

	log.InfoLog.Printf("начинается работа с клиентским сокетом %s", c.conn.RemoteAddr().String())

	// получить данные запроса
	data, err := c.ReadConn()
	if err != nil {
		// по возвращении клиентским сокетом EOF или другой ошибки логируем ошибку,
		// так как не успели вычитать все данные, а клиент уже закрыл сокет
		log.ErrorLog.Println(err)
		return
	}
	
	// создать структуру с данными запроса
	query, err := querydata.New(data)
	if err != nil {
		// некорректный запрос
		if errors.Is(err, parsequeryf.ErrInvalidHttpReq) {
			err = c.sendResponseHeader(&types.StatusData{
				Code: constants.StatusBadRequest,
			}, err)
			return
		}
		log.ErrorLog.Println(err)
		return
	}

	// логируем клиентские заголовки
	log.InfoLog.Println("распарсили данные, поступившие от клиента:")

	log.InfoLog.Printf("\"%v %v %v\" %v %v \"%v\"\n",
		query.ParsedQueryString().Method(), query.ParsedQueryString().Path(), query.ParsedQueryString().Protocol(), c.conn.RemoteAddr().String(),
		query.ParsedReqHeaders()["Host"], query.ParsedReqHeaders()["User-Agent"])

	// работаем с путем до файла, взятым из строки запроса
	path := filepath.Join(c.rootPath, query.ParsedQueryString().Path())

	// открываем запрашиваемый файл
	f, fi, err := c.openFile(path)
	if err != nil {
		log.ErrorLog.Println(err)
		return
	}

	// закрыть файл
	defer Close(f, "")

	log.InfoLog.Printf("определен путь до файла %s:", path)

	// если файл - каталог, выводим его содержимое
	if fi.IsDir() {
		c.workingWithCatalog(query.ParsedQueryString().Path())
		return
	}
	
	// отправить клиенту заголовки и файл
	err = c.SendFile(f, fi)
	if err != nil {
		log.ErrorLog.Println(err)
	}
}

func (c *Connection) SendFile(f *os.File, fi os.FileInfo) error {
	// отправляем клиенту заголовки
	err := c.sendResponseHeader(&types.StatusData{
		Code: constants.StatusOK,
		Size: fi.Size(),
		Name: fi.Name(),
	}, nil)
	if err != nil {
		log.ErrorLog.Println(err)
		return err
	}
	// отправить файл клиенту
	if err = filef.SendFile(c.conn, f, fi.Size()); err != nil {
		return fmt.Errorf("файл не был отправлен клиенту: %w", err)
	}
	return nil
}

// работаем с каталогом
func (c *Connection) workingWithCatalog(queryPath string) {
	log.InfoLog.Printf("файл %s: is a directory", filepath.Join(c.rootPath, queryPath))

	// выводим содержимое каталога
	buf, err := dirf.ShowDir(c.rootPath, queryPath, c.template)
	if err != nil {
		// содержимое каталога не готово к отправке - 500
		err = c.sendInternalServerError(err)
		log.ErrorLog.Printf("содержимое каталога %s не готово к отправке: %w", filepath.Join(c.rootPath, queryPath), err)
		return
	}
	// отправляем заголовки
	err = c.sendResponseHeader(&types.StatusData{
		Code:        constants.StatusOK,
		Size:        int64(buf.Len()),
		ContentType: "text/html"}, nil)

	if err != nil {
		log.ErrorLog.Printf("не удалось отправить заголовки: %w", err)
		return
	}
	// записать содержимое буфера в клиентский сокет
	_, err = c.conn.Write(buf.Bytes())
	if err != nil {
		log.ErrorLog.Printf("содержимое каталога %s не готово к отправке: %w", filepath.Join(c.rootPath, queryPath), err)
		return
	}
	log.InfoLog.Printf("клиенту отправлен html файл с содержимым %s", filepath.Join(c.rootPath, queryPath))
}

// получаем дескриптор открытого файла
func (c *Connection) openFile(path string) (*os.File, os.FileInfo, error) {
	var respdata *types.StatusData

	file, err := filef.OpenFile(c.conn, path)
	if err != nil {
		switch {
		// файл должен быть, иначе 404
		case errors.Is(err, fs.ErrNotExist):
			// создаем ответ сервера для клиента: файл не найден
			respdata = &types.StatusData{
				Code: constants.StatusNotFound,
			}
		// файл должен быть доступен, иначе 403
		case errors.Is(err, fs.ErrPermission):
			// создаем ответ сервера для клиента: доступ к файлу запрещен
			respdata = &types.StatusData{
				Code: constants.StatusForbidden,
			}
		// файл не был открыт - 500
		default:
			// создаем ответ сервера для клиента: ошибка со стороны сервера
			respdata = &types.StatusData{
				Code: constants.StatusInternalServerError,
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
		Code: constants.StatusInternalServerError,
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

// закрытие файла или соединения
func Close(c io.Closer, m string) {
	err := c.Close()
	if err != nil {
		log.ErrorLog.Println(err)
		return 
	}
	if m != "" {
		log.InfoLog.Println(m)
	}
}
