package headerdata

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Kostushka/tcp_server/internal/connection/constants"
	"github.com/Kostushka/tcp_server/internal/connection/types"
	"github.com/Kostushka/tcp_server/internal/log"
)

// структура с сформированными данными для строки статуса и заголовков ответа
type HeaderData struct {
	responseData *types.ResponseData
}

func (h *HeaderData) ResponseData() *types.ResponseData {
	return h.responseData
}

// формируем данные заголовков для ответа клиенту
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

// формируем и отправляем клиенту заголовки ответа
func (h *HeaderData) WriteResponseHeader(w io.Writer) error {
	respStatus := types.ResponseStatusLine{
		Version: "HTTP/1.1",
		Status:  h.responseData.Status,
		Phrase:  h.responseData.Phrase,
	}
	respHeaders := types.ResponseHeaders{}

	respHeaders = append(respHeaders, "Server: someserver/1.18.0")
	respHeaders = append(respHeaders, "Connection: close")
	respHeaders = append(respHeaders, "Date: "+time.Now().Format(time.UnixDate))
	if h.responseData.Size != "" {
		respHeaders = append(respHeaders, "Size: "+h.responseData.Size)
	}
	// не пишем Content-Type, если ошибка
	if h.responseData.Status != strconv.Itoa(constants.StatusOK) {
		return writeToConn(w, respStatus, respHeaders)
	}
	if h.responseData.ContentType != "" {
		respHeaders = append(respHeaders, "Content-Type: "+h.responseData.ContentType)
		return writeToConn(w, respStatus, respHeaders)
	}

	// если у файла в названии есть расширение, пишем тип файла в заголовок Content-Type
	extIndex := strings.LastIndex(h.responseData.Name, ".")
	if extIndex == -1 {
		respHeaders = append(respHeaders, "Content-Type: application/octet-stream")
		// пишем ответ в клиентский сокет
		return writeToConn(w, respStatus, respHeaders)
	}
	ext := h.responseData.Name[extIndex:]
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
