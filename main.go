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
	"strings"
	"time"

	"io/ioutil"

	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron"
)

type Lotterys struct {
	Id            int            `json:"id"`
	Lottery       string         `json:"lottery"`
	CreateTime    sql.NullTime   `json:"create_time"`
	Code          sql.NullString `json:"code"`
	DetailsLink   string         `json:"detailsLink"`
	VideoLink     string         `json:"videoLink"`
	Date          sql.NullString `json:"date"`
	Week          string         `json:"week"`
	Red           sql.NullString `json:"red"`
	Blue          sql.NullString `json:"blue"`
	Sales         string         `json:"sales"`
	PoolMoney     string         `json:"poolmoney"`
	Content       string         `json:"content"`
	MyPrizeGrade  sql.NullInt32  `json:"myPrizeGrade"`
	CreateTimeStr string         `json:"create_time_str"`
	RedCount      sql.NullInt32
	BlueCount     sql.NullInt32
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

var port = flag.String("p", "5134", "????????????")
var db *sql.DB
var kjggUrl = "http://www.cwl.gov.cn/cwl_admin/front/cwlkj/search/kjxx/findDrawNotice?name=ssq&issueCount=30"
var kjggHistoryUrl = "http://www.cwl.gov.cn/cwl_admin/front/cwlkj/search/kjxx/findDrawNotice?name=ssq&issueCount=&issueStart=&issueEnd=&dayStart=2021-11-28&dayEnd=2022-02-08"

func main() {
	flag.Parse()

	initLotterySqlite()
	defer db.Close()

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
	http.HandleFunc("/queryKjgg", queryKjggImpl)

	//????????????????????? ????????????????????????????????????????????????
	c := cron.New()
	c.AddFunc("0 40 21 * * ?", queryKjgg) //??????21???31???
	//c.AddFunc("*/10 * * * * ?", queryKjgg) //???10???
	c.Start()
	defer c.Stop()

	go queryKjgg()

	fmt.Println("??????????????????,??????:", *port)
	err = http.ListenAndServe(":"+*port, nil)
	if err != nil {
		fmt.Println("??????http????????????:", err)
		return
	}
}

func lotteryFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		io.WriteString(w, "?????????POST??????")
		return
	}

	var cnt = 0
	var redBalls []int64
	var blueBall int64
	//?????? 1-33  6?????????
	for cnt < 6 {
		n, err := rand.Int(rand.Reader, big.NewInt(34))
		if err != nil {
			fmt.Println("rand int error:", err)
			continue
		}
		if n.Int64() == 0 {
			continue
		}
		//??????????????????
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

	//?????? 1-16 1?????????
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
		redStr := strconv.FormatInt(redBalls[index], 10)
		if len(redStr) < 2 {
			redStr = "0" + redStr //?????????0
		}
		resultStr += redStr + " "
	}

	blueStr := strconv.FormatInt(blueBall, 10)
	if len(blueStr) < 2 {
		blueStr = "0" + blueStr //?????????0
	}
	resultStr += blueStr

	//????????????????????????sqlite????????????
	record(resultStr)

	var bts = []byte(resultStr)
	w.Write(bts)
}

func lotteryHistoryFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		io.WriteString(w, "?????????POST??????")
		return
	}

	results := getRecord()

	bts, err := json.Marshal(results)
	if err != nil {
		io.WriteString(w, "???????????????????????????")
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

func initLotterySqlite() {
	absDir, err := os.Getwd()
	if err != nil {
		fmt.Println("????????????????????????????????????????????????" + err.Error())
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
		"create_time" TIMESTAMP default (datetime('now', 'localtime')),
		"code" varchar(8) NULL,
		"details_link" varchar(64) NULL,
		"video_link" varchar(64) NULL,
		"date" varchar(32) NULL,
		"week" varchar(8) NULL,
		"red" varchar(16) NULL,
		"blue" varchar(8) NULL,
		"sales" varchar(16) NULL,
		"pool_money" varchar(16) NULL,
		"content" varchar(255) NULL,
		"my_prize_grade" tinyint(2) NULL,
		"red_count" tinyint(2) NULL,
		"blue_count" tinyint(2) NULL
	  );`
	_, err = db.Exec(tableSql)
	if err != nil {
		fmt.Println("?????????????????????:", err)
		panic(err)
	}
}

func record(str string) {
	absDir, err := os.Getwd()
	if err != nil {
		fmt.Println("????????????????????????????????????????????????" + err.Error())
		return
	}
	db, err := sql.Open("sqlite3", absDir+"/serverDB.db")
	if err != nil {
		fmt.Printf("sqlite open failed:[%v]", err.Error())
		return
	}
	defer db.Close()

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
	fmt.Println("??????id:", id)
}

func getRecord() []Lotterys {
	absDir, err := os.Getwd()
	if err != nil {
		fmt.Println("????????????????????????????????????????????????" + err.Error())
		return nil
	}
	db, err := sql.Open("sqlite3", absDir+"/serverDB.db")
	if err != nil {
		fmt.Printf("sqlite open failed:[%v]", err.Error())
		return nil
	}
	defer db.Close()

	querySql := "select id, lottery, create_time,code, date, red, blue, my_prize_grade from lottery order by create_time desc;"
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
		err = rows.Scan(&item.Id, &item.Lottery, &item.CreateTime, &item.Code, &item.Date, &item.Red, &item.Blue, &item.MyPrizeGrade)
		if err != nil {
			fmt.Println("Scan error:", err)
			continue
		}

		if item.Red.Valid {
			item.Red.String = strings.Replace(item.Red.String, ",", " ", -1)
		}
		if item.CreateTime.Valid {
			item.CreateTimeStr = item.CreateTime.Time.Format("2006-01-02 15:04:05")
		}
		results = append(results, item)
	}
	return results
}

func queryKjggImpl(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		io.WriteString(w, "?????????GET??????")
		return
	}

	go queryKjgg()

	io.WriteString(w, "success")
}

func queryKjgg() {
	fmt.Println("queryKjgg")
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
		fmt.Println("???????????????????????????????????????:", err)
		return
	}
	for i := 0; i < len(kjggData.Result); i++ {
		//????????????
		kjDate := kjggData.Result[i].Date[:strings.Index(kjggData.Result[i].Date, "(")]
		fullKjDate := kjDate + " 21:30:00"
		if (i + 1) < len(kjggData.Result) {
			prevDate := kjggData.Result[i+1].Date[:strings.Index(kjggData.Result[i+1].Date, "(")]
			fullPrevDate := prevDate + " 21:30:00"
			results := getLotteryRecord(fullPrevDate, fullKjDate)
			for j := 0; j < len(results); j++ {
				item := results[j]
				item.Code = sql.NullString{kjggData.Result[i].Code, true}
				item.DetailsLink = kjggData.Result[i].DetailsLink
				item.VideoLink = kjggData.Result[i].VideoLink
				item.Date = sql.NullString{kjggData.Result[i].Date, true}
				item.Week = kjggData.Result[i].Week
				item.Red = sql.NullString{kjggData.Result[i].Red, true}
				item.Blue = sql.NullString{kjggData.Result[i].Blue, true}
				item.Sales = kjggData.Result[i].Sales
				item.PoolMoney = kjggData.Result[i].PoolMoney
				item.Content = kjggData.Result[i].Content
				redCount, blueCount, prizeGrade := calcMyPrizeGrade(item.Lottery, kjggData.Result[i].Red, kjggData.Result[i].Blue)
				item.MyPrizeGrade = sql.NullInt32{int32(prizeGrade), true}
				item.RedCount = sql.NullInt32{int32(redCount), true}
				item.BlueCount = sql.NullInt32{int32(blueCount), true}
				updateMyRecord(item)
			}
		}
	}
}

//??????????????????????????????????????????????????????????????? ????????????????????????????????????????????????
func getLotteryRecord(startDate string, endDate string) []Lotterys {
	absDir, err := os.Getwd()
	if err != nil {
		fmt.Println("????????????????????????????????????????????????" + err.Error())
		return nil
	}
	db, err := sql.Open("sqlite3", absDir+"/serverDB.db")
	if err != nil {
		fmt.Printf("sqlite open failed:[%v]", err.Error())
		return nil
	}
	defer db.Close()

	querySql := `select id, lottery from lottery where my_prize_grade is null and create_time between ? and ? order by create_time desc;`

	stmt, err := db.Prepare(querySql)
	if err != nil {
		fmt.Println("Prepare error:", err)
		return nil
	}
	rows, err := stmt.Query(startDate, endDate)
	if err != nil {
		fmt.Println("query error:", err)
		return nil
	}
	defer rows.Close()

	var results []Lotterys
	for rows.Next() {
		var item Lotterys
		err = rows.Scan(&item.Id, &item.Lottery)
		if err != nil {
			fmt.Println("Scan error:", err)
			continue
		}

		results = append(results, item)
	}
	return results
}

//?????????????????????????????????????????? ??????????????????????????? ?????????????????? ?????????
func calcMyPrizeGrade(myCode string, red string, blue string) (int, int, int) {
	myNumbers := strings.Split(myCode, " ") //???????????? ?????????????????????
	redBalls := strings.Split(red, ",")     //??????????????????
	redCount := 0
	prizeGrade := 7
	for i := 0; i < 6; i++ {
		myRedBall, err := strconv.Atoi(myNumbers[i])
		if err != nil {
			fmt.Println("???????????????int??????:", err)
			continue
		}
		for j := 0; j < 6; j++ {
			theirRedBall, err := strconv.Atoi(redBalls[j])
			if err != nil {
				fmt.Println("???????????????int??????2:", err)
				continue
			}
			if myRedBall == theirRedBall {
				redCount++
				break
			}
		}
	}

	myBlueBall, err := strconv.Atoi(myNumbers[6])
	if err != nil {
		fmt.Println("?????????????????????int?????????:", err)
		return redCount, 0, prizeGrade
	}
	theirBlueBall, err := strconv.Atoi(blue)
	if err != nil {
		fmt.Println("?????????????????????int?????????:", err)
		return redCount, 0, prizeGrade
	}
	blueFlag := false
	if myBlueBall == theirBlueBall {
		blueFlag = true
	}

	//???????????? ??????????????????????????????????????????
	if redCount == 6 {
		if blueFlag {
			prizeGrade = 1
		} else {
			prizeGrade = 2
		}
	} else if redCount == 5 {
		if blueFlag {
			prizeGrade = 3
		} else {
			prizeGrade = 4
		}
	} else if redCount == 4 {
		if blueFlag {
			prizeGrade = 4
		} else {
			prizeGrade = 5
		}
	} else if redCount <= 3 {
		if blueFlag && redCount == 3 {
			prizeGrade = 5
		} else if blueFlag {
			prizeGrade = 6
		} else {
			prizeGrade = 7
		}
	}
	blueCount := 0
	if blueFlag {
		blueCount = 1
	}
	return redCount, blueCount, prizeGrade
}

func updateMyRecord(item Lotterys) {
	fmt.Println("Id:", item.Id, " ??????", item.Lottery, " ", item.MyPrizeGrade, " ??????")
	absDir, err := os.Getwd()
	if err != nil {
		fmt.Println("????????????????????????????????????????????????" + err.Error())
		return
	}
	db, err := sql.Open("sqlite3", absDir+"/serverDB.db")
	if err != nil {
		fmt.Printf("sqlite open failed:[%v]", err.Error())
		return
	}
	defer db.Close()

	updateStr := `update lottery set code=?,details_link=?,video_link=?,date=?,week=?,red=?,blue=?,
		sales=?,pool_money=?,content=?,my_prize_grade=?,red_count=?,blue_count=? where id=?;`
	stmt, err := db.Prepare(updateStr)
	if err != nil {
		fmt.Println("prepare update sql:", updateStr, " ??????:", err)
		return
	}

	ret, err := stmt.Exec(item.Code, item.DetailsLink, item.VideoLink, item.Date, item.Week, item.Red, item.Blue,
		item.Sales, item.PoolMoney, item.Content, item.MyPrizeGrade, item.RedCount, item.BlueCount, item.Id)
	if err != nil {
		fmt.Println("??????????????????:", err)
		return
	}

	_, err = ret.RowsAffected()
	if err != nil {
		fmt.Println("??????ID:", item.Id, " ??????")
		return
	}
	fmt.Println("??????id:", item.Id, "??????")
}
