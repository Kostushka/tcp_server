package stringf

import (
	"errors"
	"net/url"
	"strings"
)

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

type requestHeaders map[string]string

var errInvalidHttpReq = errors.New("incorrect request format: not HTTP")

// парсим строку запроса в структуру, заголовки - в map
func ParseQueryString(data []byte) (*queryString, requestHeaders, error) {
	// структура с данными строки запроса HTTP-протокола
	q := queryString{}
	// map с заголовками запроса
	reqhead := make(requestHeaders, 5)

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
