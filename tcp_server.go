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
	"bufio"
)

type QueryString struct {
	method string
	path string
	protocol string
}

type RequestHeaders map[string]string

var path string = "/home/kostushka"

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
			
			// структура с данными строки запроса HTTP-протокола
			q := QueryString{}
			// map с заголовками запроса
			// reqhead := make(RequestHeaders)

			// получить данные запроса
			data := getRequestData(conn)
				
			fmt.Println("получили буфер", data)
			// парсим строку запроса в структуру
			parseQueryString(&q, data)
		
			// парсим заголовки запроса в map
			// parseRequestHeaders(reqhead, string(buf))
			
			// открываем запрашиваемый файл
			f, err := os.Open(path + q.path)
			
			// файл должен быть
			switch {
				case errors.Is(err, fs.ErrNotExist):
					_, err := conn.Write([]byte(err.Error() + "\n"))
					if err != nil {
						log.Fatal(err)
					}
				case err != nil:
					log.Fatal(err)
				default:
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
						conn.Write(fileBuf)
					}		
			}
			
			// закрыть клиентское соединение
			conn.Close()
		}(conn)
	}
}

func parseQueryString(q *QueryString, str string) {
	// читаем строку из буфера
	query, err := bufio.NewReader(strings.NewReader(str)).ReadString('\n')
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}
	
	buf := strings.Split(query, " ")
	if len(buf) < 3 {
		log.Fatal(errors.New("not HTTP"))
	}
	q.method = buf[0]
	q.path = buf[1]
	q.protocol = buf[2]

	query, err = bufio.NewReader(strings.NewReader(str)).ReadString('\n')
	fmt.Println(query)
}

// func parseRequestHeaders(reqhead map[string]string, str string) {
	// buf := strings.Split(str, " ")
	// fmt.Println(buf)
// }

func getRequestData(conn *net.TCPConn) string {
	// буфер для чтения из клиентского сокета
	buf := make([]byte, 4096)
	
	var data string
	// пока клиентский сокет пишет, читаем в буфер
	for  {
		_, err := conn.Read(buf)
		
		// по возвращении клиентским сокетом EOF или другой ошибки логируем ошибку, так как не успели вычитать все данные, а клиент уже закрыл сокет
		if err != nil {
			log.Fatal(err)
		}
		
		// по возвращении клиентским сокетом пустой строки, перестаем читать
		if strings.Contains(string(buf), "\n\n") {
			data += string(buf)
			break
		}
		
		// если размер данных больше, чем размер буфера
		if len(buf) >= cap(buf) {
			data += string(buf)
			buf = make([]byte, len(buf))
		}
	}
	return data
}
