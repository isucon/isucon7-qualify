package bench

import (
	"bench/counter"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var (
	loginReg = regexp.MustCompile(`^/login$`)
)

func validateJsonMessage(state *State, chanID int, lastMessageID int, msgs []*JsonMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	if 100 < len(msgs) {
		return fatalErrorf("メッセージの件数が正しくありません")
	}
	if lastMessageID >= msgs[0].ID {
		return fatalErrorf("メッセージの順番が正しくありません")
	}
	for i := 0; i < len(msgs)-1; i++ {
		if msgs[i].ID >= msgs[i+1].ID {
			return fatalErrorf("メッセージの順番が正しくありません")
		}
	}
	for _, msg := range msgs {
		err := state.ValidateJsonMessage(chanID, msg)
		if err != nil {
			return fatalErrorf("メッセージの検証に失敗 %v", err)
		}
	}
	return nil
}

func checkHTML(f func(*http.Response, *goquery.Document) error) func(*http.Response, *bytes.Buffer) error {
	return func(res *http.Response, body *bytes.Buffer) error {
		doc, err := goquery.NewDocumentFromReader(body)
		if err != nil {
			return fatalErrorf("ページのHTMLがパースできませんでした")
		}
		return f(res, doc)
	}
}

func genPostProfileBody(dispName, fileName string, avatar []byte) (*bytes.Buffer, string, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	if avatar != nil {
		part, err := writer.CreateFormFile("avatar_icon", filepath.Base(fileName))
		if err != nil {
			return nil, "", err
		}

		for sum := 0; sum < len(avatar); {
			n, err := part.Write(avatar)
			if err != nil {
				return nil, "", err
			}
			sum += n
		}
	}

	if dispName != "" {
		err := writer.WriteField("display_name", dispName)
		if err != nil {
			return nil, "", err
		}
	}

	err := writer.Close()
	if err != nil {
		return nil, "", err
	}

	return body, writer.FormDataContentType(), err
}

func checkRedirectStatusCode(res *http.Response, body *bytes.Buffer) error {
	if res.StatusCode == 302 || res.StatusCode == 303 {
		return nil
	}
	return fmt.Errorf("期待していないステータスコード %d Expected 302 or 303", res.StatusCode)
}

func loadStaticFile(ctx context.Context, checker *Checker, path string) error {
	return checker.Play(ctx, &CheckAction{
		EnableCache:          true,
		SkipIfCacheAvailable: true,

		Method: "GET",
		Path:   path,
		CheckFunc: func(res *http.Response, body *bytes.Buffer) error {
			// Note. EnableCache時はPlay時に自動でReponseは最後まで読まれる
			if res.StatusCode == http.StatusOK {
				counter.IncKey("staticfile-200")
			} else if res.StatusCode == http.StatusNotModified {
				counter.IncKey("staticfile-304")
			} else {
				return fmt.Errorf("期待していないステータスコード %d", res.StatusCode)
			}
			return nil
		},
	})
}

func goLoadStaticFiles(ctx context.Context, checker *Checker, paths ...string) {
	for _, path := range paths {
		go loadStaticFile(ctx, checker, path)
	}
}

func goLoadAsset(ctx context.Context, checker *Checker) {
	var assetFiles []string
	for _, sf := range StaticFiles {
		assetFiles = append(assetFiles, sf.Path)
	}
	goLoadStaticFiles(ctx, checker, assetFiles...)
}

func goLoadAvatar(ctx context.Context, checker *Checker, paths ...string) {
	goLoadStaticFiles(ctx, checker, paths...)
}

func LoadRegister(ctx context.Context, state *State) error {
	user, checker, push := state.PopNewUser()
	if user == nil {
		return nil
	}

	err := checker.Play(ctx, &CheckAction{
		Method:    "POST",
		Path:      "/register",
		CheckFunc: checkRedirectStatusCode,
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
		Description: "新規ユーザが作成できること",
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:    "POST",
		Path:      "/login",
		CheckFunc: checkRedirectStatusCode,
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
		Description: "作成したユーザでログインできること",
	})
	if err != nil {
		return err
	}

	user.Avatar = DataSet.Avatars[rand.Intn(len(DataSet.Avatars))]

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("avatar_icon", filepath.Base(user.Avatar.FilePath))
	if err != nil {
		return err
	}

	for sum := 0; sum < len(user.Avatar.Bytes); {
		n, err := part.Write(user.Avatar.Bytes)
		if err != nil {
			return err
		}
		sum += n
	}

	err = writer.WriteField("display_name", user.DisplayName)
	if err != nil {
		return err
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:      "POST",
		Path:        "/profile",
		ContentType: writer.FormDataContentType(),
		PostBody:    body,
		CheckFunc:   checkRedirectStatusCode,
		Description: "プロフィールを変更できること",
	})
	if err != nil {
		return err
	}

	push()

	return nil
}

func LoadGetHistory(ctx context.Context, state *State) error {
	maxFollow := rand.Intn(3) + 1
	maxPage := 1

	chanID := state.GetRandomChannelID()
	user, checker, push := state.PopRandomUser()
	if user == nil {
		return nil
	}
	defer push()

	err := checker.Play(ctx, &CheckAction{
		Method:      "POST",
		Path:        "/login",
		CheckFunc:   checkRedirectStatusCode,
		Description: "ログインできること",
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               fmt.Sprintf("/history/%d", chanID),
		ExpectedStatusCode: 200,
		Description:        "チャットログが表示できること",
		CheckFunc: checkHTML(func(res *http.Response, doc *goquery.Document) error {
			var e error
			doc.Find(".pagination li").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
				n, err := strconv.Atoi(strings.TrimSpace(selection.Text()))
				if err != nil {
					if strings.Contains(selection.Text(), "«»") {
						e = fatalErrorf("pagination に数字でない文字が含まれています")
						return false
					}
				} else {
					if n != 1 && n != maxPage+1 {
						e = fatalErrorf("pagination の数字が連番ではありません")
						return false
					}
					maxPage = n
				}
				return true
			})
			return e
		}),
	})
	if err != nil {
		return err
	}

	perm := rand.Perm(maxPage)

	if maxFollow < maxPage {
		perm = perm[:maxFollow]
	}

	for _, page := range perm {
		err = checker.Play(ctx, &CheckAction{
			Method:             "GET",
			Path:               fmt.Sprintf("/history/%d?page=%d", chanID, page+1),
			ExpectedStatusCode: 200,
			Description:        "チャットログが表示できること",
			CheckFunc: checkHTML(func(res *http.Response, doc *goquery.Document) error {
				var e error
				m := 1
				doc.Find(".pagination li").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
					n, err := strconv.Atoi(strings.TrimSpace(selection.Text()))
					if err != nil {
						if strings.Contains(selection.Text(), "«»") {
							e = fatalErrorf("pagination に数字でない文字が含まれています")
							return false
						}
					} else {
						if n != 1 && n != m+1 {
							e = fatalErrorf("pagination の数字が連番ではありません")
							return false
						}
						m = n
					}
					return true
				})

				avatarPathSet := map[string]struct{}{}
				doc.Find(".message").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
					avatarPath, avatarPathFound := selection.Find(".avatar").First().Attr("src")
					userName := selection.Find("h5").First().Text()
					date := selection.Find(".message-date").First().Text()
					if !avatarPathFound {
						e = fatalErrorf("アバター画像のパスがありません")
						return false
					}
					if userName == "" {
						e = fatalErrorf("表示名が表示されていません")
						return false
					}
					if date == "" {
						e = fatalErrorf("発言時刻が表示されていません")
						return false
					}
					avatarPathSet[avatarPath] = struct{}{}
					return true
				})

				var avatarPaths []string
				for path := range avatarPathSet {
					avatarPaths = append(avatarPaths, path)
				}
				goLoadAvatar(ctx, checker, avatarPaths...)
				return e
			}),
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func LoadProfile(ctx context.Context, state *State) error {
	user, checker, push := state.PopRandomUser()
	if user == nil {
		return nil
	}
	defer push()

	act := &CheckAction{
		Method:      "POST",
		Path:        "/login",
		CheckFunc:   checkRedirectStatusCode,
		Description: "ログインできること",
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
	}

	err := checker.Play(ctx, act)
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               fmt.Sprintf("/profile/%s", user.Name),
		ExpectedStatusCode: 200,
		Description:        "プロフィールが表示できること",
	})
	if err != nil {
		return err
	}

	// TODO 表示名の更新 アバター画像の更新 両方の更新

	act = &CheckAction{
		Method:      "GET",
		Path:        "/logout",
		CheckFunc:   checkRedirectStatusCode,
		Description: "ログアウトできること",
	}

	err = checker.Play(ctx, act)
	if err != nil {
		return err
	}

	return nil
}

func LoadGetChannel(ctx context.Context, state *State) error {
	chanID := state.GetRandomChannelID()
	user, checker, push := state.PopRandomUser()
	if user == nil {
		return nil
	}
	defer push()

	err := checker.Play(ctx, &CheckAction{
		Method:      "POST",
		Path:        "/login",
		CheckFunc:   checkRedirectStatusCode,
		Description: "ログインできること",
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
	})
	if err != nil {
		return err
	}

	lastMessageID := 0
	err = checker.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               fmt.Sprintf("/channel/%d", chanID),
		ExpectedStatusCode: 200,
		Description:        "チャンネルが表示できること",
	})
	if err != nil {
		return err
	}

	goLoadAsset(ctx, checker)

	msgs := []*JsonMessage{}
	err = checker.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               fmt.Sprintf("/message?channel_id=%d&last_message_id=%d", chanID, lastMessageID),
		ExpectedStatusCode: 200,
		Description:        "メッセージが取得できること",
		CheckFunc: func(res *http.Response, body *bytes.Buffer) error {
			dec := json.NewDecoder(body)
			err := dec.Decode(&msgs)
			if err != nil {
				return fatalErrorf("Jsonのデコードに失敗 %v", err)
			}
			if len(msgs) == 0 {
				return nil
			}

			if lastMessageID >= msgs[0].ID {
				return fatalErrorf("メッセージの順番が正しくありません")
			}
			for i := 0; i < len(msgs)-1; i++ {
				if msgs[i].ID >= msgs[i+1].ID {
					return fatalErrorf("メッセージの順番が正しくありません")
				}
			}
			for _, msg := range msgs {
				err = state.ValidateJsonMessage(chanID, msg)
				if err != nil {
					return fatalErrorf("メッセージの検証に失敗 %s", err)
				}
			}

			lastMessageID = msgs[len(msgs)-1].ID
			counter.AddKey("get-message-count", len(msgs))
			return nil
		},
	})
	if err != nil {
		return err
	}

	avatarPathSet := map[string]struct{}{}
	for _, msg := range msgs {
		path := fmt.Sprintf("/icons/%s", msg.User.AvatarIcon)
		avatarPathSet[path] = struct{}{}
	}
	var avatarPaths []string
	for path := range avatarPathSet {
		avatarPaths = append(avatarPaths, path)
	}
	goLoadAvatar(ctx, checker, avatarPaths...)

	return nil
}

func LoadReadWriteUser(ctx context.Context, state *State, chanID int) error {
	user, checker, push := state.PopRandomUser()
	if user == nil {
		return nil
	}
	defer push()

	// ログイン
	err := checker.Play(ctx, &CheckAction{
		Method:    "POST",
		Path:      "/login",
		CheckFunc: checkRedirectStatusCode,
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
		Description: "ログインできること",
	})
	if err != nil {
		return err
	}

	lastMessageID := 0
	getMessage := func() error {
		return checker.Play(ctx, &CheckAction{
			Method:             "GET",
			Path:               fmt.Sprintf("/message?channel_id=%d&last_message_id=%d", chanID, lastMessageID),
			ExpectedStatusCode: 200,
			Description:        "メッセージが取得できること",
			CheckFunc: func(res *http.Response, body *bytes.Buffer) error {
				msgs := []*JsonMessage{}
				dec := json.NewDecoder(body)
				err := dec.Decode(&msgs)
				if err != nil {
					return fatalErrorf("Jsonのデコードに失敗 %v", err)
				}

				err = validateJsonMessage(state, chanID, lastMessageID, msgs)
				if err != nil {
					return err
				}

				if 0 < len(msgs) {
					lastMessageID = msgs[len(msgs)-1].ID
					counter.AddKey("get-message-count", len(msgs))
				}

				avatarPathSet := map[string]struct{}{}
				for _, msg := range msgs {
					path := fmt.Sprintf("/icons/%s", msg.User.AvatarIcon)
					avatarPathSet[path] = struct{}{}
				}
				var avatarPaths []string
				for path := range avatarPathSet {
					avatarPaths = append(avatarPaths, path)
				}
				goLoadAvatar(ctx, checker, avatarPaths...)
				return nil
			},
		})
	}

	// チャンネル表示
	err = checker.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               fmt.Sprintf("/channel/%d", chanID),
		ExpectedStatusCode: 200,
		Description:        "チャンネルが表示できること",
	})
	if err != nil {
		return err
	}

	goLoadAsset(ctx, checker)

	// 初回メッセージ取得
	err = getMessage()
	if err != nil {
		return err
	}

	unreads := []JsonUnreadInfo{}
	pollAct := &CheckAction{
		DisableSlowChecking: true,
		Method:              "GET",
		Path:                "/fetch",
		ExpectedStatusCode:  200,
		Description:         "新着情報が取得できること",
		CheckFunc: func(res *http.Response, body *bytes.Buffer) error {
			dec := json.NewDecoder(body)
			err := dec.Decode(&unreads)
			if err != nil {
				return fatalErrorf("Jsonのデコードに失敗 %v", err)
			}
			return nil
		},
	}

	// random sleep
	time.Sleep(time.Duration(rand.Intn(1000)) * time.Microsecond)

	writeTicker := time.NewTicker(500 * time.Millisecond)
	defer writeTicker.Stop()

	pollCh := make(chan error, 1)
	pollCh <- nil

	// pollして新着あれば取得
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-writeTicker.C:
			text := RandomText()
			complete, ok := state.AddSendMessage(&MessageInfo{
				ChannelID: chanID,
				UserName:  user.Name,
				Message:   text,
			})
			if !ok {
				continue
			}
			err := checker.Play(ctx, &CheckAction{
				Method:             "POST",
				Path:               "/message",
				ExpectedStatusCode: 204,
				PostData: map[string]string{
					"channel_id": fmt.Sprint(chanID),
					"message":    text,
				},
				Description: "メッセージが送信できること",
			})
			if err != nil {
				return err
			}
			complete()
		case err := <-pollCh:
			if err != nil {
				return err
			}

			go func() {
				err := checker.Play(ctx, pollAct)
				if err != nil {
					pollCh <- err
					return
				}

				for _, c := range unreads {
					if c.ChannelID == chanID && 0 < c.Unread {
						pollCh <- getMessage()
						return
					}
				}

				pollCh <- nil
			}()
		}
	}
}

func LoadReadOnlyUser(ctx context.Context, state *State, chanID int) error {
	user, checker, push := state.PopRandomUser()
	if user == nil {
		return nil
	}
	defer push()

	// ログイン
	err := checker.Play(ctx, &CheckAction{
		Method:    "POST",
		Path:      "/login",
		CheckFunc: checkRedirectStatusCode,
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
		Description: "ログインできること",
	})
	if err != nil {
		return err
	}

	lastMessageID := 0
	getMessage := func() error {
		return checker.Play(ctx, &CheckAction{
			Method:             "GET",
			Path:               fmt.Sprintf("/message?channel_id=%d&last_message_id=%d", chanID, lastMessageID),
			ExpectedStatusCode: 200,
			Description:        "メッセージが取得できること",
			CheckFunc: func(res *http.Response, body *bytes.Buffer) error {
				msgs := []*JsonMessage{}
				dec := json.NewDecoder(body)
				err := dec.Decode(&msgs)
				if err != nil {
					return fatalErrorf("Jsonのデコードに失敗 %v", err)
				}

				err = validateJsonMessage(state, chanID, lastMessageID, msgs)
				if err != nil {
					return err
				}

				if 0 < len(msgs) {
					lastMessageID = msgs[len(msgs)-1].ID
					counter.AddKey("get-message-count", len(msgs))
				}

				avatarPathSet := map[string]struct{}{}
				for _, msg := range msgs {
					path := fmt.Sprintf("/icons/%s", msg.User.AvatarIcon)
					avatarPathSet[path] = struct{}{}
				}
				var avatarPaths []string
				for path := range avatarPathSet {
					avatarPaths = append(avatarPaths, path)
				}
				goLoadAvatar(ctx, checker, avatarPaths...)

				return nil
			},
		})
	}

	// チャンネル表示
	err = checker.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               fmt.Sprintf("/channel/%d", chanID),
		ExpectedStatusCode: 200,
		Description:        "チャンネルが表示できること",
	})
	if err != nil {
		return err
	}

	goLoadAsset(ctx, checker)

	// 初回メッセージ取得
	err = getMessage()
	if err != nil {
		return err
	}

	unreads := []JsonUnreadInfo{}
	pollAct := &CheckAction{
		DisableSlowChecking: true,
		Method:              "GET",
		Path:                "/fetch",
		ExpectedStatusCode:  200,
		Description:         "新着情報が取得できること",
		CheckFunc: func(res *http.Response, body *bytes.Buffer) error {
			dec := json.NewDecoder(body)
			err := dec.Decode(&unreads)
			if err != nil {
				return fatalErrorf("Jsonのデコードに失敗 %v", err)
			}
			return nil
		},
	}

	// pollして新着あれば取得
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			err := checker.Play(ctx, pollAct)
			if err != nil {
				return err
			}

			for _, c := range unreads {
				if c.ChannelID == chanID && 0 < c.Unread {
					err = getMessage()
					if err != nil {
						return err
					}
					break
				}
			}
		}
	}
}

// Validation

func CheckNotLoggedInUser(ctx context.Context, state *State) error {
	user, checker, push := state.PopRandomUser()
	if user == nil {
		return nil
	}
	defer push()

	checker.ResetCookie()

	err := checker.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               "/",
		ExpectedStatusCode: 200,
		Description:        "ページが表示されること",
		CheckFunc: checkHTML(func(res *http.Response, doc *goquery.Document) error {
			if trim(doc.Find("body > nav > a").Text()) != "Isubata" {
				return fatalErrorf("ブランド名が適切に表示されていません")
			}

			if trim(doc.Find("body > div > div > main > h1").Text()) != "ようこそ Isubata へ。" {
				return fatalErrorf("見出しが適切に表示されていません")
			}

			if trim(doc.Find("body > div > div > main > p:nth-child(3) > a").Text()) != "ログイン" {
				return fatalErrorf("ログインページへのリンクが表示されていません")
			}

			if trim(doc.Find("body > div > div > main > p:nth-child(4) > a").Text()) != "新規登録" {
				return fatalErrorf("新規登録ページへのリンクが表示されていません")
			}

			if trim(doc.Find("#navbarsExampleDefault > ul > li:nth-child(1) > a").Text()) != "新規登録" {
				return fatalErrorf("ヘッダの新規登録ページへのリンクが適切に表示されていません")
			}

			if trim(doc.Find("#navbarsExampleDefault > ul > li:nth-child(2) > a").Text()) != "ログイン" {
				return fatalErrorf("ヘッダのログインページへのリンクが適切に表示されていません")
			}
			return nil
		}),
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               "/register",
		ExpectedStatusCode: 200,
		Description:        "ページが表示されること",
		CheckFunc: checkHTML(func(res *http.Response, doc *goquery.Document) error {
			if doc.Find("body > nav > a").Text() != "Isubata" {
				return fatalErrorf("ブランド名が適切に表示されていません")
			}

			if doc.Find("#inputname").Size() != 1 {
				return fatalErrorf("入力フォーム適切に表示されていません")
			}

			if doc.Find("#inputpass").Size() != 1 {
				return fatalErrorf("入力フォーム適切に表示されていません")
			}

			if doc.Find("body > div > div > main > form > button").Text() != "登録" {
				return fatalErrorf("登録ボタンが適切に表示されていません")
			}

			return nil
		}),
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               "/login",
		ExpectedStatusCode: 200,
		Description:        "ページが表示されること",
		CheckFunc: checkHTML(func(res *http.Response, doc *goquery.Document) error {
			if doc.Find("body > nav > a").Text() != "Isubata" {
				return fatalErrorf("ブランド名が適切に表示されていません")
			}

			if doc.Find("#inputname").Size() != 1 {
				return fatalErrorf("入力フォーム適切に表示されていません")
			}

			if doc.Find("#inputpass").Size() != 1 {
				return fatalErrorf("入力フォーム適切に表示されていません")
			}

			if doc.Find("body > div > div > main > form > button").Text() != "ログイン" {
				return fatalErrorf("ログインボタンが適切に表示されていません")
			}

			return nil
		}),
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:           "GET",
		Path:             "/channel/1",
		CheckFunc:        checkRedirectStatusCode,
		ExpectedLocation: loginReg,
		Description:      "ログインページにリダイレクトされること",
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:           "GET",
		Path:             fmt.Sprintf("/profile/%s", user.Name),
		CheckFunc:        checkRedirectStatusCode,
		ExpectedLocation: loginReg,
		Description:      "ログインページにリダイレクトされること",
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:           "GET",
		Path:             "/add_channel",
		CheckFunc:        checkRedirectStatusCode,
		ExpectedLocation: loginReg,
		Description:      "ログインページにリダイレクトされること",
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:           "GET",
		Path:             "/history/1",
		CheckFunc:        checkRedirectStatusCode,
		ExpectedLocation: loginReg,
		Description:      "ログインページにリダイレクトされること",
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               fmt.Sprintf("/message?channel_id=%d&last_message_id=%d", 1, 123),
		ExpectedStatusCode: 403,
		Description:        "非ログインユーザはメッセージが取得できないこと",
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		DisableSlowChecking: true,
		Method:              "GET",
		Path:                "/fetch",
		ExpectedStatusCode:  403,
		Description:         "非ログインユーザは新着情報が取得できないこと",
	})
	if err != nil {
		return err
	}

	body, ctype, err := genPostProfileBody(user.Name, user.Avatar.FilePath, user.Avatar.Bytes)
	if err != nil {
		return err
	}
	err = checker.Play(ctx, &CheckAction{
		Method:           "POST",
		Path:             "/profile",
		ContentType:      ctype,
		PostBody:         body,
		CheckFunc:        checkRedirectStatusCode,
		ExpectedLocation: loginReg,
		Description:      "ログインページにリダイレクトされること",
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:           "POST",
		Path:             "/add_channel",
		CheckFunc:        checkRedirectStatusCode,
		ExpectedLocation: loginReg,
		Description:      "ログインページにリダイレクトされること",
		PostData: map[string]string{
			"name":        "ダミー部屋名",
			"description": "ダミー部屋詳細",
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func checkAvatarImage(expectedMD5 string) func(*http.Response, *bytes.Buffer) error {
	return func(res *http.Response, body *bytes.Buffer) error {
		hasher := md5.New()
		io.Copy(hasher, body)
		md5 := hex.EncodeToString(hasher.Sum(nil))

		if md5 != expectedMD5 {
			return fatalErrorf("画像データが正しくありません")
		}

		return nil
	}
}

func CheckStaticFiles(ctx context.Context, state *State) error {
	user, checker, push := state.PopRandomUser()
	if user == nil {
		return nil
	}
	defer push()

	for _, staticFile := range StaticFiles {
		sf := staticFile
		err := checker.Play(ctx, &CheckAction{
			Method:             "GET",
			Path:               sf.Path,
			ExpectedStatusCode: 200,
			Description:        "静的ファイルが取得できること",
			CheckFunc: func(res *http.Response, body *bytes.Buffer) error {
				hasher := md5.New()
				_, err := io.Copy(hasher, body)
				if err != nil {
					return fatalErrorf("レスポンスボディの取得に失敗 %v", err)
				}
				hash := hex.EncodeToString(hasher.Sum(nil))
				if hash != sf.Hash {
					return fatalErrorf("静的ファイルの内容が正しくありません")
				}
				return nil
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func CheckLogin(ctx context.Context, state *State) error {
	user, checker, push := state.PopRandomUser()
	if user == nil {
		return nil
	}
	defer push()

	err := checker.Play(ctx, &CheckAction{
		Method:    "POST",
		Path:      "/login",
		CheckFunc: checkRedirectStatusCode,
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
		Description: "存在するユーザでログインできること",
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:      "GET",
		Path:        "/logout",
		CheckFunc:   checkRedirectStatusCode,
		Description: "ログアウトできること",
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:             "POST",
		Path:               "/login",
		ExpectedStatusCode: 403,
		PostData: map[string]string{
			"name":     RandomAlphabetString(32),
			"password": RandomAlphabetString(32),
		},
		Description: "存在しないユーザでログインできないこと",
	})
	if err != nil {
		return err
	}

	return nil
}

func CheckGetProfileFail(ctx context.Context, state *State) error {
	user, checker, push := state.PopRandomUser()
	if user == nil {
		return nil
	}
	defer push()

	err := checker.Play(ctx, &CheckAction{
		Method:    "POST",
		Path:      "/login",
		CheckFunc: checkRedirectStatusCode,
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
		Description: "存在するユーザでログインできること",
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               fmt.Sprintf("/profile/%s", RandomAlphabetString(32)),
		ExpectedStatusCode: 404,
		Description:        "存在しないユーザのプロフィールはNotFoundが返ること",
	})
	if err != nil {
		return err
	}

	return nil
}

func CheckRegisterProfile(ctx context.Context, state *State) error {
	user, checker, push := state.PopNewUser()
	if user == nil {
		return nil
	}

	user2, checker2, push2 := state.PopRandomUser()
	if user2 == nil {
		return nil
	}
	defer push2()

	err := checker.Play(ctx, &CheckAction{
		Method:             "POST",
		Path:               "/login",
		ExpectedStatusCode: 403,
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
		Description: "登録前のユーザでログインできないこと",
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:    "POST",
		Path:      "/register",
		CheckFunc: checkRedirectStatusCode,
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
		Description: "ユーザが作成できること",
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:             "POST",
		Path:               "/register",
		ExpectedStatusCode: 409,
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password + "Hoge",
		},
		Description: "登録済のユーザ名が使えないこと",
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:    "POST",
		Path:      "/login",
		CheckFunc: checkRedirectStatusCode,
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
		Description: "作成したユーザでログインできること",
	})
	if err != nil {
		return err
	}

	err = checker2.Play(ctx, &CheckAction{
		Method:    "POST",
		Path:      "/login",
		CheckFunc: checkRedirectStatusCode,
		PostData: map[string]string{
			"name":     user2.Name,
			"password": user2.Password,
		},
		Description: "ログインできること",
	})
	if err != nil {
		return err
	}

	checkSelfProfile := func(name, dispName string, avatar *Avatar) error {
		return checker.Play(ctx, &CheckAction{
			Method:             "GET",
			Path:               fmt.Sprintf("/profile/%s", name),
			ExpectedStatusCode: 200,
			Description:        "プロフィールが表示できること",
			CheckFunc: checkHTML(func(res *http.Response, doc *goquery.Document) error {
				if doc.Find("body > div > div > main > form > div > div:nth-child(2) > p").Text() != name {
					return fatalErrorf("自分のプロフィール画面にユーザ名が正しく表示されていません ユーザ名 %s", name)
				}

				// デフォルトではユーザ名 = 表示名
				if doc.Find("body > div > div > main > form > div > div:nth-child(4) > input").AttrOr("value", "") != dispName {
					return fatalErrorf("自分のプロフィール画面に表示名が正しく表示されていません ユーザ名 %s", name)
				}

				url, exists := doc.Find("body > div > div > main > form > div > div:nth-child(8) > img").Attr("src")
				if !exists {
					return fatalErrorf("自分のプロフィール画面に画像が正しく表示されていません ユーザ名 %s", name)
				}

				err := checker.Play(ctx, &CheckAction{
					Method:             "GET",
					Path:               url,
					ExpectedStatusCode: 200,
					Description:        "正しいアバターが取得できること",
					CheckFunc:          checkAvatarImage(avatar.MD5),
				})
				if err != nil {
					return err
				}

				return nil
			}),
		})
	}

	checkOtherProfile := func(name, dispName string, avatar *Avatar) error {
		return checker2.Play(ctx, &CheckAction{
			Method:             "GET",
			Path:               fmt.Sprintf("/profile/%s", name),
			ExpectedStatusCode: 200,
			Description:        "他人のプロフィールが表示できること",
			CheckFunc: checkHTML(func(res *http.Response, doc *goquery.Document) error {
				if doc.Find("body > div > div > main > div > div:nth-child(2) > p").Text() != name {
					return fatalErrorf("他人のプロフィール画面にユーザ名が正しく表示されていません ユーザ名 %s", name)
				}

				if doc.Find("body > div > div > main > div > div:nth-child(4) > p").Text() != dispName {
					return fatalErrorf("他人のプロフィール画面に表示名が正しく表示されていません ユーザ名 %s", dispName)
				}

				url, exists := doc.Find("body > div > div > main > div > div:nth-child(6) > img").Attr("src")
				if !exists {
					return fatalErrorf("他人のプロフィール画面に画像が正しく表示されていません ユーザ名 %s", name)
				}

				err := checker.Play(ctx, &CheckAction{
					Method:             "GET",
					Path:               url,
					ExpectedStatusCode: 200,
					Description:        "正しいアバターが取得できること",
					CheckFunc:          checkAvatarImage(avatar.MD5),
				})
				if err != nil {
					return err
				}

				return nil
			}),
		})
	}

	// Note. デフォルトでは ユーザ名 = 表示名
	err = checkSelfProfile(user.Name, user.Name, DataSet.DefaultAvatar)
	if err != nil {
		return err
	}

	err = checkOtherProfile(user.Name, user.Name, DataSet.DefaultAvatar)
	if err != nil {
		return err
	}

	largeAvatar := DataSet.LargeAvatars[rand.Intn(len(DataSet.LargeAvatars))]
	body, ctype, err := genPostProfileBody("", largeAvatar.FilePath, largeAvatar.Bytes)
	err = checker.Play(ctx, &CheckAction{
		DisableSlowChecking: true,
		Method:              "POST",
		Path:                "/profile",
		ContentType:         ctype,
		PostBody:            body,
		ExpectedStatusCode:  400,
		Description:         "大きい画像に変更できないこと",
	})
	if err != nil {
		return err
	}

	user.Avatar = DataSet.Avatars[rand.Intn(len(DataSet.Avatars))]
	body, ctype, err = genPostProfileBody(user.DisplayName, user.Avatar.FilePath+".bin", user.Avatar.Bytes)
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:             "POST",
		Path:               "/profile",
		ContentType:        ctype,
		PostBody:           body,
		ExpectedStatusCode: 400,
		Description:        "画像以外の拡張子のファイルに変更できないこと",
	})
	if err != nil {
		return err
	}

	body, ctype, err = genPostProfileBody(user.DisplayName, user.Avatar.FilePath, user.Avatar.Bytes)
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:      "POST",
		Path:        "/profile",
		ContentType: ctype,
		PostBody:    body,
		CheckFunc:   checkRedirectStatusCode,
		Description: "プロフィールを変更できること",
	})
	if err != nil {
		return err
	}

	err = checkSelfProfile(user.Name, user.DisplayName, user.Avatar)
	if err != nil {
		return err
	}

	err = checkOtherProfile(user.Name, user.DisplayName, user.Avatar)
	if err != nil {
		return err
	}

	push()

	return nil
}

func CheckGetChannel(ctx context.Context, state *State) error {
	chanID := state.GetRandomChannelID()
	user, checker, push := state.PopRandomUser()
	if user == nil {
		return nil
	}
	defer push()

	err := checker.Play(ctx, &CheckAction{
		Method:      "POST",
		Path:        "/login",
		CheckFunc:   checkRedirectStatusCode,
		Description: "ログインできること",
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               fmt.Sprintf("/channel/%d", chanID),
		ExpectedStatusCode: 200,
		Description:        "チャンネルが表示できること",
		CheckFunc: checkHTML(func(res *http.Response, doc *goquery.Document) error {
			if trim(doc.Find("#navbarsExampleDefault > ul > li:nth-child(3) > a").Text()) != user.DisplayName {
				return fatalErrorf("ヘッダに表示名が正しく表示されていません")
			}

			var e error
			doc.Find("body > div > div > nav > ul > li").EachWithBreak(func(_ int, s *goquery.Selection) bool {
				href, ok := s.Find("a").Attr("href")
				if !ok {
					e = fatalErrorf("チャンネルのリンクが適切に設定されていません")
					return false
				}

				ar := strings.Split(href, "/")
				chanID, err := strconv.Atoi(ar[len(ar)-1])
				if err != nil {
					e = fatalErrorf("チャンネルのリンクが適切に設定されていません")
					return false
				}

				channel, ok := state.GetChannel(chanID)
				if !ok {
					e = fatalErrorf("存在しないはずのチャンネルが表示されています チャンネルID %v チャンネル名 %v", chanID, trim(s.Text()))
					return false
				}

				if trim(s.Text()) != channel.Name {
					e = fatalErrorf("チャンネル名が適切に表示されていません")
					return false
				}

				return true
			})
			return e
		}),
	})
	if err != nil {
		return err
	}

	return nil
}

func CheckPostAddChannel(ctx context.Context, state *State) (int, error) {
	chanID := -1
	user, checker, push := state.PopRandomUser()
	if user == nil {
		return chanID, nil
	}
	defer push()

	err := checker.Play(ctx, &CheckAction{
		Method:      "POST",
		Path:        "/login",
		CheckFunc:   checkRedirectStatusCode,
		Description: "ログインできること",
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
	})
	if err != nil {
		return chanID, err
	}

	c := &Channel{
		Name:        user.DisplayName + "の部屋",
		Description: user.DisplayName + "の部屋です",
	}

	err = checker.Play(ctx, &CheckAction{
		Method:      "POST",
		Path:        "/add_channel",
		Description: "チャンネル追加ができること",
		PostData: map[string]string{
			"name":        c.Name,
			"description": c.Description,
		},
		CheckFunc: func(res *http.Response, body *bytes.Buffer) error {
			err := checkRedirectStatusCode(res, body)
			if err != nil {
				return err
			}

			loc := res.Header.Get("Location")

			if !strings.Contains(loc, "/") {
				return fatalErrorf("リダイレクトURLが適切に設定されていません")
			}

			s := strings.Split(loc, "/")
			chanID, err = strconv.Atoi(s[len(s)-1])
			if err != nil {
				return fatalErrorf("リダイレクトURLが適切に設定されていません")
			}

			state.AddChannel(chanID, c)

			return nil
		},
	})
	if err != nil {
		return chanID, err
	}

	return chanID, nil
}

func CheckPostAddChannelFail(ctx context.Context, state *State) error {
	user, checker, push := state.PopRandomUser()
	if user == nil {
		return nil
	}
	defer push()

	err := checker.Play(ctx, &CheckAction{
		Method:      "POST",
		Path:        "/login",
		CheckFunc:   checkRedirectStatusCode,
		Description: "ログインできること",
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
	})
	if err != nil {
		return err
	}

	name := "チェックの部屋"
	description := "チェックの部屋です"

	err = checker.Play(ctx, &CheckAction{
		Method:             "POST",
		Path:               "/add_channel",
		ExpectedStatusCode: 400,
		Description:        "descriptionなしでチャンネル追加ができないこと",
		PostData: map[string]string{
			"name": name,
		},
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:             "POST",
		Path:               "/add_channel",
		ExpectedStatusCode: 400,
		Description:        "nameなしでチャンネル追加ができないこと",
		PostData: map[string]string{
			"description": description,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func CheckGetAddChannel(ctx context.Context, state *State) error {
	user, checker, push := state.PopRandomUser()
	if user == nil {
		return nil
	}
	defer push()

	err := checker.Play(ctx, &CheckAction{
		Method:      "POST",
		Path:        "/login",
		CheckFunc:   checkRedirectStatusCode,
		Description: "ログインできること",
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               "/add_channel",
		ExpectedStatusCode: 200,
		Description:        "チャンネル追加ページが表示されること",
		CheckFunc: checkHTML(func(res *http.Response, doc *goquery.Document) error {
			if doc.Find("#inputname").Size() != 1 {
				return fatalErrorf("入力フォーム適切に表示されていません")
			}

			if doc.Find("#inputdescription").Size() != 1 {
				return fatalErrorf("入力フォーム適切に表示されていません")
			}

			if doc.Find("body > div > div > main > form > button").Text() != "登録" {
				return fatalErrorf("登録ボタンが適切に表示されていません")
			}
			return nil
		}),
	})
	if err != nil {
		return err
	}

	return nil
}

func validateHistoryPagination(doc *goquery.Document) (int, error) {
	var e error
	page := 0
	arrowR := false
	arrowL := false

	doc.Find(".pagination li").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		text := strings.TrimSpace(selection.Text())
		n, err := strconv.Atoi(text)
		if err != nil {
			if text == "«" {
				arrowL = true
			} else if text == "»" {
				arrowR = true
			} else {
				e = fatalErrorf("pagination に数字でない文字が含まれています")
				return false
			}
		} else {
			if n != 1 && n != page+1 {
				e = fatalErrorf("pagination の数字が連番ではありません")
				return false
			}
			page = n
		}

		return true
	})

	if e != nil {
		return 0, e
	}

	// Note. メッセージ0件の場合でも1ページ目を表示していることになる
	if page == 0 {
		return 0, fatalErrorf("pagination にページ数が表示されていません")
	}

	if page == 1 && (arrowR || arrowL) {
		e = fatalErrorf("pagination に不要な矢印が表示されています")
	}

	return page, e
}

type PageFollowMode int

const (
	FollowModeRandom PageFollowMode = iota
	FollowModeHead
	FollowModeTail
)

func CheckGetHistory(ctx context.Context, state *State, chanID int, mode PageFollowMode) error {
	user, checker, push := state.PopRandomUser()
	if user == nil {
		return nil
	}
	defer push()

	err := checker.Play(ctx, &CheckAction{
		Method:      "POST",
		Path:        "/login",
		CheckFunc:   checkRedirectStatusCode,
		Description: "ログインできること",
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
	})
	if err != nil {
		return err
	}

	maxPage := 1
	minMap, _ := state.SnapshotMessageCount()
	err = checker.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               fmt.Sprintf("/history/%d", chanID),
		ExpectedStatusCode: 200,
		Description:        "チャットログが表示できること",
		CheckFunc: checkHTML(func(res *http.Response, doc *goquery.Document) error {
			_, maxMap := state.SnapshotMessageCount()

			page, err := validateHistoryPagination(doc)
			if err != nil {
				return err
			}

			minMsg, maxMsg := minMap[chanID], maxMap[chanID]

			// page数の計算を簡単化するため
			if minMsg == 0 {
				minMsg = 1
			}
			if maxMsg == 0 {
				maxMsg = 1
			}

			if page < (minMsg+19)/20 {
				return fatalErrorf("pagination の数が足りません")
			}

			if (maxMsg+19)/20 < page {
				return fatalErrorf("pagination の数が多すぎます")
			}

			maxPage = page

			return nil
		}),
	})
	if err != nil {
		return err
	}

	if minMap[chanID] == 0 {
		// there are no messages
		return nil
	}

	pages := []int{}
	if mode == FollowModeRandom {
		// random
		for _, r := range rand.Perm(maxPage) {
			pages = append(pages, r+1)
			if 5 <= len(pages) {
				break
			}
		}
	} else if mode == FollowModeHead {
		// head
		for i := 1; i <= maxPage; i++ {
			pages = append(pages, i)
			if 5 <= len(pages) {
				break
			}
		}
	} else {
		// tail
		for i := maxPage; 0 <= i; i-- {
			pages = append(pages, i)
			if 5 <= len(pages) {
				break
			}
		}
	}

	for _, page := range pages {
		err = checker.Play(ctx, &CheckAction{
			Method:             "GET",
			Path:               fmt.Sprintf("/history/%d?page=%d", chanID, page),
			ExpectedStatusCode: 200,
			Description:        "チャットログが表示できること",
			CheckFunc: checkHTML(func(res *http.Response, doc *goquery.Document) error {
				_, err := validateHistoryPagination(doc)
				if err != nil {
					return err
				}

				var e error
				msgCount := 0
				avatarPathMap := map[string]string{}
				messageSet := map[string]bool{}
				doc.Find(".message").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
					msgCount++
					avatarPath, avatarPathFound := selection.Find(".avatar").First().Attr("src")
					userName := selection.Find("h5").First().Text()
					content := selection.Find(".content").First().Text()
					date := selection.Find(".message-date").First().Text()

					if !avatarPathFound {
						e = fatalErrorf("アバター画像のパスがありません")
						return false
					}
					if userName == "" {
						e = fatalErrorf("表示名が表示されていません")
						return false
					}
					if content == "" {
						e = fatalErrorf("メッセージが表示されていません")
						return false
					}
					if date == "" {
						e = fatalErrorf("発言時刻が表示されていません")
						return false
					}

					idx := strings.LastIndex(userName, "@")
					if idx < 0 {
						e = fatalErrorf("表示名のフォーマットが正しくありません")
						return false
					}

					name := trim(userName[idx+1:])
					dispName := trim(userName[:idx])
					if name == "" {
						e = fatalErrorf("表示名のフォーマットが正しくありません")
						return false
					}

					u, ok := state.FindUserByName(name)
					if !ok {
						e = fatalErrorf("ユーザ名の表示が正しくありません")
						return false
					}
					if dispName != u.DisplayName {
						e = fatalErrorf("表示名の表示が正しくありません")
						return false
					}

					err = state.ValidateHistoryMessage(chanID, name, content, date)
					if err != nil {
						e = err
						return false
					}

					if messageSet[content] {
						e = fatalErrorf("メッセージが重複して表示されています")
						return false
					}
					messageSet[content] = true

					avatarPathMap[u.Name] = avatarPath
					return true
				})
				if e != nil {
					return e
				}

				if page == maxPage {
					if !(1 <= msgCount && msgCount <= 20) {
						return fatalErrorf("メッセージの表示件数が正しくありません")
					}
				} else {
					if msgCount != 20 {
						return fatalErrorf("メッセージの表示件数が正しくありません")
					}
				}

				cnt := 0
				for name, path := range avatarPathMap {
					u, ok := state.FindUserByName(name)
					if !ok {
						return fatalErrorf("ユーザ名の表示が正しくありません")
					}

					if cnt < 5 {
						err := checker.Play(ctx, &CheckAction{
							Method:             "GET",
							Path:               path,
							ExpectedStatusCode: 200,
							Description:        "正しいアバターが取得できること",
							CheckFunc:          checkAvatarImage(u.Avatar.MD5),
						})
						if err != nil {
							return err
						}
					}
					cnt++
				}

				return nil
			}),
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// fetch 検証用のユーザ作成
func CheckFecthRegisterAndLogin(ctx context.Context, state *State) error {
	if state.fetchCheckUser != nil {
		panic("fetchCheckUser is set already")
	}

	user, checker, _ := state.PopNewUser()

	err := checker.Play(ctx, &CheckAction{
		Method:    "POST",
		Path:      "/register",
		CheckFunc: checkRedirectStatusCode,
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
		Description: "新規ユーザが作成できること",
	})
	if err != nil {
		return err
	}

	err = checker.Play(ctx, &CheckAction{
		Method:    "POST",
		Path:      "/login",
		CheckFunc: checkRedirectStatusCode,
		PostData: map[string]string{
			"name":     user.Name,
			"password": user.Password,
		},
		Description: "作成したユーザでログインできること",
	})
	if err != nil {
		return err
	}

	state.fetchCheckUser = user

	return nil
}

func CheckFecthUnreadCount(ctx context.Context, state *State) error {
	checker := state.GetChecker(state.fetchCheckUser)

	minCnt, _ := state.SnapshotMessageCount()
	unreads := []JsonUnreadInfo{}
	err := checker.Play(ctx, &CheckAction{
		DisableSlowChecking: true,
		Method:              "GET",
		Path:                "/fetch",
		ExpectedStatusCode:  200,
		Description:         "新着情報が取得できること",
		CheckFunc: func(res *http.Response, body *bytes.Buffer) error {
			_, maxCnt := state.SnapshotMessageCount()

			dec := json.NewDecoder(body)
			err := dec.Decode(&unreads)
			if err != nil {
				return fatalErrorf("Jsonのデコードに失敗 %v", err)
			}

			valid := true
			for _, x := range unreads {
				if !(minCnt[x.ChannelID] <= x.Unread && x.Unread <= maxCnt[x.ChannelID]) {
					log.Printf("BAD UNREADS ChanID:%d Unread=%d ExpectedRange=[%d,%d]", x.ChannelID, x.Unread, minCnt, maxCnt)
					valid = false
				}
			}
			if !valid {
				return fatalErrorf("新着の件数が正しくありません")
			}

			return nil
		},
	})

	if err != nil {
		return err
	}

	return nil
}

func CheckMessageScenario(ctx context.Context, state *State) error {
	postLogin := func(user *AppUser, checker *Checker) error {
		return checker.Play(ctx, &CheckAction{
			Method:    "POST",
			Path:      "/login",
			CheckFunc: checkRedirectStatusCode,
			PostData: map[string]string{
				"name":     user.Name,
				"password": user.Password,
			},
			Description: "存在するユーザでログインできること",
		})
	}

	getMessage := func(checker *Checker, chanID, lastMessageID int, checkFunc func([]*JsonMessage) error) (error, int) {
		err := checker.Play(ctx, &CheckAction{
			Method:             "GET",
			Path:               fmt.Sprintf("/message?channel_id=%d&last_message_id=%d", chanID, lastMessageID),
			ExpectedStatusCode: 200,
			Description:        "メッセージが取得できること",
			CheckFunc: func(res *http.Response, body *bytes.Buffer) error {
				msgs := []*JsonMessage{}
				dec := json.NewDecoder(body)
				err := dec.Decode(&msgs)
				if err != nil {
					return fatalErrorf("Jsonのデコードに失敗 %v", err)
				}

				err = validateJsonMessage(state, chanID, lastMessageID, msgs)
				if err != nil {
					return err
				}

				if checkFunc != nil {
					err = checkFunc(msgs)
					if err != nil {
						return err
					}
				}

				if 0 < len(msgs) {
					lastMessageID = msgs[len(msgs)-1].ID
					counter.AddKey("get-message-count", len(msgs))
				}

				return nil
			},
		})
		return err, lastMessageID
	}

	getFetch := func(checker *Checker, checkFunc func([]*JsonUnreadInfo) error) (error, map[int]int) {
		var unreads []*JsonUnreadInfo
		err := checker.Play(ctx, &CheckAction{
			DisableSlowChecking: true,
			Method:              "GET",
			Path:                "/fetch",
			ExpectedStatusCode:  200,
			Description:         "新着情報が取得できること",
			CheckFunc: func(res *http.Response, body *bytes.Buffer) error {
				dec := json.NewDecoder(body)
				err := dec.Decode(&unreads)
				if err != nil {
					return fatalErrorf("Jsonのデコードに失敗 %v", err)
				}

				if checkFunc != nil {
					err = checkFunc(unreads)
					if err != nil {
						return err
					}
				}

				return nil
			},
		})
		if err != nil {
			return err, nil
		}

		m := map[int]int{}
		for _, u := range unreads {
			m[u.ChannelID] = u.Unread
		}
		return nil, m
	}

	getChannel := func(checker *Checker, chanID int) error {
		return checker.Play(ctx, &CheckAction{
			Method:             "GET",
			Path:               fmt.Sprintf("/channel/%d", chanID),
			ExpectedStatusCode: 200,
			Description:        "チャンネルが表示できること",
		})
	}

	postMessage := func(user *AppUser, checker *Checker, chanID int) (error, string) {
		var text string
		var complete func()
		var ok bool

		for !ok {
			text = RandomText()
			complete, ok = state.AddSendMessage(&MessageInfo{
				ChannelID: chanID,
				UserName:  user.Name,
				Message:   text,
			})
		}

		err := checker.Play(ctx, &CheckAction{
			Method:             "POST",
			Path:               "/message",
			ExpectedStatusCode: 204,
			PostData: map[string]string{
				"channel_id": fmt.Sprint(chanID),
				"message":    text,
			},
			Description: "メッセージが送信できること",
		})
		if err != nil {
			return err, ""
		}

		complete()
		return nil, text
	}

	user1, checker1, push1 := state.PopRandomUser()
	if user1 == nil {
		return nil
	}
	defer push1()

	user2, checker2, push2 := state.PopRandomUser()
	if user2 == nil {
		return nil
	}
	defer push2()

	user3, checker3, push3 := state.PopRandomUser()
	if user2 == nil {
		return nil
	}
	defer push3()

	var umap1, umap2, umap3 map[int]int
	var last1, last2 int
	var err, err1, err2, err3 error
	var wg sync.WaitGroup
	chanID := state.GetMsgCheckChannelID()

	wg.Add(3)

	go func() {
		defer wg.Done()
		err1 = postLogin(user1, checker1)
		if err1 != nil {
			return
		}

		err1 = getChannel(checker1, chanID)
		if err1 != nil {
			return
		}

		err1, last1 = getMessage(checker1, chanID, 0, nil)
		if err1 != nil {
			return
		}

		err1, umap1 = getFetch(checker1, func(unreads []*JsonUnreadInfo) error {
			for _, u := range unreads {
				if u.ChannelID == chanID {
					// 並行で走らない前提
					if u.Unread != 0 {
						return fatalErrorf("新着件数が正しくありません A")
					}
				}
			}
			return nil
		})
		if err1 != nil {
			return
		}
	}()

	go func() {
		defer wg.Done()

		err2 = postLogin(user2, checker2)
		if err2 != nil {
			return
		}

		err2 = getChannel(checker2, chanID)
		if err2 != nil {
			return
		}

		err2, last2 = getMessage(checker2, chanID, 0, nil)
		if err2 != nil {
			return
		}

		err2, umap2 = getFetch(checker2, nil)
		if err2 != nil {
			return
		}
	}()

	go func() {
		defer wg.Done()

		err3 = postLogin(user3, checker3)
		if err3 != nil {
			return
		}

		err3 = getChannel(checker3, 1)
		if err3 != nil {
			return
		}

		err3, _ = getMessage(checker3, 1, 0, nil)
		if err3 != nil {
			return
		}

		err3, umap3 = getFetch(checker3, nil)
		if err3 != nil {
			return
		}
	}()

	wg.Wait()

	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	if err3 != nil {
		return err3
	}

	err, sentText := postMessage(user1, checker1, chanID)
	if err != nil {
		return err
	}

	checkPlusOne := func(prevMap map[int]int) func([]*JsonUnreadInfo) error {
		return func(unreads []*JsonUnreadInfo) error {
			for _, u := range unreads {
				prev := prevMap[u.ChannelID]

				if u.ChannelID == chanID {
					if prev+1 != u.Unread {
						return fatalErrorf("新着件数が正しくありません B")
					}
				} else {
					if prev > u.Unread {
						return fatalErrorf("新着件数が正しくありません C")
					}
				}
			}
			return nil
		}
	}

	wg.Add(3)
	go func() {
		defer wg.Done()
		err1, umap1 = getFetch(checker1, checkPlusOne(umap1))
	}()

	go func() {
		defer wg.Done()
		err2, umap2 = getFetch(checker2, checkPlusOne(umap2))
	}()

	go func() {
		defer wg.Done()
		err3, umap3 = getFetch(checker3, checkPlusOne(umap3))
	}()

	wg.Wait()

	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	if err3 != nil {
		return err3
	}

	checkMsg := func(msgs []*JsonMessage) error {
		contain := false
		for _, msg := range msgs {
			if msg.Content == sentText {
				contain = true
				break
			}
		}
		if !contain {
			return fatalErrorf("送信したメッセージが取得できません")
		}

		msg := msgs[len(msgs)-1]
		if msg.Content != sentText {
			return fatalErrorf("メッセージの順番が正しくありません")
		}

		if msg.User.Name != user1.Name {
			return fatalErrorf("メッセージのユーザ名が正しくありません")
		}

		if msg.User.DisplayName != user1.DisplayName {
			return fatalErrorf("メッセージの表示名が正しくありません")
		}

		if 100 < len(msgs) {
			return fatalErrorf("メッセージの件数が正しくありません")
		}

		for i := 0; i < len(msgs)-1; i++ {
			if msgs[i].ID >= msgs[i+1].ID {
				return fatalErrorf("メッセージの順番が正しくありません")
			}
		}

		for _, msg := range msgs {
			err := state.ValidateJsonMessage(chanID, msg)
			if err != nil {
				return fatalErrorf("メッセージの検証に失敗 %v", err)
			}
		}

		return nil
	}

	err, last1 = getMessage(checker1, chanID, last1, func(msgs []*JsonMessage) error {
		if len(msgs) != 1 {
			return fatalErrorf("メッセージの取得件数が正しくありません")
		}
		return checkMsg(msgs)
	})
	if err != nil {
		return err
	}

	err, last2 = getMessage(checker2, chanID, last2, func(msgs []*JsonMessage) error {
		if len(msgs) != 1 {
			return fatalErrorf("メッセージの取得件数が正しくありません")
		}
		return checkMsg(msgs)
	})
	if err != nil {
		return err
	}

	err, _ = getMessage(checker3, chanID, 0, checkMsg)
	if err != nil {
		return err
	}

	err = checker1.Play(ctx, &CheckAction{
		Method:             "GET",
		Path:               fmt.Sprintf("/history/%d", chanID),
		ExpectedStatusCode: 200,
		Description:        "チャットログが表示できること",
		CheckFunc: checkHTML(func(res *http.Response, doc *goquery.Document) error {
			_, err := validateHistoryPagination(doc)
			if err != nil {
				return err
			}

			if doc.Find(".message").Size() == 0 {
				return fatalErrorf("メッセージが表示されていません")
			}

			selection := doc.Find(".message").Last()
			_, avatarPathFound := selection.Find(".avatar").First().Attr("src")
			userName := selection.Find("h5").First().Text()
			content := selection.Find(".content").First().Text()
			date := selection.Find(".message-date").First().Text()

			if !avatarPathFound {
				return fatalErrorf("アバター画像のパスがありません")
			}
			if userName == "" {
				return fatalErrorf("表示名が表示されていません")
			}
			if content == "" {
				return fatalErrorf("メッセージが表示されていません")
			}
			if date == "" {
				return fatalErrorf("発言時刻が表示されていません")
			}

			idx := strings.LastIndex(userName, "@")
			if idx < 0 {
				return fatalErrorf("表示名のフォーマットが正しくありません")
			}

			name := trim(userName[idx+1:])
			dispName := trim(userName[:idx])
			if name == "" {
				return fatalErrorf("表示名の表示が正しくありません")
			}

			u, ok := state.FindUserByName(name)
			if !ok {
				return fatalErrorf("ユーザ名の表示が正しくありません")
			}

			if dispName != u.DisplayName {
				return fatalErrorf("表示名の表示が正しくありません")
			}

			err = state.ValidateHistoryMessage(chanID, name, content, date)
			if err != nil {
				return err
			}

			if trim(content) != sentText {
				return fatalErrorf("送信したメッセージが表示されていません")
			}

			if u.Name != user1.Name {
				return fatalErrorf("ユーザ名が正しくありません")
			}

			return nil
		}),
	})
	if err != nil {
		return err
	}

	return nil
}
