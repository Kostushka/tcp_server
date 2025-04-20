// Package file - пакет с функциями для работы с файлами
package file

import (
	"errors"
	"io"
	"os"

	"github.com/Kostushka/tcp_server/internal/connection/consts"
	"github.com/Kostushka/tcp_server/internal/log"
)

// Open - открываем файл по пути
func Open(path string) (*os.File, error) {
	// открываем запрашиваемый файл
	f, err := os.Open(path) //nolint:gosec

	if err != nil {
		return nil, err
	}

	return f, nil
}

// Send - отправляем клиенту файл
func Send(w io.Writer, f *os.File) error {
	// читаем файл
	fileBuf := make([]byte, consts.BufSize)
	// если указать размер буфера больше размера файла, то буфер будет содержать в конце нули
	// curl и браузер не будут ориентироваться на заголовок Content-Type: - они скажут, что это бинарный файл:
	// Warning: Binary output can mess up your terminal. Use "--output -" to tell
	// Warning: curl to output it to your terminal anyway, or consider "--output
	// Warning: <FILE>" to save to a file.
	// также если задать буфер равным размеру файла fileBuf := make([]byte, fileSize),
	// то можем исчерпать оперативную память, если файл имеет большой размер
	// например, RAM - 1 Гб, а файл - 5 Гб; буфер лежит в RAM, выделить на него 5 Гб не получится
	for {
		n, err := f.Read(fileBuf)
		// читаем файл, пока не встретим EOF
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return err
		}
		// записать содержимое буфера в клиентский сокет
		_, err = w.Write(fileBuf[:n])
		if err != nil {
			return err
		}
	}

	log.Infof("клиенту отправлено тело ответа")

	return nil
}
