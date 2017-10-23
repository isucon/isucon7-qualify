package bench

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"bench/counter"
)

type JsonUser struct {
	ID          int    `json:"id"`
	AvatarIcon  string `json:"avatar_icon"`
	DisplayName string `json:"display_name"`
	Name        string `json:"name"`
}

type JsonMessage struct {
	ID      int      `json:"id"`
	Content string   `json:"content"`
	Date    string   `json:"date"`
	User    JsonUser `json:"user"`
}

type JsonUnreadInfo struct {
	ChannelID int `json:"channel_id"`
	Unread    int `json:"unread"`
}

type BenchDataSet struct {
	Users    []*AppUser
	NewUsers []*AppUser

	Channels    []*Channel
	NewChannels []*Channel

	Avatars       []*Avatar
	LargeAvatars  []*Avatar
	DefaultAvatar *Avatar

	Texts    []string
	Messages []*MessageInfo
}

type Avatar struct {
	FilePath string
	Ext      string
	SHA1     string
	MD5      string
	Bytes    []byte
}

type AppUser struct {
	sync.Mutex
	Name        string
	Password    string
	DisplayName string
	Avatar      *Avatar
}

type MessageInfo struct {
	UserName      string
	ChannelID     int
	Message       string
	SendBeginTime time.Time
	SendEndTime   time.Time
	SendComplete  bool
}

type Channel struct {
	ID          int
	Name        string
	Description string
}

type State struct {
	mtx                sync.Mutex
	users              []*AppUser
	newUsers           []*AppUser
	userMap            map[string]*AppUser
	checkerMap         map[*AppUser]*Checker
	channelMap         map[int]*Channel
	activeChannelIDs   []int
	inactiveChannelIDs []int
	msgCheckChannelIDs []int
	tmpChannelIDs      []int

	msgMtx    sync.Mutex
	msgMap    map[int]map[string]*MessageInfo
	msgCntMin map[int]int
	msgCntMax map[int]int

	fetchCheckUser *AppUser
}

func (s *State) Init() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.users = append(s.users, DataSet.Users...)
	s.newUsers = append(s.newUsers, DataSet.NewUsers...)
	s.userMap = map[string]*AppUser{}
	s.channelMap = map[int]*Channel{}
	s.checkerMap = map[*AppUser]*Checker{}
	s.msgMap = map[int]map[string]*MessageInfo{}
	s.msgCntMin = map[int]int{}
	s.msgCntMax = map[int]int{}

	p := rand.Perm(len(DataSet.Channels))
	for i, c := range DataSet.Channels {
		chanID := c.ID
		s.channelMap[chanID] = c

		if i == p[0] {
			s.activeChannelIDs = append(s.activeChannelIDs, chanID)
		} else if i == p[1] {
			s.msgCheckChannelIDs = append(s.msgCheckChannelIDs, chanID)
		} else {
			s.inactiveChannelIDs = append(s.inactiveChannelIDs, chanID)
		}
	}

	for _, u := range DataSet.Users {
		s.userMap[u.Name] = u
	}

	for _, msg := range DataSet.Messages {
		_, ok := s.AddSendMessage(msg)
		if !ok {
			panic("duplicated message in the dataset")
		}
	}
}

func (s *State) DistributeTmpChannelIDs() {
	for _, idx := range rand.Perm(len(s.tmpChannelIDs)) {
		chanID := s.tmpChannelIDs[idx]
		if len(s.activeChannelIDs) < 5 {
			s.activeChannelIDs = append(s.activeChannelIDs, chanID)
		} else if len(s.msgCheckChannelIDs) < 2 {
			s.msgCheckChannelIDs = append(s.msgCheckChannelIDs, chanID)
		} else {
			s.inactiveChannelIDs = append(s.inactiveChannelIDs, chanID)
		}
	}
}

func (s *State) TotalChannelCount() int {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return len(s.activeChannelIDs) + len(s.inactiveChannelIDs) + len(s.msgCheckChannelIDs) + len(s.tmpChannelIDs)
}

func (s *State) PopRandomUser() (*AppUser, *Checker, func()) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	n := len(s.users)
	if n == 0 {
		return nil, nil, nil
	}

	i := rand.Intn(n)
	u := s.users[i]

	s.users[i] = s.users[n-1]
	s.users[n-1] = nil
	s.users = s.users[:n-1]

	return u, s.getCheckerLocked(u), func() { s.PushUser(u) }
}

func (s *State) FindUserByName(name string) (*AppUser, bool) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	u, ok := s.userMap[name]
	return u, ok
}

func (s *State) popNewUserLocked() (*AppUser, *Checker, func()) {
	n := len(s.newUsers)
	if n == 0 {
		return nil, nil, nil
	}

	u := s.newUsers[n-1]
	s.newUsers = s.newUsers[:n-1]

	return u, s.getCheckerLocked(u), func() { s.PushUser(u) }
}

func (s *State) PopNewUser() (*AppUser, *Checker, func()) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.popNewUserLocked()
}

func (s *State) PushUser(u *AppUser) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.userMap[u.Name] = u
	s.users = append(s.users, u)
}

func (s *State) GetChecker(u *AppUser) *Checker {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.getCheckerLocked(u)
}

func (s *State) getCheckerLocked(u *AppUser) *Checker {
	checker, ok := s.checkerMap[u]

	if !ok {
		checker = NewChecker()
		checker.debugHeaders["X-Username"] = u.Name
		s.checkerMap[u] = checker
	}

	return checker
}

func (s *State) AddChannel(id int, c *Channel) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.channelMap[id] = c
	s.tmpChannelIDs = append(s.tmpChannelIDs, id)
}

func (s *State) GetRandomChannelID() int {
	if rand.Intn(100) < 50 {
		return s.GetActiveChannelID()
	} else {
		return s.GetInactiveChannelID()
	}
}

func (s *State) GetActiveChannelID() int {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.activeChannelIDs[rand.Intn(len(s.activeChannelIDs))]
}

func (s *State) GetInactiveChannelID() int {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.inactiveChannelIDs[rand.Intn(len(s.inactiveChannelIDs))]
}

func (s *State) GetMsgCheckChannelID() int {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.msgCheckChannelIDs[rand.Intn(len(s.msgCheckChannelIDs))]
}

func (s *State) GetChannel(chanID int) (*Channel, bool) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	c, ok := s.channelMap[chanID]
	return c, ok
}

func (s *State) AddSendMessage(x *MessageInfo) (func(), bool) {
	s.msgMtx.Lock()
	defer s.msgMtx.Unlock()

	m, ok := s.msgMap[x.ChannelID]
	if !ok {
		m = map[string]*MessageInfo{}
		s.msgMap[x.ChannelID] = m
	}

	_, ok = m[x.Message]
	if ok {
		return nil, false
	}

	s.msgCntMax[x.ChannelID]++
	if x.SendComplete {
		// 最初から送信済 (初期データセット)
		s.msgCntMin[x.ChannelID]++
	} else {
		x.SendBeginTime = time.Now()
	}
	m[x.Message] = x

	return func() {
		s.msgMtx.Lock()
		defer s.msgMtx.Unlock()

		if !x.SendComplete {
			s.msgCntMin[x.ChannelID]++
			x.SendComplete = true
		}
		x.SendEndTime = time.Now()
	}, true
}

func (s *State) ValidateJsonMessage(chanID int, msg *JsonMessage) error {
	s.msgMtx.Lock()
	defer s.msgMtx.Unlock()

	m, ok := s.msgMap[chanID]
	if !ok {
		return fmt.Errorf("発言していないチャンネルへのメッセージ")
	}

	x, ok := m[trim(msg.Content)]
	if !ok {
		return fmt.Errorf("発言していないメッセージ %v", msg)
	}

	if x.UserName != msg.User.Name {
		return fmt.Errorf("発言者が異なります")
	}

	// 2018/04/21 17:11:28
	d := msg.Date
	if len(d) != 19 || strings.Count(d, ":") != 2 || strings.Count(d, "/") != 2 || strings.Count(d, " ") != 1 {
		return fmt.Errorf("時刻のフォーマットが正しくありません")
	}

	if !x.SendBeginTime.IsZero() && time.Since(x.SendBeginTime) < 200*time.Millisecond {
		counter.IncKey("message-bonus")
	}

	// 未読件数の検証を厳しくする場合有効にする
	/*
		if !x.SendComplete {
			s.msgCntMin[x.ChannelID]++
			x.SendComplete = true
		}
	*/

	return nil
}

func (s *State) ValidateHistoryMessage(chanID int, userName string, msg string, date string) error {
	s.msgMtx.Lock()
	defer s.msgMtx.Unlock()

	m, ok := s.msgMap[chanID]
	if !ok {
		return fmt.Errorf("発言していないチャンネルへのメッセージ")
	}

	x, ok := m[trim(msg)]
	if !ok {
		return fmt.Errorf("発言していないメッセージ %v", msg)
	}

	if x.UserName != userName {
		return fmt.Errorf("発言者が異なります")
	}

	d := trim(date)
	if len(d) != 19 || strings.Count(d, ":") != 2 || strings.Count(d, "/") != 2 || strings.Count(d, " ") != 1 {
		return fmt.Errorf("時刻のフォーマットが正しくありません")
	}

	// 未読件数の検証を厳しくする場合有効にする
	/*
		if !x.SendComplete {
			s.msgCntMin[x.ChannelID]++
			x.SendComplete = true
		}
	*/

	return nil
}

func (s *State) SnapshotMessageCount() (map[int]int, map[int]int) {
	s.msgMtx.Lock()
	defer s.msgMtx.Unlock()

	minMap := map[int]int{}
	maxMap := map[int]int{}

	for k, v := range s.msgCntMin {
		minMap[k] = v
	}

	for k, v := range s.msgCntMax {
		maxMap[k] = v
	}

	return minMap, maxMap
}
