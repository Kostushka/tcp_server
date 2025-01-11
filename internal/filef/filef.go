package filef

import (
	"errors"
	"github.com/Kostushka/tcp_server/internal/types"
	"io/fs"
	"net"
	"os"
)

// открываем файл по пути
func OpenFile(w *net.TCPConn, path string) (*os.File, *types.StatusData, error) {
	// открываем запрашиваемый файл
	f, err := os.Open(path)

	if err != nil {
		// файл должен быть, иначе 404
		if errors.Is(err, fs.ErrNotExist) {
			// создаем ответ сервера для клиента: файл не найден
			data := &types.StatusData{
				Code: types.StatusNotFound,
				Size: 0,
				Name: ""}
			return nil, data, err
		}
		// файл должен быть доступен, иначе 403
		if errors.Is(err, fs.ErrPermission) {
			// создаем ответ сервера для клиента: доступ к файлу запрещен
			data := &types.StatusData{
				Code: types.StatusForbidden,
				Size: 0,
				Name: ""}
			return nil, data, err
		}
		// файл не был открыт - 500
		// создаем ответ сервера для клиента: ошибка со стороны сервера
		data := &types.StatusData{
			Code: types.StatusInternalServerError,
			Size: 0,
			Name: ""}
		return nil, data, err
	}
	return f, nil, nil
}
