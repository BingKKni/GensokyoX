package handlers

import (
	"encoding/json"

	"github.com/hoshinonyaruko/gensokyo/callapi"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/hoshinonyaruko/gensokyo/structs"
	"github.com/tencent-connect/botgo/openapi"
)

func init() {
	callapi.RegisterHandler("get_friend_list", GetFriendList)
}

type FriendList struct {
	Data    []structs.FriendData `json:"data"`
	Message string               `json:"message"`
	RetCode int                  `json:"retcode"`
	Status  string               `json:"status"`
	Echo    interface{}          `json:"echo"`
}

func GetFriendList(client callapi.Client, api openapi.OpenAPI, apiv2 openapi.OpenAPI, message callapi.ActionMessage) (string, error) {
	var friendList FriendList
	var err error

	// Retrieves all users from the database
	users, err := idmap.ListAllUsers()
	if err != nil {
		mylog.Printf("Error fetching user list: %v", err)
		return "", err
	}

	// Ensure Data is not nil even if users is empty
	if users == nil {
		friendList.Data = []structs.FriendData{}
	} else {
		friendList.Data = users
	}

	friendList.Message = ""
	friendList.RetCode = 0
	friendList.Status = "ok"

	if message.Echo == "" {
		friendList.Echo = "0"
	} else {
		friendList.Echo = message.Echo
	}

	var result []byte
	result, err = json.Marshal(friendList)
	if err != nil {
		mylog.Printf("Error marshaling data: %v", err)
		return "", err
	}

	return string(result), nil
}
