package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	df "github.com/Individual-1/jackbox-client/drawful"
)

const (
	// URL data for room information
	roomBase = "ecast.jackboxgames.com"
	roomPath = "/room/"

	// URL endpoint for websocket connections
	wsBase       = "ecast.jackboxgames.com:38203"
	wsInfoPath   = "/socket.io/1/"
	wsSocketPath = "/socket.io/1/websocket/"
	wsInfoRegexp = "([a-z0-9]{28}):60:60:websocket,flashsocket"

	// Max ws message size (bytes)
	wsMaxMessageSize = 2048
)

// RoomInfo represents the data returned about a given room code
type RoomInfo struct {
	RoomID           string `json:"roomid"`
	Server           string `json:"server"`
	AppTag           string `json:"apptag"`
	AppID            string `json:"appid"`
	NumAudience      int    `json:"numAudience"`
	AudienceEnabled  bool   `json:"audienceEnabled"`
	JoinAs           string `json:"joinAs"`
	RequiresPassword bool   `json:"requiresPassword"`
}

// JackboxClient is a struct representing one session between a user and the
// Jackbox.tv games service
type JackboxClient struct {
	// Metadata about the room this client is associated with
	roomInfo RoomInfo

	// User ID, UUID format
	userID string

	// Websocket connection to jackbox service
	conn *websocket.Conn

	// Buffered channel for messages to the jackbox service
	sendJackbox chan string

	// Buffered channel for messages to the client frontend
	sendClient chan string

	// Waitgroup for ws handlers
	wg sync.WaitGroup

	// Initialized bool
	initialized bool
}

func main() {
	jc, err := MakeJackboxClient()
	if err != nil {
		return
	}

	err = jc.JoinRoom("test2", "SANB")
	if err != nil {
		return
	}

	err = jc.SetPlayerPicture("./test.json")
	if err != nil {
		return
	}

	jc.wg.Wait()
}

// MakeJackboxClient initializes a jackbox client with an auto-generated user id
func MakeJackboxClient() (*JackboxClient, error) {
	var jc JackboxClient
	var err error

	err = jc.genUserID()
	if err != nil {
		return &jc, err
	}

	jc.sendJackbox = make(chan string, 20)
	jc.sendClient = make(chan string, 20)

	jc.initialized = true

	return &jc, nil
}

func (jc *JackboxClient) genUserID() error {
	userID, err := uuid.NewUUID()
	if err != nil {
		return err
	}

	jc.userID = userID.String()

	return nil
}

// GetRoomInfo generates a user ID if necessary and retrieves the room information for a given code
func (jc *JackboxClient) GetRoomInfo(roomID string) error {
	var err error

	roomURL := url.URL{Scheme: "https", Host: roomBase, Path: path.Join(roomPath, roomID)}

	// https://ecast.jackboxgames.com/room/{roomID}}?userId={userID}
	// Add our generated user uuid to query
	if jc.userID == "" {
		err = jc.genUserID()
		if err != nil {
			return err
		}
	}

	roomURL.Query().Add("userId", jc.userID)

	resp, err := http.Get(roomURL.String())
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return errors.New("Failed to find room")
	}

	err = json.NewDecoder(resp.Body).Decode(&jc.roomInfo)
	if err != nil {
		return err
	}

	return nil
}

// JoinRoom joins the specified room code with the specified name
func (jc *JackboxClient) JoinRoom(name string, roomID string) error {
	var msg jbMsg
	var arg jbMsgActionJoinRoom

	if !jc.initialized {
		return errors.New("JackboxClient must be initialized before use")
	}

	err := jc.GetRoomInfo(roomID)
	if err != nil {
		return err
	}

	arg.Action = "JoinRoom"
	arg.AppID = jc.roomInfo.AppID
	arg.JoinType = jc.roomInfo.JoinAs
	arg.Name = name
	arg.Options = jbMsgActionJoinRoomOptions{RoomCode: jc.roomInfo.RoomID, Name: name}
	arg.RoomID = jc.roomInfo.RoomID
	arg.Type = "Action"
	arg.UserID = jc.userID

	sarg, err := json.Marshal(arg)
	if err != nil {
		return err
	}

	msg.Name = "msg"
	msg.Args = append(msg.Args, sarg)

	smsg, err := msg.Serialize()
	if err != nil {
		return err
	}

	err = jc.setupWebsocket()
	if err != nil {
		return err
	}

	jc.wg.Add(2)
	go jc.handleJBWrites()
	go jc.handleJBReads()

	jc.sendJackbox <- smsg

	return nil
}

// SetPlayerPicture send a setPlayerPicture action to the JB server
func (jc *JackboxClient) SetPlayerPicture(fn string) error {
	var msg jbMsg
	var arg jbMsgActionSendMsgToRoomOwner
	var pic jbOwnerMsgSetPlayerPicture

	if !jc.initialized {
		return errors.New("JackboxClient must be initialized before use")
	}

	arg.Type = "Action"
	arg.Action = "SendMessageToRoomOwner"
	arg.AppID = jc.roomInfo.AppID
	arg.RoomID = jc.roomInfo.RoomID
	arg.UserID = jc.userID

	pl, err := df.LoadDrawing(fn)
	if err != nil {
		return err
	}

	pic.PictureLines = pl
	pic.SetPlayerPicture = true

	sp, err := json.Marshal(pic)
	if err != nil {
		return err
	}

	arg.Message = sp

	sarg, err := json.Marshal(arg)
	if err != nil {
		return err
	}

	msg.Name = "msg"
	msg.Args = append(msg.Args, sarg)

	smsg, err := msg.Serialize()
	if err != nil {
		return err
	}

	jc.sendJackbox <- smsg

	return nil
}

func (jc *JackboxClient) setupWebsocket() error {
	wsInfoURL := url.URL{Scheme: "https", Host: wsBase, Path: wsInfoPath}

	resp, err := http.Get(wsInfoURL.String())
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	wsInfo, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	re := regexp.MustCompile(wsInfoRegexp)
	matches := re.FindSubmatch(wsInfo)
	if len(matches) != 2 {
		return errors.New("Failed to retrieve websocket name")
	}

	wsName := string(matches[1])

	wsConnectURL := url.URL{Scheme: "wss", Host: wsBase, Path: path.Join(wsSocketPath, wsName)}

	jc.conn, _, err = websocket.DefaultDialer.Dial(wsConnectURL.String(), nil)
	if err != nil {
		return err
	}

	return nil
}

// handleJBReads is a websocket handler for incoming messages from the jackbox service
func (jc *JackboxClient) handleJBReads() {
	defer jc.conn.Close()
	defer jc.wg.Done()

	jc.conn.SetReadLimit(wsMaxMessageSize)

	for {
		_, msg, err := jc.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Websocket error: %v", err)
			}
			break
		}

		msg = bytes.TrimSpace(msg)
		jc.handleJBWSMsg(string(msg))
	}
}

// handleJBWrites is a routine for writing messages out to the jackbox service
func (jc *JackboxClient) handleJBWrites() {
	defer jc.conn.Close()
	defer jc.wg.Done()

	for {
		msg := <-jc.sendJackbox

		fmt.Println(msg)
		w, err := jc.conn.NextWriter(websocket.TextMessage)
		if err != nil {
			return
		}

		w.Write([]byte(msg))
	}
}

func (jc *JackboxClient) handleJBWSMsg(msg string) {
	switch {
	// First message sent, probably just ignore?
	case msg == "1::":

	// Websocket keepalive
	case msg == "2:::":
		jc.sendJackbox <- "2::"

	// General event and result messages
	case strings.HasPrefix(msg, "5:::"):
		jc.handleJBMsgType(msg[4:])
	}
}

// handleJBMsgType parses and handles messages prefixed with 5:::
// These are generally result or event messages used to convey data
func (jc *JackboxClient) handleJBMsgType(msg string) {
	var pMsg jbMsg
	var err error

	err = json.Unmarshal([]byte(msg), &pMsg)
	if err != nil {
		return
	}

	for _, item := range pMsg.Args {
		var jbMsgItem jbMsgType

		err = json.Unmarshal(item, &jbMsgItem)
		if err != nil {
			return
		}

		switch jbMsgItem.Type {
		case "Action":
		case "Result":
			var resMsg jbMsgResult
			err = json.Unmarshal(item, &resMsg)
			if err != nil {
				return
			}

		case "Event":
			var eventMsg jbMsgEvent
			err = json.Unmarshal(item, &eventMsg)
			if err != nil {
				return
			}
		}
	}
}
