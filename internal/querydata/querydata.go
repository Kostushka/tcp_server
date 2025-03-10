package querydata

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// данные запроса
type queryData struct {
	data              []byte
	ParsedQueryString *queryString
	ParsedReqHeaders  requestHeaders
}

// создать структуру с данными запроса
func New(data []byte) (*queryData, error) {
	// распарсить строку запроса в структуру, заголовки - в map
	queryLine, reqhead, err := NewParseQueryData(data)
	// отправить в клиентский сокет ошибку
	if err != nil {
		return nil, err
	}

	return &queryData{
		// записать данные запроса в буфер структуры
		data:              data,
		ParsedQueryString: queryLine,
		ParsedReqHeaders:  reqhead,
	}, nil
}

var ErrInvalidHttpReq = errors.New("incorrect request format: not HTTP")

// структура с содержимым строки запроса
type queryString struct {
	method   string
	path     string
	protocol string
}

func (q *queryString) Method() string {
	return q.method
}
func (q *queryString) Path() string {
	return q.path
}
func (q *queryString) Protocol() string {
	return q.protocol
}

// заголовки запроса
type requestHeaders map[string]string

// создаем структуру со строкой и map с заголовками запроса
func NewParseQueryData(data []byte) (*queryString, requestHeaders, error) {
	// структура с данными строки запроса HTTP-протокола
	q := queryString{}
	// map с заголовками запроса
	reqhead := make(requestHeaders, 5)

	// парсим строку запроса в структуру
	endQueryString, err := q.parseQueryString(data)
	if err != nil {
		return nil, nil, err
	}
	// парсим заголовки в map
	reqhead.parseRequestHeaders(data, endQueryString)
	return &q, reqhead, nil
}

// парсим строку запроса в структуру
func (q *queryString) parseQueryString(data []byte) (int, error) {
	// читаем строку из буфера
	var queryBuf strings.Builder
	var i int
	// в конце строки ожидаем либо \r\n, либо \n
	for i = 0; string(data[i]) != "\r" && string(data[i]) != "\n"; i++ {
		if err := queryBuf.WriteByte(data[i]); err != nil {
			return 0, fmt.Errorf("не удалось распарсить строку запроса: %v", err)
		}
	}
	// если в конце строки \r\n - пропускаем два символа для перехода на новую строку
	if string(data[i]) == "\r" {
		i++
	}
	i++

	// парсим строку запроса
	buf := strings.Split(trimQueryStringSpace(queryBuf.String()), " ")
	// в буфере должно быть 3 элемента: метод, путь, версия протокола
	if len(buf) < 3 {
		return 0, fmt.Errorf("не удалось распарсить строку запроса: %v", ErrInvalidHttpReq)
	}
	// декодируем path на случай, если он не в латинице
	convertPath, err := url.QueryUnescape(buf[1])
	if err != nil {
		return 0, fmt.Errorf("не удалось распарсить строку запроса: %v", err)
	}
	q.method = buf[0]
	q.path = convertPath
	q.protocol = buf[2]

	return i, nil
}

// парсим заголовки в map
func (r requestHeaders) parseRequestHeaders(data []byte, i int) {
	// парсим заголовки
	headerBuf := data[i:]
	buf := strings.Split(string(headerBuf), "\r\n")
	// если в конце строки не \r\n, а \n
	if len(buf) == 1 {
		buf = strings.Split(string(headerBuf), "\n")
	}
	// в конце после заголовков ожидаем пустую строку
	for j := 0; buf[j] != ""; j++ {
		sepIndex := strings.Index(buf[j], ":")
		r[buf[j][:sepIndex]] = strings.TrimSpace(buf[j][sepIndex+1:])
	}
}

// учитываем, что строка запроса может содержать более одного пробела, например:
// GET        /                HTTP/1.1
// удаляем лишние пробелы
func trimQueryStringSpace(str string) string {
	var prev byte
	var i int
	// если бы использовали конкатинацию строк, то кол-во перевыделений памяти было бы строго равно кол-во итераций (строку модифицировать нельзя)
	// с каждой итерацией объем копирования данных возрастал бы
	var res strings.Builder // для эффективного прирощения строки используем strings.Builder - по сути срез и append
	for ; i < len(str); i++ {
		if str[i] == prev && prev == ' ' {
			continue
		}
		prev = str[i]
		res.WriteByte(str[i])
	}
	return res.String()
}
