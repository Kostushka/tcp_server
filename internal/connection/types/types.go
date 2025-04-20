// Package types - пакет со структурами для строки статуса и заголовков ответа
package types

// ResponseStatusLine - строка статуса ответа
type ResponseStatusLine struct {
	Version string
	Status  string
	Phrase  string
}

// StatusData - собираемые данные для строки статуса и заголовков ответа
type StatusData struct {
	Code        int
	Size        int64
	Name        string
	ContentType string
}

// ResponseData - сформированные данные для строки статуса и заголовков ответа
type ResponseData struct {
	Status      string
	Phrase      string
	Size        string
	Name        string
	ContentType string
}
