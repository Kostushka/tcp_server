package main

import (
	"net"
	"log"
	"io"
	"strings"
	"fmt"
	"errors"
	"os"
	"io/fs"
)

type QueryString struct {
	method string
	path string
	protocol string
}

var path string = "/media/user/B4F1-F772"

func main() {

	// объявляем структуру с данными будущего сервера
	laddr := net.TCPAddr{
		IP: net.IPv4(127, 0, 0, 1),
		Port: 5000,
	}
	// получаем структуру с методами для работы с соединениями
	l, err := net.ListenTCP("tcp", &laddr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		// слушаем сокетные соединения (запросы)
		conn, err := l.AcceptTCP()
		if err != nil {
			log.Fatal(err)
		}
		// обрабатываем каждое клиентское соединение в отдельной горутине
		go func(conn *net.TCPConn) {
			// буфер для чтения из клиентского сокета
			buf := make([]byte, 4096)
			
			// структура с данными строки запроса HTTP-протокола
			q := QueryString{}
			
			// пока клиентский сокет пишет, читаем в буфер
			for  {
				_, err := conn.Read(buf)
				// по возвращении клиентским сокетом EOF, перестаем читать
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Fatal(err)
				}
				
				// парсим строку запроса в структуру
				parseQueryString(&q, string(buf))
				
				// открываем запрашиваемый файл
				f, err := os.Open(path + q.path)
				// файл должен быть
				if errors.Is(err, fs.ErrNotExist) {
					_, err := conn.Write([]byte(err.Error() + "\n"))
					if err != nil {
						log.Fatal(err)
					}
					continue
				} else if err != nil {
					log.Fatal(err)
				}

				// читаем файл
				fileBuf := make([]byte, 4096)
				for {
					_, err := f.Read(fileBuf)
					if err == io.EOF {
						break
					}
					if err != nil {
						log.Fatal(err)
					}
					fmt.Println(string(fileBuf))
				}
			}
						
			// закрыть клиентское соединение
			conn.Close()
		}(conn)
	}
}

func parseQueryString(q *QueryString, str string) {
	buf := strings.Split(str, " ")
	if len(buf) < 3 {
		log.Fatal(errors.New("not HTTP"))
	}
	q.method = buf[0]
	q.path = buf[1]
	q.protocol = buf[2]
}
