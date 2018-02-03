package bench

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	DataPath = "./data"
	DataSet  BenchDataSet
)

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

func prepareUserDataSet() {
	log.Println("datapath", DataPath)
	file, err := os.Open(filepath.Join(DataPath, "user.tsv"))
	must(err)
	defer file.Close()

	s := bufio.NewScanner(file)
	for i := 0; s.Scan(); i++ {
		line := strings.Split(s.Text(), "\t")
		displayName := line[0]
		addr := line[1]
		userName := strings.Split(addr, "@")[0]

		user := &AppUser{
			Name:        userName,
			Password:    userName + reverse(userName),
			DisplayName: displayName,
		}

		if i < 1000 {
			DataSet.Users = append(DataSet.Users, user)
		} else {
			DataSet.NewUsers = append(DataSet.NewUsers, user)
		}
	}
}

func prepareAvatarDataSet() {
	filePath := filepath.Join(DataPath, "default.png")
	b, err := ioutil.ReadFile(filePath)
	must(err)

	DataSet.DefaultAvatar = &Avatar{
		FilePath: filePath,
		SHA1:     fmt.Sprintf("%x", sha1.Sum(b)),
		MD5:      fmt.Sprintf("%x", md5.Sum(b)),
		Bytes:    b,
	}

	files, err := ioutil.ReadDir(filepath.Join(DataPath, "avatar"))
	must(err)

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".png") ||
			strings.HasSuffix(file.Name(), ".jpg") ||
			strings.HasSuffix(file.Name(), ".jpeg") ||
			strings.HasSuffix(file.Name(), ".gif") {

			path := filepath.Join(DataPath, "avatar", file.Name())
			b, err := ioutil.ReadFile(path)
			must(err)

			assert(file.Size() <= 1*1024*1024, "invalid avatar size ", path)

			DataSet.Avatars = append(DataSet.Avatars, &Avatar{
				FilePath: path,
				SHA1:     fmt.Sprintf("%x", sha1.Sum(b)),
				MD5:      fmt.Sprintf("%x", md5.Sum(b)),
				Bytes:    b,
			})
		}
	}

	largeFiles, err := ioutil.ReadDir(filepath.Join(DataPath, "large-avatar"))
	must(err)

	for _, file := range largeFiles {
		if strings.HasSuffix(file.Name(), ".png") ||
			strings.HasSuffix(file.Name(), ".jpg") ||
			strings.HasSuffix(file.Name(), ".jpeg") ||
			strings.HasSuffix(file.Name(), ".gif") {

			path := filepath.Join(DataPath, "large-avatar", file.Name())
			b, err := ioutil.ReadFile(path)
			must(err)

			assert(1*1024*1024 < file.Size(), "invalid large-avatar size", path)

			DataSet.LargeAvatars = append(DataSet.LargeAvatars, &Avatar{
				FilePath: path,
				SHA1:     fmt.Sprintf("%x", sha1.Sum(b)),
				MD5:      fmt.Sprintf("%x", md5.Sum(b)),
				Bytes:    b,
			})
		}
	}

	assert(0 < len(DataSet.Avatars), "no avatars")
	assert(0 < len(DataSet.LargeAvatars), "no large-avatars")
}

func prepareUserAvatar() {
	rnd := rand.New(rand.NewSource(3656))

	for _, user := range DataSet.Users {
		user.Avatar = DataSet.Avatars[rnd.Intn(len(DataSet.Avatars))]
	}
}

func prepareAdditionalAvatarImage() {
	defaultHash := "e4nwaAsqAt5od9"
	old := []byte(defaultHash)
	rnd := rand.New(rand.NewSource(3657))

	for i, j := 0, len(DataSet.Avatars); i < j; i++ {
		new := []byte(genSalt(len(old), rnd))
		b := bytes.Replace(DataSet.Avatars[i].Bytes, old, new, -1)
		avatar := &Avatar{
			FilePath: DataSet.Avatars[i].FilePath,
			SHA1:     fmt.Sprintf("%x", sha1.Sum(b)),
			MD5:      fmt.Sprintf("%x", md5.Sum(b)),
			Bytes:    b,
		}
		DataSet.Avatars = append(DataSet.Avatars, avatar)
	}
}

func prepareMessageDataSet() {
	rnd := rand.New(rand.NewSource(3656))

	texts := []string{}
	files, err := ioutil.ReadDir(filepath.Join(DataPath, "message"))
	must(err)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".txt") {
			file, err := os.Open(filepath.Join(DataPath, "message", file.Name()))
			must(err)
			defer file.Close()

			s := bufio.NewScanner(file)
			for i := 0; s.Scan(); i++ {
				// jsonデコードした時に一致しなくなる
				text := trim(strings.Replace(s.Text(), "　", "", -1))
				if !utf8.ValidString(text) {
					panic(text)
				}
				texts = append(texts, text)
			}
		}
	}

	DataSet.Texts = texts

	n := len(DataSet.Users)
	for i := 0; i < 10000; i++ {
		chanID := rnd.Intn(10) + 1
		user := DataSet.Users[i%n]

		n := len(DataSet.Texts)
		content := DataSet.Texts[rnd.Intn(n)] + DataSet.Texts[rnd.Intn(n)] + DataSet.Texts[rnd.Intn(n)]

		DataSet.Messages = append(DataSet.Messages, &MessageInfo{
			UserName:     user.Name,
			ChannelID:    chanID,
			Message:      content,
			SendComplete: true,
		})
	}
}

func prepareChannelDataSet() {
	for i := 0; i < 10; i++ {
		c := &Channel{
			ID:          i + 1,
			Name:        fmt.Sprintf("%dちゃんねる", i+1),
			Description: fmt.Sprintf("ここは %dちゃんねるです", i+1),
		}
		DataSet.Channels = append(DataSet.Channels, c)
	}
}

func PrepareDataSet() {
	prepareUserDataSet()
	prepareAvatarDataSet()
	prepareMessageDataSet()
	prepareUserAvatar()
	prepareChannelDataSet()
	prepareAdditionalAvatarImage()
}

var saltRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func genSalt(n int, rnd *rand.Rand) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = saltRunes[rnd.Intn(len(saltRunes))]
	}
	return string(b)
}

func fbadf(w io.Writer, f string, params ...interface{}) {
	for i, param := range params {
		switch v := param.(type) {
		case []byte:
			params[i] = fmt.Sprintf("_binary x'%s'", hex.EncodeToString(v))
		case *time.Time:
			params[i] = strconv.Quote(v.Format("2006-01-02 15:04:05"))
		case time.Time:
			params[i] = strconv.Quote(v.Format("2006-01-02 15:04:05"))
		default:
			params[i] = strconv.Quote(fmt.Sprint(v))
		}
	}
	fmt.Fprintf(w, f, params...)
}

func GenerateInitialDataSetSQL(outputPath string) {
	outFile, err := os.Create(outputPath)
	must(err)
	defer outFile.Close()

	w := gzip.NewWriter(outFile)
	defer w.Close()

	fbadf(w, "SET NAMES utf8mb4;")
	fbadf(w, "BEGIN;")

	rnd := rand.New(rand.NewSource(3656))
	baseTime, _ := time.Parse("2006-01-02", "2017-01-01")
	// channel
	for i, c := range DataSet.Channels {
		t := baseTime.Add(time.Duration(i+2) * time.Minute)
		fbadf(w, "INSERT INTO channel (id, name, description, updated_at, created_at) VALUES (%s, %s, %s, %s, %s);", c.ID, c.Name, c.Description, t, t)
	}

	// user
	for i, user := range DataSet.Users {
		salt := genSalt(20, rnd)
		passDigest := fmt.Sprintf("%x", sha1.Sum([]byte(salt+user.Password)))
		must(err)
		avatar_name := "default.png"
		if user.Avatar != nil {
			avatar_name = user.Avatar.SHA1 + filepath.Ext(user.Avatar.FilePath)
		}
		t := baseTime.AddDate(0, 1, 0).Add(time.Duration(3656*i) * time.Minute)
		fbadf(w, "INSERT INTO user (id, name, salt, password, display_name, avatar_icon, created_at) VALUES (%s, %s, %s, %s, %s, %s, %s);",
			i+1, user.Name, salt, passDigest, user.DisplayName, avatar_name, t)
	}

	// default image
	fbadf(w, "INSERT INTO image (id, name, data) VALUES (%s, %s, %s);", "1", "default.png", DataSet.DefaultAvatar.Bytes)

	// image
	for i, user := range DataSet.Users {
		if user.Avatar != nil {
			avatar_name := user.Avatar.SHA1 + filepath.Ext(user.Avatar.FilePath)
			avatar_data := user.Avatar.Bytes
			must(err)
			fbadf(w, "INSERT INTO image (id, name, data) VALUES (%s, %s, %s);", i+2, avatar_name, avatar_data)
		}
	}

	// message
	n := len(DataSet.Users)
	for i, msg := range DataSet.Messages {
		user := DataSet.Users[i%n]
		if msg.UserName != user.Name {
			panic("dataset index error")
		}

		chanID := msg.ChannelID
		userID := i%n + 1
		content := msg.Message
		t := baseTime.AddDate(0, 2, 0).Add(time.Duration(i*17) * time.Second)
		fbadf(w, "INSERT INTO message (channel_id, user_id, content, created_at) VALUES (%s, %s, %s, %s);", chanID, userID, content, t)
	}

	fbadf(w, "COMMIT;")
}
