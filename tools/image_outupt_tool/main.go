package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

type Image struct {
	ID   int
	Name string
	Data []byte
}

func main() {

	//database設定
	dbconf := "root@tcp(localhost:3306)/isubata?charset=utf8mb4"
	db, err := sql.Open("mysql", dbconf)
	defer db.Close()

	if err != nil {
		fmt.Println(err.Error())
	}

	err = db.Ping()

	if err != nil {
		log.Fatalln("データベース接続失敗")
	}

	rows, err := db.Query("SELECT id, name, data FROM image")
	if err != nil {
		log.Fatalln("select 失敗")
	}

	defer rows.Close()

	for rows.Next() {
		var img Image
		err := rows.Scan(&img.ID, &img.Name, &img.Data)
		if err != nil {
			log.Fatalln(err.Error())
		}

		reader := bytes.NewReader(img.Data)

		f, err := os.Create(fmt.Sprintf("./images/%s", img.Name))
		if err != nil {
			fmt.Errorf("err %v", err)
		}
		defer f.Close()

		io.Copy(f, reader)
	}
}
