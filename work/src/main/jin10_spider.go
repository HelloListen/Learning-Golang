//jin10.com抓取，使用10个代理IP，60秒抓取一次
//抓取时间0:00-7:00
//过滤关键字"jin10", "金十", "推荐阅读", "视频", "新品上线"

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	// "fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	apiURL     = "http://api.wallstreetcn.com/v2/admin/livenews?api_key=YofaP1f3"
	testapiURL = "http://api.wallstcn.com/v2/admin/livenews?api_key=lQecdGAe"
	PREVTXT    = "prevContent.txt"
	LISTCOUNT  = 5
)

var prevContent = make([]string, 0)

var proxy = []string{"http://123.59.83.131:23128", "http://123.59.83.140:23128", "http://123.59.83.137:23128",
	"http://123.59.83.147:23128", "http://123.59.83.139:23128", "http://123.59.83.132:23128",
	"http://123.59.83.141:23128", "http://123.59.83.148:23128", "http://123.59.83.130:23128", "http://123.59.83.145:23128"}

type Jin10 struct {
	jin10_page string `json:"-"`
	CodeType   string `json:"codeType"`
	CreateAt   int64  `json:"createAt"`
	Channels   []int  `json:"channels"`
	Content    string `json:"content"`
	Importance int    `json:"importance"`
}

func (j Jin10) getByProxy(url_addr, proxy_addr string) (*http.Response, error) {
	request, err := http.NewRequest("GET", url_addr, nil)
	if err != nil {
		return nil, err
	}
	proxy, err := url.Parse(proxy_addr)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxy),
		},
	}
	return client.Do(request)
}

func (j Jin10) dealTime() (ts int64, err error) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return 0, err
	}
	now := time.Now()
	now = now.In(loc)
	return now.Unix(), nil
}

func (j *Jin10) matchResult() (content []string, newImportance []int, err error) {
	defer func() {
		if x := recover(); x != nil {
			log.Printf("WARN: panic in %v", x)
			return
		}
	}()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(j.jin10_page))
	if err != nil {
		return nil, nil, err
	}
	var newContent = make([]string, 0)
	var importance int
	for i := 0; i <= LISTCOUNT; i++ {
		_, hasStyle := doc.Find("#newslist .newsline table").Eq(i).Attr("style")
		if hasStyle {
			importance = 2
		} else {
			importance = 1
		}
		newImportance = append(newImportance, importance)
		ID, hasID := doc.Find("#newslist .newsline").Eq(i).Attr("id")
		if !hasID {
			log.Println(errors.New("content match failed."))
		}
		rawContent := doc.Find("#content_" + ID).Text()
		if rawContent == "" {
			c := doc.Find("#newslist .newsline").Eq(i).Find("table table tr").Text()
			if c == "" {
				content = nil
				err = nil
				return
			}
			re, err := regexp.Compile("\\s{2,}")
			if err != nil {
				return nil, nil, err
			}
			c = re.ReplaceAllString(c, " ")
			c = strings.TrimSpace(c)
			src := strings.Split(c, " ")
			actual := strings.Replace(src[2], "实际：", "", -1)
			rawContent = src[1] + actual + "，" + src[4] + "，" + src[3] + "。"
		}
		keyword := []string{"jin10", "金十", "推荐阅读", "视频", "新品上线", "金10"}
		keywordSlice := make([]string, 0)
		for _, v := range keyword {
			rawBool := strings.Contains(rawContent, v)
			keywordSlice = append(keywordSlice, strconv.FormatBool(rawBool))
		}
		hasKeyword := strings.Join(keywordSlice, ",")
		if hasK := strings.Contains(hasKeyword, "true"); hasK {
			rawContent = ""
		}
		newContent = append(newContent, rawContent)
	}
	newContentStr := strings.Join(newContent, ",")
	oldContentSrt := strings.Join(prevContent, ",")
	if sameContent := strings.EqualFold(newContentStr, oldContentSrt); sameContent {
		content = nil
	} else {
		var count int
		for k, v := range newContent {
			if prevContent[0] == v {
				count = k
				break
			} else {
				count = len(newContent) - 1
			}
		}
		content = newContent[:count]
		prevContent = newContent
		f, err := os.OpenFile(PREVTXT, os.O_RDWR, 0777)
		defer f.Close()
		if err != nil {
			log.Println(err)
		}
		_, err = f.WriteString(strings.Join(content, ","))
		if err != nil {
			log.Println(err)
		}
	}
	err = nil
	return
}

func (j *Jin10) getPage(proxy string) error {
	url := "http://jin10.com/"
	resp, err := j.getByProxy(url, proxy)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	j.jin10_page = string(body)
	return nil
}

func (j Jin10) postContent(json []byte) error {
	body := bytes.NewBuffer(json)
	res, err := http.Post(apiURL, "application/json;charset=utf-8", body)
	if err != nil {
		return err
	}
	msg, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return err
	}
	log.Println(string(msg))
	return nil
}

// func (j Jin10) safeHandler(fn match) match {
// 	return func() (content string, err error) {
// 		defer func() {
// 			if e, ok := recover.(error); ok {
// 				log.Printf("WARN:panic in %v-%v", fn, e)
// 			}
// 		}()
// 		fn()
// 	}
// }

func init() {
	log.Println("Init...")
	if _, err := os.Stat(PREVTXT); os.IsNotExist(err) {
		file, err := os.Create(PREVTXT)
		defer file.Close()
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		f, err := os.Open(PREVTXT)
		defer f.Close()
		if err != nil {
			log.Println(err)
			return
		}
		b1 := make([]byte, 1024)
		_, err = f.Read(b1)
		if err != nil && err != io.EOF {
			log.Println(err)
			return
		}
		t := string(b1)
		prevContent = strings.Split(t, ",")
	}
}

func main() {
	log.Println("Start...")
	var jin10 = &Jin10{}
	//var end = make(chan bool)
	n := 0
	//ticker := time.NewTicker(time.Second * time.Duration((60 + rand.Intn(30))))
	//go func() {
	//for range ticker.C {
	for {
		hour := time.Now().Hour()
		min := time.Now().Minute()
		if (hour >= 0 && hour <= 6) || (hour == 23 && min >= 30) {
			err := jin10.getPage(proxy[n])
			if err != nil {
				log.Println(err)
				return
			}
			content, importance, err := jin10.matchResult()
			if err != nil {
				log.Println(err)
				return
			}
			if content == nil {
				log.Println("no new content")
			} else {
				for k, _ := range content {
					ts, err := jin10.dealTime()
					if err != nil {
						log.Println(err)
						return
					}
					jin10.Importance = importance[k]
					jin10.Content = content[k]
					jin10.CodeType = "markdown"
					jin10.Channels = []int{1}
					jin10.CreateAt = ts
					p, err := json.Marshal(jin10)
					if err != nil {
						log.Println(err)
						return
					}
					n++
					if n == len(proxy) {
						n = 0
					}
					if content[k] != "" {
						jin10.postContent(p)
					}
				}
			}
		} else {
			jin10.jin10_page = ""
			log.Println("Waiting...")
		}
		time.Sleep(time.Duration(rand.Intn(60)+60) * time.Second)
	}
	//}
	//}()
	//<-end
}
