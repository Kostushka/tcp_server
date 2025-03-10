package file

import (
	"io"
	"os"

	"github.com/Kostushka/tcp_server/internal/log"
)

// открываем файл по пути
func Open(w io.Writer, path string) (*os.File, error) {
	// открываем запрашиваемый файл
	f, err := os.Open(path)

	if err != nil {
		return nil, err
	}
	return f, nil
}

// отправляем клиенту файл
func Send(w io.Writer, f *os.File, fileSize int64) error {
	// читаем файл
	fileBuf := make([]byte, 4096) // если указать размер буфера больше размера файла, то буфер будет содержать в конце нули
	// curl и браузер не будут ориентироваться на заголовок Content-Type: - они скажут, что это бинарный файл:
	// Warning: Binary output can mess up your terminal. Use "--output -" to tell
	// Warning: curl to output it to your terminal anyway, or consider "--output
	// Warning: <FILE>" to save to a file.
	// также если задать буфер равным размеру файла fileBuf := make([]byte, fileSize) , то можем исчерпать оперативную память, если файл имеет большой размер
	// например, RAM - 1 Гб, а файл - 5 Гб; буфер лежит в RAM, выделить на него 5 Гб не получится
	for {
		n, err := f.Read(fileBuf)
		// читаем файл, пока не встретим EOF
		if err == io.EOF {
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
