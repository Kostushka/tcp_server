package stringf

import (
	"github.com/Kostushka/tcp_server/internal/types"
	"net/url"
	"strings"
)

// парсим строку запроса в структуру, заголовки - в map
func ParseQueryString(data []byte) (*types.QueryString, types.RequestHeaders, error) {
	// структура с данными строки запроса HTTP-протокола
	q := types.QueryString{}
	// map с заголовками запроса
	reqhead := make(types.RequestHeaders, 5)

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
		return nil, nil, types.ErrInvalidHttpReq
	}
	// декодируем path на случай, если он не в латинице
	convertPath, err := url.QueryUnescape(buf[1])
	if err != nil {
		return nil, nil, err
	}
	q.Method = buf[0]
	q.Path = convertPath
	q.Protocol = buf[2]

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
