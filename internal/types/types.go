package types

import (
	"errors"
	"log"
	"os"
)

type QueryString struct {
	Method   string
	Path     string
	Protocol string
}

type RequestHeaders map[string]string

type ResponseStatusLine struct {
	Version string
	Status  string
	Phrase  string
}

type ResponseHeaders []string

const (
	StatusOK                  = 200
	StatusBadRequest          = 400
	StatusForbidden           = 403
	StatusNotFound            = 404
	StatusInternalServerError = 500
)

var (
	ErrNoRootDir      = errors.New("Не указан путь до *корневого* каталога")
	ErrInvalidAddr    = errors.New("Указан некорректный IP-адрес")
	ErrInvalidHttpReq = errors.New("incorrect request format: not HTTP")
)

var InfoLog *log.Logger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
var ErrorLog *log.Logger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

var TemplateDirNames []byte

type StatusData struct {
	Code int
	Size int64
	Name string
}

type ResponseData struct {
	Status string
	Phrase string
	Size   string
	Name   string
}
