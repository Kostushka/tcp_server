package querydata

import (
	"github.com/Kostushka/tcp_server/internal/querydata/parsequeryf"
)

// данные запроса
type queryData struct {
	data              []byte
	parsedQueryString *parsequeryf.QueryString
	parsedReqHeaders  parsequeryf.RequestHeaders
}

// создать структуру с данными запроса
func New(data []byte) (*queryData, error) {
	// распарсить строку запроса в структуру, заголовки - в map
	queryLine, reqhead, err := parsequeryf.NewParseQueryData(data)
	// отправить в клиентский сокет ошибку
	if err != nil {
		return nil, err
	}

	return &queryData{
		// записать данные запроса в буфер структуры
		data:              data,
		parsedQueryString: queryLine,
		parsedReqHeaders:  reqhead,
	}, nil
}

func (q *queryData) ParsedQueryString() *parsequeryf.QueryString {
	return q.parsedQueryString
}
func (q *queryData) ParsedReqHeaders() parsequeryf.RequestHeaders {
	return q.parsedReqHeaders
}
