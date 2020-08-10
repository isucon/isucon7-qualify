package main

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"strings"
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
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}

	out, _ := os.Create(filename)
	defer out.Close()

	switch true {
	case strings.HasSuffix(filename, ".jpg"), strings.HasSuffix(filename, ".jpeg"):
		var opts jpeg.Options
		opts.Quality = 1
		err = jpeg.Encode(out, img, &opts)
	case strings.HasSuffix(filename, ".png"):
		err = png.Encode(out, img)
	case strings.HasSuffix(filename, ".gif"):
		err = gif.Encode(out, img, nil)
	default:
		log.Println("Other file type:", filename)
	}
	if err != nil {
		return err
	}

	return nil
}

type Image struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
	Data []byte `db:"data"`
}

func main() {
	db, err := init_db()
	if err != nil {
		log.Fatal(err)
	}

	image := Image{}
	rows, err := db.Queryx("SELECT * FROM image LIMIT 10")
	if err != nil {
		log.Println(err)
	}
	for rows.Next() {
		err = rows.StructScan(&image)
		if err != nil {
			log.Println(err)
		}
		log.Println(image.Name)

		err = writeFile(image.Name, image.Data)
		if err != nil {
			log.Println(err)
		}
	}
}
