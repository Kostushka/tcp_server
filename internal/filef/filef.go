package filef

import (
	"errors"
	"fmt"
	"github.com/Kostushka/tcp_server/internal/netf"
	"github.com/Kostushka/tcp_server/internal/types"
	"io/fs"
	"net"
	"os"
	"path/filepath"
)

// работаем с путем до файла, взятым из строки запроса
func WorkingWithFile(conn *net.TCPConn, rootPath, queryPath string) error {
	path := filepath.Join(rootPath, queryPath)

	// открываем запрашиваемый файл
	f, err := openFile(conn, path)
	if err != nil {
		return err
	}
	// закрыть файл
	defer func() {
		err := f.Close()
		if err != nil {
			types.ErrorLog.Println(err)
		}
	}()

	types.InfoLog.Printf("определен путь до файла %s:", path)

	// получить информацию о файле
	fi, err := f.Stat()
	if err != nil {
		// файл не отправлен - 500
		err = netf.SendResponseHeader(conn, &types.StatusData{
			Code: types.StatusInternalServerError,
			Size: 0,
			Name: "",
		}, err)
		return fmt.Errorf("файл %s не готов к отправке: %w", path, err)
	}

	// если файл - каталог, выводим его содержимое
	if fi.IsDir() {
		types.InfoLog.Printf("файл %s: is a directory", path)
		// выводим содержимое каталога
		if err := netf.ShowDir(conn, rootPath, queryPath); err != nil {
			// содержимое каталога не отправлено - 500
			err = netf.SendResponseHeader(conn, &types.StatusData{
				Code: types.StatusInternalServerError,
				Size: 0,
				Name: "",
			}, err)
			return fmt.Errorf("содержимое каталога %s не готово к отправке: %w", path, err)
		}
		types.InfoLog.Printf("клиенту отправлен html файл с содержимым %s", path)
		return nil
	}

	// отправляем клиенту заголовки
	err = netf.SendResponseHeader(conn, &types.StatusData{
		Code: types.StatusOK,
		Size: fi.Size(),
		Name: fi.Name(),
	}, nil)

	if err != nil {
		return err
	}

	// отправить файл клиенту
	err = netf.SendFile(conn, f, fi.Size())
	if err != nil {
		return fmt.Errorf("файл не был отправлен клиенту: %w", err)
	}
	return nil
}

// открываем файл по пути
func openFile(w *net.TCPConn, path string) (*os.File, error) {
	// открываем запрашиваемый файл
	f, err := os.Open(path)

	if err != nil {
		// файл должен быть, иначе 404
		if errors.Is(err, fs.ErrNotExist) {
			// создаем ответ сервера для клиента: файл не найден
			err = netf.SendResponseHeader(w, &types.StatusData{
				Code: types.StatusNotFound,
				Size: 0,
				Name: ""}, err)
			return nil, err
		}
		// файл должен быть доступен, иначе 403
		if errors.Is(err, fs.ErrPermission) {
			// создаем ответ сервера для клиента: доступ к файлу запрещен
			err = netf.SendResponseHeader(w, &types.StatusData{
				Code: types.StatusForbidden,
				Size: 0,
				Name: ""}, err)
			return nil, err
		}
		// файл не был открыт - 500
		// создаем ответ сервера для клиента: ошибка со стороны сервера
		err = netf.SendResponseHeader(w, &types.StatusData{
			Code: types.StatusInternalServerError,
			Size: 0,
			Name: ""}, err)
		return nil, err
	}
	return f, nil
}
