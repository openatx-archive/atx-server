package dingrobot

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
)

const defaultAPI = "https://oapi.dingtalk.com/robot/send"

type Robot struct {
	token string
	at    robotAt
}

type robotAt struct {
	AtMobiles []string `json:"atMobiles"`
	AtAll     bool     `json:"isAtAll"`
}

type robotRequest struct {
	Type string `json:"msgtype"`
	Text struct {
		Content string `json:"content"`
	} `json:"text,omitempty"`
	Markdown struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	} `json:"markdown,omitempty"`
	Link struct {
		Title      string `json:"title"`
		Text       string `json:"text"`
		PictureURL string `json:"picUrl"` // optional
		MessageURL string `json:"messageUrl"`
	} `json:"link,omitempty"`
	At robotAt `json:"at,omitempty"`
}

type robotResponse struct {
	Code    int    `json:"errcode"`
	Message string `json:"errmsg"`
}

func New(token string) *Robot {
	return &Robot{
		token: token,
	}
}

func (r *Robot) AtAll(ok bool) *Robot {
	newRobot := *r
	newRobot.at.AtAll = ok
	return &newRobot
}

func (r *Robot) AtMobiles(tels ...string) *Robot {
	newRobot := *r
	newRobot.at.AtMobiles = tels
	return &newRobot
}

func (r *Robot) sendUrl() string {
	return defaultAPI + "?access_token=" + r.token
}

func (r *Robot) postData(request robotRequest) error {
	request.At = r.at
	jdata, _ := json.Marshal(request)
	resp, err := http.Post(r.sendUrl(), "application/json", bytes.NewBuffer(jdata))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	robotResp := robotResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&robotResp); err != nil {
		return err
	}
	if robotResp.Code != 0 {
		return errors.New(robotResp.Message)
	}
	return nil
}

func (r *Robot) Text(content string) error {
	request := robotRequest{
		Type: "text",
	}
	request.Text.Content = content
	return r.postData(request)
}

func (r *Robot) Markdown(title string, text string) error {
	request := robotRequest{Type: "markdown"}
	request.Markdown.Title = title
	request.Markdown.Text = text
	return r.postData(request)
}

func (r *Robot) Link(title, text string, url string, picUrl string) error {
	request := robotRequest{Type: "link"}
	request.Link.Title = title
	request.Link.Text = text
	request.Link.PictureURL = picUrl
	request.Link.MessageURL = url
	return r.postData(request)
}
