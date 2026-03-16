/*
main.go является точкой входа CLI-приложения SSTV. Файл поднимает HTTP-сервер,
читает адрес прослушивания из флага командной строки и подключает маршрутизатор
из внутреннего пакета server. Веб-ресурсы фронтенда вшиваются в бинарник через
go:embed, поэтому приложение запускается как единый self-contained исполняемый
файл без внешней директории со статикой.
*/
package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"

	"github.com/h1nezo/sstv/internal/server"
)

//go:embed web
var embeddedWeb embed.FS

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	webRoot, err := fs.Sub(embeddedWeb, "web")
	if err != nil {
		log.Fatal(err)
	}

	mux := server.New(http.FS(webRoot))

	log.Printf("sstv listening on http://localhost%s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
