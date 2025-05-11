// Package headerdata - пакет для работы с заголовками ответа
package headerdata

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Kostushka/tcp_server/internal/connection/consts"
	"github.com/Kostushka/tcp_server/internal/connection/types"
	"github.com/Kostushka/tcp_server/internal/log"
)

// заголовки ответа
type responseHeaders []string

// добавить заголовок
func (r *responseHeaders) Add(headerName, headerValue string) {
	*r = append(*r, headerName+": "+headerValue)
}

// сформировать буфер с заголовками
func (r *responseHeaders) ToBytes() []byte {
	var headers bytes.Buffer
	for _, v := range *r {
		_, err := headers.WriteString(v + "\n")
		if err != nil {
			log.Errorf("заголовок %q не был записан в буфер: %v", v, err)
		}

		log.Infof(v)
	}

	headers.WriteByte('\n')

	return headers.Bytes()
}

// HeaderData - структура с сформированными данными для строки статуса и заголовков ответа
type HeaderData struct {
	responseData *types.ResponseData
}

// ResponseData - возвращает структуру с сформированными данными для строки статуса и заголовков ответа
func (h *HeaderData) ResponseData() *types.ResponseData {
	return h.responseData
}

// SetResponseData - формируем данные заголовков для ответа клиенту
func (h *HeaderData) SetResponseData(data *types.StatusData) {
	// заполняем структуру данных для формирования ответа клиенту
	h.responseData = &types.ResponseData{
		Status:      strconv.Itoa(data.Code),
		Phrase:      http.StatusText(data.Code),
		Size:        strconv.FormatInt(data.Size, 10),
		Name:        data.Name,
		ContentType: data.ContentType,
	}
}

// WriteResponseHeader - формируем и отправляем клиенту заголовки ответа
func (h *HeaderData) WriteResponseHeader(w io.Writer) error {
	respStatus := types.ResponseStatusLine{
		Version: "HTTP/1.1",
		Status:  h.responseData.Status,
		Phrase:  h.responseData.Phrase,
	}
	respHeaders := responseHeaders{}

	respHeaders.Add("Server", "someserver/1.18.0")
	respHeaders.Add("Connection", "close")
	respHeaders.Add("Date", time.Now().Format(time.UnixDate))

	if h.responseData.Size != "" {
		respHeaders.Add("Size", h.responseData.Size)
	}
	// не пишем Content-Type, если ошибка
	if h.responseData.Status != strconv.Itoa(consts.StatusOK) {
		return writeToConn(w, respStatus, respHeaders)
	}

	if h.responseData.ContentType != "" {
		respHeaders.Add("Content-Type", h.responseData.ContentType)

		return writeToConn(w, respStatus, respHeaders)
	}

	// если у файла в названии есть расширение, пишем тип файла в заголовок Content-Type
	extIndex := strings.LastIndex(h.responseData.Name, ".")
	if extIndex == -1 {
		respHeaders.Add("Content-Type", "application/octet-stream")
		// пишем ответ в клиентский сокет
		return writeToConn(w, respStatus, respHeaders)
	}

	contentType := mime.TypeByExtension(h.responseData.Name[extIndex:])

	if contentType == "" {
		respHeaders.Add("Content-Type", "application/octet-stream")

		// пишем ответ в клиентский сокет
		return writeToConn(w, respStatus, respHeaders)
	}

	respHeaders.Add("Content-Type", contentType)

	// пишем ответ в клиентский сокет
	return writeToConn(w, respStatus, respHeaders)
}

// пишем заголовки в клиентский сокет
func writeToConn(w io.Writer, respStatus types.ResponseStatusLine, respHeaders responseHeaders) error {
	// сформировать статусную строку
	var statusString = respStatus.Version + " " + respStatus.Status + " " + respStatus.Phrase + "\n"

	// записать в клиентский сокет статусную строку
	_, err := w.Write([]byte(statusString))
	if err != nil {
		return fmt.Errorf("строка статуса не была записана в сокет: %w", err)
	}

	log.Infof("---")
	log.Infof("%s", statusString)

	// сформировать буфер с заголовками ответа
	headers := respHeaders.ToBytes()

	log.Infof("---")
	// записать в клиентский сокет заголовки ответа
	_, err = w.Write(headers)
	if err != nil {
		return fmt.Errorf("заголовки не были записаны в сокет: %w", err)
	}

	log.Infof("клиенту отправлены заголовки ответа")

	return nil
}
