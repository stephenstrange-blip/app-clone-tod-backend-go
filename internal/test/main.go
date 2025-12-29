package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	customUtil "github.com/app-clone-tod-utils"
	"github.com/joho/godotenv"
)

type testResponse struct {
	URL     string
	Handler func(m string, url string) (*http.Cookie, error)
}

var Reset = "\033[0m"
var Red = "\033[31m"
var Green = "\033[32m"

type Color struct {
}

func (c *Color) Green(str string) string {
	return Green + str + Reset
}

func (c *Color) Red(str string) string {
	return Red + str + Reset
}

func init() {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatalf("Error loading env file, %s\n", err.Error())
	}

}

func main() {
	METHODS := []string{
		http.MethodPost,
		// http.MethodGet,
		// http.MethodPut,
		// http.MethodDelete,
	}

	test := []testResponse{
		// {URL: "/logout/", Handler: Ctr.Handle(AUTH.HandleLogout)},
		// {URL: "/signup/", Handler: Ctr.GetHandler(AUTH.HandleSignup)},
		// {URL: "/auth/local/", Handler: testLocalAuth},
		// {URL: "/auth/github/", Handler: Ctr.GetHandler(AUTH.HandleAuthGithub)},
		// {URL: "/users/auth/me/", Handler: Ctr.GetHandler(AUTH.HandleAuthMe)},
		// {URL: "/users/profile/", Handler: Ctr.GetHandler(Ctr.HandleProfile)},
		{URL: "/users/request/", Handler: testRequestController},
		// {URL: "/users/network/", Handler: Ctr.GetHandler(Ctr.HandleNetwork)},
		// {URL: "/users/reaction/", Handler: testReactionController},
		// {URL: "/users/chat/2", Handler: Ctr.GetHandler(Ctr.HandleChat(dbConfig))},
		// {URL: "/users/chat/e", Handler: Ctr.GetHandler(Ctr.HandleChat(dbConfig))},
		// {URL: "/users/post/", Handler: testPostController},
		// {URL: "/users/post/5", Handler: testPostController},
		// {URL: "/users/post/e", Handler: Ctr.GetHandler(Ctr.HandleSinglePost(dbConfig))},
		// {URL: "/users/post/5/comment/3", Handler: testCommentController},
		// {URL: "/users/post/2/comment/e", Handler: Ctr.GetHandler(Ctr.HandleSingleComment(dbConfig))},
	}

	for _, t := range test {
		for _, m := range METHODS {
			if _, err := t.Handler(m, t.URL); err != nil {
				fmt.Printf("error: %s\n\n", err.Error())
			}

			time.Sleep(time.Second * 4)
		}
	}
}

func testCommentController(m, requestURL string) (*http.Cookie, error) {
	var (
		C   = &Color{}
		req *http.Request
	)

	type Comment struct {
		AuthorID   int    `json:"authorID,omitzero"`
		GetReplies bool   `json:"getReplies,omitzero"`
		Message    string `json:"message,omitzero"`
	}

	CommentParams := &Comment{
		AuthorID: 2,
		Message:  "Comment",
	}

	b, err := json.Marshal(CommentParams)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(b)

	// put all fields to url query as GET doesn't have a request body
	if m == http.MethodGet {

		urlQuerry := url.Values{}
		urlQuerry.Add("getReplies", "true")

		req, _ = http.NewRequest(m, fmt.Sprintf("%s%s?%s", "http://localhost:8080", requestURL, urlQuerry.Encode()), nil)
	} else {
		req, _ = http.NewRequest(m, fmt.Sprintf("%s%s", "http://localhost:8080", requestURL), reader)
	}

	cookie, err := testLocalAuth(http.MethodPost, "/auth/local/")
	if err != nil {
		return nil, err
	}

	req.AddCookie(cookie)
	// req.Header.Set("Content-type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	message := fmt.Sprintf("URL: %s,\n Method: %s\n Status: %d,\n Content-type: %s,\n Request_Content-type: %s,\n Body: %s\n", requestURL, m, resp.StatusCode, resp.Header.Get("Content-type"), req.Header.Get("Content-type"), string(body))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Println(C.Green(message))
	} else {
		fmt.Println(C.Red(message))
	}

	return nil, nil
}

func testPostController(m, requestURL string) (*http.Cookie, error) {
	var (
		C   = &Color{}
		req *http.Request
	)

	type Post struct {
		PostID     int       `json:"postID,omitzero"`
		CategoryID int       `json:"categoryID,omitzero"`
		AuthorID   int       `json:"authorID,omitzero"`
		Published  bool      `json:"published"`
		IsDeleted  bool      `json:"isDeleted"`
		Message    string    `json:"message,omitzero"`
		Title      string    `json:"title,omitzero"`
		Start      time.Time `json:"start,omitzero"`
		End        time.Time `json:"end,omitzero"`
		MyPosts    bool      `json:"myPosts,omitempty"`
	}

	postParams := &Post{
		PostID:     5,
		CategoryID: 1,
		AuthorID:   1,
		Published:  false,
		IsDeleted:  false,
		Message:    "Message",
		Title:      "Title",
		Start:      time.Date(2025, time.December, 15, 13, 26, 9, 0, time.Local),
		End:        time.Date(2025, time.December, 16, 19, 1, 1, 1, time.Local),
		MyPosts:    true,
	}

	b, err := json.Marshal(postParams)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(b)

	// put all fields to url query as GET doesn't have a request body
	if m == http.MethodGet {

		urlQuerry := url.Values{}
		urlQuerry.Add("start", postParams.Start.Format(time.RFC3339))
		urlQuerry.Add("end", postParams.End.Format(time.RFC3339))
		urlQuerry.Add("published", strconv.FormatBool(postParams.Published))
		urlQuerry.Add("myPosts", strconv.FormatBool(postParams.MyPosts))
		urlQuerry.Add("categoryId", strconv.FormatInt(int64(postParams.CategoryID), 10))

		req, _ = http.NewRequest(m, fmt.Sprintf("%s%s?%s", "http://localhost:8080", requestURL, urlQuerry.Encode()), nil)
	} else {
		req, _ = http.NewRequest(m, fmt.Sprintf("%s%s", "http://localhost:8080", requestURL), reader)
	}

	cookie, err := testLocalAuth(http.MethodPost, "/auth/local/")
	if err != nil {
		return nil, err
	}

	req.AddCookie(cookie)
	req.Header.Set("Content-type", "application/x-www-form-urlencoded")
	// req.Header.Set("Content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	message := fmt.Sprintf("URL: %s,\n Method: %s\n Status: %d,\n Content-type: %s,\n Request_Content-type: %s,\n Body: %s\n", requestURL, m, resp.StatusCode, resp.Header.Get("Content-type"), req.Header.Get("Content-type"), string(body))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Println(C.Green(message))
	} else {
		fmt.Println(C.Red(message))
	}

	return nil, nil
}

func testRequestController(m, url string) (*http.Cookie, error) {
	C := &Color{}

	type Body struct {
		TargetID     int  `json:"targetId"`
		RequesterID  int  `json:"requesterId"`
		FromRequests bool `json:"fromRequests"`
	}

	params := Body{
		TargetID:     2,
		RequesterID:  1,
		FromRequests: false,
	}

	b, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(b)
	req, _ := http.NewRequest(m, fmt.Sprintf("%s%s%s", "http://localhost:8080", url, "?fromRequests=true"), reader)

	cookie, err := testLocalAuth(http.MethodPost, "/auth/local/")
	if err != nil {
		return nil, err
	}
	req.AddCookie(cookie)
	// req.Header.Set("Content-type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-type", "application/json")
	// fmt.Printf("Cookies %+v\n", req.CookiesNamed(customUtil.COOKIE_NAME))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	message := fmt.Sprintf("URL: %s, Method: %s, Status: %d, Content-type: %s, Request_Content-type: %s, Body: %s", url, m, resp.StatusCode, resp.Header.Get("Content-type"), req.Header.Get("Content-type"), string(body))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Println(C.Green(message))
	} else {
		fmt.Println(C.Red(message))
	}

	cookies := resp.Cookies()
	for _, cookie := range cookies {
		if cookie.Name == customUtil.COOKIE_NAME {
			return cookie, nil
		}
	}

	return nil, nil
}

func testReactionController(m, url string) (*http.Cookie, error) {
	C := &Color{}

	type Body struct {
		Id_react  int `json:"id_react,omitzero"` // the row id
		PostID    int `json:"postID,omitzero"`
		ReactorID int `json:"reactorID,omitzero"`
		ReactID   int `json:"reactID,omitzero"` // lists what type of react from "React" table
	}

	params := Body{
		Id_react: 1,
		PostID:   1,
		ReactID:  1,
	}

	b, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(b)

	req, _ := http.NewRequest(m, fmt.Sprintf("%s%s%s", "http://localhost:8080", url, "?id_react=1"), reader)

	cookie, err := testLocalAuth(http.MethodPost, "/auth/local/")
	if err != nil {
		return nil, err
	}
	req.AddCookie(cookie)
	req.Header.Set("Content-type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	message := fmt.Sprintf("URL: %s, Method: %s, Status: %d, Content-type: %s, Request_Content-type: %s, Body: %s", url, m, resp.StatusCode, resp.Header.Get("Content-type"), req.Header.Get("Content-type"), string(body))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Println(C.Green(message))
	} else {
		fmt.Println(C.Red(message))
	}

	cookies := resp.Cookies()
	for _, cookie := range cookies {
		if cookie.Name == customUtil.COOKIE_NAME {
			return cookie, nil
		}
	}

	return nil, errors.New("cookie not found")
}

func testLocalAuth(m, url string) (*http.Cookie, error) {
	// C := &Color{}

	type LocalAuth struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	params, err := json.Marshal(LocalAuth{Username: "Admin", Password: "admin"})
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(params)

	req, err := http.NewRequest(m, fmt.Sprintf("%s%s", "http://localhost:8080", url), reader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-type", "application/json") // important to avoid End of JSON error
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// body, _ := io.ReadAll(resp.Body)

	// message := fmt.Sprintf("URL: %s, Method: %s, Status: %d, Content-type: %s, Request_Content-type: %s, Body: %s\n", url, m, resp.StatusCode, resp.Header.Get("Content-type"), req.Header.Get("Content-type"), string(body))

	// if resp.StatusCode >= 200 && resp.StatusCode < 300 {
	// 	fmt.Println(C.Green(message))
	// } else {
	// 	fmt.Println(C.Red(message))
	// }

	cookies := resp.Cookies()
	for _, cookie := range cookies {
		if cookie.Name == customUtil.COOKIE_NAME {
			return cookie, nil
		}
	}

	return nil, errors.New("cookie not found")
}
