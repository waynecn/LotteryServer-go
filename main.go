package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"time"

	"io/ioutil"

	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron"
)

type Lotterys struct {
	Id         int          `json:"id"`
	Lottery    string       `json:"lottery"`
	CreateTime sql.NullTime `json:"create_time"`
}

type History struct {
	Success  string     `json:"success"`
	Lotterys []Lotterys `json:"lotterys"`
}

type KjggData struct {
	State     int        `json:"state"`
	Message   string     `json:"message"`
	PageCount int        `json:"pageCount"`
	CountNum  int        `json:"countNum"`
	TFlag     int        `json:"Tflag"`
	Result    []KjggItem `json:"result"`
}

type KjggItem struct {
	Name        string            `json:"name"`
	Code        string            `json:"code"`
	DetailsLink string            `json:"detailsLink"`
	VideoLink   string            `json:"videoLink"`
	Date        string            `json:"date"`
	Week        string            `json:"week"`
	Red         string            `json:"red"`
	Blue        string            `json:"blue"`
	Sales       string            `json:"sales"`
	PoolMoney   string            `json:"poolmoney"`
	Content     string            `json:"content"`
	AddMoney    string            `json:"addmoney"`
	AddMoney2   string            `json:"addmoney2"`
	Msg         string            `json:"msg"`
	Z2Add       string            `json:"z2add"`
	M2Add       string            `json:"m2add"`
	PrizeGrades []PrizeGradesItem `json:"prizegrades"`
}

type PrizeGradesItem struct {
	Type      int    `json:"type"`
	TypeNum   string `json:"typenum"`
	TypeMoney string `json:"typemoney"`
}

var port = flag.String("p", "5134", "指定端口")
var db *sql.DB
var kjggUrl = "http://www.cwl.gov.cn/cwl_admin/front/cwlkj/search/kjxx/findDrawNotice?name=ssq&issueCount=30"
var kjggHistoryUrl = "http://www.cwl.gov.cn/cwl_admin/front/cwlkj/search/kjxx/findDrawNotice?name=ssq&issueCount=&issueStart=&issueEnd=&dayStart=2021-11-28&dayEnd=2022-02-08"

func main() {
	flag.Parse()

	_, err := strconv.Atoi(*port)
	if err != nil {
		*port = "5134"
	}

	// Create a simple file server
	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/", fs)

	//http request response
	http.HandleFunc("/lottery", lotteryFunc)
	http.HandleFunc("/lotteryHistory", lotteryHistoryFunc)

	//准备启动定时器 定时查询开奖公告以及历史开奖公告
	fmt.Println("ready start cron")
	c := cron.New()
	//c.AddFunc("* 31 21 * * ?", queryKjgg)	//每天21点31分
	c.AddFunc("*/10 * * * * ?", queryKjgg) //每10秒
	fmt.Println("start cron")
	c.Start()
	defer c.Stop()

	fmt.Println("准备启动服务,端口:", *port)
	err = http.ListenAndServe(":"+*port, nil)
	if err != nil {
		fmt.Println("启动http服务失败:", err)
		return
	}
}

func lotteryFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		io.WriteString(w, "只允许POST请求")
		return
	}

	var cnt = 0
	var redBalls []int64
	var blueBall int64
	//红区 1-33  6个号码
	for cnt < 6 {
		n, err := rand.Int(rand.Reader, big.NewInt(34))
		if err != nil {
			fmt.Println("rand int error:", err)
			continue
		}
		if n.Int64() == 0 {
			continue
		}
		//排除重复数字
		regenerate := false
		for i := 0; i < len(redBalls); i++ {
			if redBalls[i] == n.Int64() {
				regenerate = true
				break
			}
		}
		if regenerate {
			continue
		}
		cnt += 1
		redBalls = append(redBalls, n.Int64())
	}

	qsort(redBalls, 0, len(redBalls)-1)

	//蓝区 1-16 1个号码
	cnt = 0
	for cnt < 1 {
		n, err := rand.Int(rand.Reader, big.NewInt(17))
		if err != nil {
			fmt.Println("rand int error:", err)
			continue
		}
		if n.Int64() == 0 {
			continue
		}
		cnt += 1
		blueBall = n.Int64()
	}

	var resultStr string
	for index := range redBalls {
		resultStr += strconv.FormatInt(redBalls[index], 10) + " "
	}

	resultStr += strconv.FormatInt(blueBall, 10)

	//将生成结果保存到sqlite数据库中
	record(resultStr)

	var bts = []byte(resultStr)
	w.Write(bts)
}

func lotteryHistoryFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		io.WriteString(w, "只允许POST请求")
		return
	}

	results := getRecord()

	//his := History{"success", results}
	bts, err := json.Marshal(results)
	if err != nil {
		io.WriteString(w, "序列化查询结果失败")
		return
	}
	io.WriteString(w, string(bts))
}

func qsort(arr []int64, start int, end int) {
	if start >= end {
		return
	}

	key := start
	value := arr[start]
	for n := start + 1; n <= end; n++ {
		if arr[n] < value {
			arr[key] = arr[n]
			arr[n] = arr[key+1]
			key++
		}
	}

	arr[key] = value

	qsort(arr, start, key-1)
	qsort(arr, key+1, end)
}

func record(str string) {
	absDir, err := os.Getwd()
	if err != nil {
		fmt.Println("获取程序工作目录失败，错误描述：" + err.Error())
		return
	}
	db, err = sql.Open("sqlite3", absDir+"/serverDB.db")
	if err != nil {
		fmt.Printf("sqlite open failed:[%v]", err.Error())
		return
	}
	defer db.Close()

	tableSql := `CREATE TABLE IF NOT EXISTS "lottery" (
		"id" INTEGER PRIMARY KEY AUTOINCREMENT,
		"lottery" varchar(32) NULL,
		"create_time" TIMESTAMP default (datetime('now', 'localtime'))
	  );`
	_, err = db.Exec(tableSql)
	if err != nil {
		fmt.Println("初始化表格失败")
		return
	}

	insertStr := "insert into lottery (lottery) values(?);"
	stmt, err := db.Prepare(insertStr)
	if err != nil {
		fmt.Println("Prepare error:", err)
		return
	}
	res, err := stmt.Exec(str)
	if err != nil {
		fmt.Println("Exec error:", err)
		return
	}
	id, err := res.LastInsertId()
	if err != nil {
		fmt.Println("LastInsertId error:", err)
		return
	}
	fmt.Println("id:", id)
}

func getRecord() []Lotterys {
	absDir, err := os.Getwd()
	if err != nil {
		fmt.Println("获取程序工作目录失败，错误描述：" + err.Error())
		return nil
	}
	db, err = sql.Open("sqlite3", absDir+"/serverDB.db")
	if err != nil {
		fmt.Printf("sqlite open failed:[%v]", err.Error())
		return nil
	}
	defer db.Close()

	tableSql := `CREATE TABLE IF NOT EXISTS "lottery" (
		"id" INTEGER PRIMARY KEY AUTOINCREMENT,
		"lottery" varchar(32) NULL,
		"create_time" TIMESTAMP default (datetime('now', 'localtime'))
	  );`
	_, err = db.Exec(tableSql)
	if err != nil {
		fmt.Println("初始化表格失败")
		return nil
	}

	querySql := "select * from lottery order by create_time desc;"
	stmt, err := db.Prepare(querySql)
	if err != nil {
		fmt.Println("Prepare error:", err)
		return nil
	}
	rows, err := stmt.Query()
	if err != nil {
		fmt.Println("query error:", err)
		return nil
	}
	defer rows.Close()

	var results []Lotterys
	for rows.Next() {
		var item Lotterys
		err = rows.Scan(&item.Id, &item.Lottery, &item.CreateTime)
		if err != nil {
			fmt.Println("Scan error:", err)
			continue
		}

		results = append(results, item)
	}
	return results
}

func queryKjgg() {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", kjggUrl, nil)
	if err != nil {
		return
	}
	req.Header.Add("Referer", "http://www.cwl.gov.cn/ygkj/kjgg/")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.99 Safari/537.36")
	req.Header.Add("X-Requested-With", "XMLHttpRequest")
	req.Header.Add("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Add("Host", "www.cwl.gov.cn")
	req.Header.Add("Cookie", "_ga=GA1.3.1940681231.1595247885; HMF_CI=a9decaf585b61e962b2d7563d6120430c8baebe64c03b6b525f7807d8433cad4e4; 21_vq=15")

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	result, _ := ioutil.ReadAll(resp.Body)

	var kjggData KjggData
	err = json.Unmarshal(result, &kjggData)
	if err != nil {
		fmt.Println("结构化开奖公告结果发生错误:", err)
		return
	}
	bts, err := json.Marshal(kjggData.Result[0])
	fmt.Println("result 1:", string(bts))
}
