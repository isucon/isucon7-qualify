package main

import (
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func init_db() (*sqlx.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/isubata?parseTime=true&loc=Local&charset=utf8mb4",
		"isucon", "isucon", "localhost", "3306")

	log.Printf("Connecting to db: %q", dsn)
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	for {
		err := db.Ping()
		if err == nil {
			break
		}
		log.Println(err)
		time.Sleep(time.Second * 3)
	}

	db.SetMaxOpenConns(20)
	db.SetConnMaxLifetime(5 * time.Minute)
	log.Printf("Succeeded to connect db.")

	return db, nil
}

func writeFile(filename string, data []byte) error {
	out, _ := os.Create("icons/" + filename)
	defer out.Close()

	_, err = out.Write(data)
	return err
}

type Image struct {
	Name string `db:"name"`
	Data []byte `db:"data"`
}

func main() {
	db, err := init_db()
	if err != nil {
		log.Fatal(err)
	}

	image := Image{}
	rows, err := db.Queryx("SELECT DISTINCT name, data FROM image")
	if err != nil {
		log.Println(err)
	}
	for rows.Next() {
		err = rows.StructScan(&image)
		if err != nil {
			log.Println(err)
		}

		err = writeFile(image.Name, image.Data)
		if err != nil {
			log.Println(err)
		}
	}
}
