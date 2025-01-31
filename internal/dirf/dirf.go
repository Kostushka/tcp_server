package dirf

import (
	"bytes"
	"html/template"
	"io"
	"os"
	"path/filepath"

	"github.com/Kostushka/tcp_server/internal/log"
)

// отправляем клиенту содержимое каталога
func ShowDir(w io.Writer, rootPath, queryPath string, t *template.Template) (*bytes.Buffer, error) {
	type args struct {
		RootPath string
		DirName  string
		Files    []string
	}

	// получаем файлы, находящиеся в каталоге
	files, err := os.ReadDir(filepath.Join(rootPath, queryPath))
	if err != nil {
		log.ErrorLog.Println(err)
		return nil, err
	}

	// получаем имена файлов
	names := []string{}
	for _, v := range files {
		if v.Name()[0] == '.' {
			continue
		}
		names = append(names, v.Name())
	}

	// если запрос идет на корень, то оставляем переменную с путем запроса пустой,
	// так как по умолчанию в html шаблоне путь запроса до директории и имена содержащихся в ней файлов/каталогов разделяет слеш
	if queryPath == "/" {
		queryPath = ""
	}

	buf := new(bytes.Buffer)
	// применяем шаблон к структуре данных, пишем выходные данные в буфер
	err = t.Execute(buf, args{
		RootPath: filepath.Join(rootPath, queryPath),
		DirName:  queryPath,
		Files:    names,
	})
	if err != nil {
		return nil, err
	}

	return buf, err
}
