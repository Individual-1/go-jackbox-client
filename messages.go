package main

import (
	"encoding/json"
	"fmt"

	"github.com/Individual-1/jackbox-client/drawful"
)

/*
	jbFive message types are generic wrappers for messages prefixed with 5:::
	They generally consist of a name and list of arguments, with each argument being the true message
*/

// jbMsg is a top-level struct for the 5::: message type
type jbMsg struct {
	Name string            `json:"name"`
	Args []json.RawMessage `json:"args"`
}

func (msg *jbMsg) Serialize() (string, error) {
	base, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("5:::%s", base), nil
}

// jbMsgType is the type field within an arg item of jbMsg
// so we can figure out which subtype to use
type jbMsgType struct {
	Type string `json:"type"`
}

/*
	These are more specific message types to handle common Jackbox message formats
*/

// jbMsgResult is for the type: "Result" of jbMsgType
type jbMsgResult struct {
	jbMsgType
	Action   string `json:"action"`
	Success  bool   `json:"success"`
	Initial  bool   `json:"initial"`
	RoomID   string `json:"roomId"`
	JoinType string `json:"joinType"`
	UserID   string `json:"userId"`
}

// jbMsgEvent is for the type: "Event"
type jbMsgEvent struct {
	jbMsgType
	Event  string                 `json:"event"`
	RoomID string                 `json:"roomId"`
	Blob   map[string]interface{} `json:"blob"`
}

// jbMsgAction is for the type: "Action"
type jbMsgAction struct {
	jbMsgType
	Action string `json:"action"`
	AppID  string `json:"appId"`
	RoomID string `json:"roomId"`
	UserID string `json:"userId"`
}

// jbMsgActionJoinRoom is a structure representing the json message to join a room
type jbMsgActionJoinRoom struct {
	jbMsgAction
	JoinType string                     `json:"joinType"`
	Name     string                     `json:"name"`
	Options  jbMsgActionJoinRoomOptions `json:"options"`
}

type jbMsgActionJoinRoomOptions struct {
	RoomCode string `json:"roomcode"`
	Name     string `json:"name"`
}

// jbMsgActionSendMsgToRoomOwner is a structure representing a message to the room owner
type jbMsgActionSendMsgToRoomOwner struct {
	jbMsgAction
	Message json.RawMessage `json:"message"`
}

// jbOwnerMsgSetPlayerPicture is the message subfield within a jbMsgActionSendMsgToRoomOwner
type jbOwnerMsgSetPlayerPicture struct {
	SetPlayerPicture bool                  `json:"setPlayerPicture"`
	PictureLines     []drawful.PictureLine `json:"pictureLines"`
}
