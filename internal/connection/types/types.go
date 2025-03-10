package types

// строка статуса ответа
type ResponseStatusLine struct {
	Version string
	Status  string
	Phrase  string
}

// собираемые данные для строки статуса и заголовков ответа
type StatusData struct {
	Code        int
	Size        int64
	Name        string
	ContentType string
}

// сформированные данные для строки статуса и заголовков ответа
type ResponseData struct {
	Status      string
	Phrase      string
	Size        string
	Name        string
	ContentType string
}
