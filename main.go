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
	"sort"
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
	Zj1         string            `json:"zj1"` //add 20241118
	Mj1         string            `json:"mj1"` //add 20241118
	Zj6         string            `json:"zj6"` //add 20241118
	Mj6         string            `json:"mj6"` //add 20241118
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

type LotteryDatas struct {
	TotalCount  int `json:"totalcount"`
	FirstCount  int `json:"firstcount"`
	SecondCount int `json:"secondcount"`
	ThirdCount  int `json:"thirdcount"`
	ForthCount  int `json:"forthcount"`
	FifthCount  int `json:"fifthcount"`
	SixthCount  int `json:"sixthcount"`
}

var port = flag.String("p", "5134", "指定端口")
var db *sql.DB
var kjggUrl = "http://www.cwl.gov.cn/cwl_admin/front/cwlkj/search/kjxx/findDrawNotice?name=ssq&issueCount=30"
var kjggHistoryUrl = "http://www.cwl.gov.cn/cwl_admin/front/cwlkj/search/kjxx/findDrawNotice?name=ssq&issueCount=&issueStart=&issueEnd=&dayStart=2021-11-28&dayEnd=2022-02-08"

var redHistory [][]int
var blueHistory []int

func main() {
	flag.Parse()

	initLotterySqlite()
	defer db.Close()

	var err error
	_, err = strconv.Atoi(*port)
	if err != nil {
		*port = "5134"
	}

	// Create a simple file server
	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/", fs)

	//http request response
	//http.HandleFunc("/lottery", lotteryFunc)
	http.HandleFunc("/lottery", lotteryFuncUseMarkov)
	http.HandleFunc("/lotteryHistory", lotteryHistoryFunc)
	http.HandleFunc("/lotteryHistoryWithPage", lotteryHistoryFuncWithPage)
	http.HandleFunc("/queryKjgg", queryKjggImpl)
	http.HandleFunc("/loadData", loadDataImpl)

	redHistory, blueHistory, err = readHistoryDataFromSql("serverDB.db")
	if err != nil {
		fmt.Printf("读取历史数据失败：%v\n", err)
		return
	}

	//准备启动定时器 定时查询开奖公告以及历史开奖公告
	c := cron.New()
	c.AddFunc("0 40 21 * * ?", queryKjgg) //每天21点31分
	//c.AddFunc("*/10 * * * * ?", queryKjgg) //每10秒
	c.Start()
	defer c.Stop()

	go queryKjgg()

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
		n, err := rand.Int(rand.Reader, big.NewInt(33))
		if err != nil {
			fmt.Println("rand int error:", err)
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
		redBalls = append(redBalls, n.Int64()+1)
	}

	qsort(redBalls, 0, len(redBalls)-1)

	//蓝区 1-16 1个号码
	cnt = 0
	for cnt < 1 {
		n, err := rand.Int(rand.Reader, big.NewInt(16))
		if err != nil {
			fmt.Println("rand int error:", err)
			continue
		}
		cnt += 1
		blueBall = n.Int64() + 1
	}

	var resultStr string
	for index := range redBalls {
		redStr := strconv.FormatInt(redBalls[index], 10)
		if len(redStr) < 2 {
			redStr = "0" + redStr //单数补0
		}
		resultStr += redStr + " "
	}

	blueStr := strconv.FormatInt(blueBall, 10)
	if len(blueStr) < 2 {
		blueStr = "0" + blueStr //单数补0
	}
	resultStr += blueStr

	//将生成结果保存到sqlite数据库中
	record(resultStr)

	var bts = []byte(resultStr)
	w.Write(bts)
}

func lotteryFuncUseMarkov(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		io.WriteString(w, "只允许POST请求")
		return
	}

	// 2. 构建概率模型
	redTransition := buildRedTransition(redHistory) // 红球转移概率表
	blueProbs := buildBlueProbability(blueHistory)  // 蓝球频率表

	// 3. 生成号码
	redNumbers := generateRedNumbers(redTransition) // 生成红球
	blueNumber := generateBlueNumber(blueProbs)     // 生成蓝球

	// 4. 输出结果（红球按升序排列，符合双色球规则）
	sort.Ints(redNumbers)
	// fmt.Printf("🎫 双色球号码：\n")
	// fmt.Printf("红球：%v\n", redNumbers)
	// fmt.Printf("蓝球：%d\n", blueNumber)

	var resultStr string
	for index := range redNumbers {
		redStr := strconv.FormatInt(int64(redNumbers[index]), 10)
		if len(redStr) < 2 {
			redStr = "0" + redStr //单数补0
		}
		resultStr += redStr + " "
	}

	blueStr := strconv.FormatInt(int64(blueNumber), 10)
	if len(blueStr) < 2 {
		blueStr = "0" + blueStr //单数补0
	}
	resultStr += blueStr

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

	bts, err := json.Marshal(results)
	if err != nil {
		io.WriteString(w, "序列化查询结果失败")
		return
	}
	io.WriteString(w, string(bts))
}

func lotteryHistoryFuncWithPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		io.WriteString(w, "只允许POST请求")
		return
	}

	r.ParseForm()
	page := r.Form.Get("page")
	pagecount := r.Form.Get("pagecount")
	//fmt.Printf("page: %s pagecount: %s\n", page, pagecount)

	results := getRecordWithPage(page, pagecount)

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

func initLotterySqlite() {
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
		fmt.Println("初始化表格失败:", err)
		panic(err)
	}
}

func record(str string) {
	absDir, err := os.Getwd()
	if err != nil {
		fmt.Println("获取程序工作目录失败，错误描述：" + err.Error())
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
	fmt.Println("新增id:", id)
}

func getRecord() []Lotterys {
	absDir, err := os.Getwd()
	if err != nil {
		fmt.Println("获取程序工作目录失败，错误描述：" + err.Error())
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

func getRecordWithPage(page string, pagecount string) []Lotterys {
	absDir, err := os.Getwd()
	if err != nil {
		fmt.Println("获取程序工作目录失败，错误描述：" + err.Error())
		return nil
	}
	db, err := sql.Open("sqlite3", absDir+"/serverDB.db")
	if err != nil {
		fmt.Printf("sqlite open failed:[%v]", err.Error())
		return nil
	}
	defer db.Close()

	querySql := "select id, lottery, create_time,code, date, red, blue, my_prize_grade from lottery order by create_time desc;"
	if len(page) > 0 && len(pagecount) > 0 {
		pagenum, err := strconv.Atoi(page)
		if err != nil {
			fmt.Println("将page转为int错误:", err)
			return nil
		}
		pagecountnum, err := strconv.Atoi(pagecount)
		if err != nil {
			fmt.Println("将pagecount转为int错误:", err)
			return nil
		}
		if pagenum > 0 {
			pagenum = pagenum - 1
		}
		offset := pagenum * pagecountnum
		querySql = fmt.Sprintf("select id, lottery, create_time,code, date, red, blue, my_prize_grade from lottery order by create_time desc LIMIT %d OFFSET %d;", pagecountnum, offset)
	}
	//fmt.Println("querySql:", querySql)
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
		io.WriteString(w, "只允许GET请求")
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
		fmt.Println("结构化开奖公告结果发生错误:", err)
		return
	}
	for i := 0; i < len(kjggData.Result); i++ {
		//开奖日期
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

// 根据日期查询居于两个开奖公告之间的号码记录 并用于后续更新开奖结果到数据库中
func getLotteryRecord(startDate string, endDate string) []Lotterys {
	absDir, err := os.Getwd()
	if err != nil {
		fmt.Println("获取程序工作目录失败，错误描述：" + err.Error())
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

// 根据开奖结果计算号码是几等奖 返回：红球匹配数量 篮球匹配数量 几等奖
func calcMyPrizeGrade(myCode string, red string, blue string) (int, int, int) {
	myNumbers := strings.Split(myCode, " ") //红蓝一起 最后一个是蓝球
	redBalls := strings.Split(red, ",")     //开奖红球数组
	redCount := 0
	prizeGrade := 7
	for i := 0; i < 6; i++ {
		myRedBall, err := strconv.Atoi(myNumbers[i])
		if err != nil {
			fmt.Println("将号码转为int错误:", err)
			continue
		}
		for j := 0; j < 6; j++ {
			theirRedBall, err := strconv.Atoi(redBalls[j])
			if err != nil {
				fmt.Println("将号码转为int错误2:", err)
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
		fmt.Println("将我的蓝球转为int时错误:", err)
		return redCount, 0, prizeGrade
	}
	theirBlueBall, err := strconv.Atoi(blue)
	if err != nil {
		fmt.Println("将结果蓝球转为int时错误:", err)
		return redCount, 0, prizeGrade
	}
	blueFlag := false
	if myBlueBall == theirBlueBall {
		blueFlag = true
	}

	//根据红球 蓝球数量计算双色球属于几等奖
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
	fmt.Println("Id:", item.Id, " 号码", item.Lottery, " ", item.MyPrizeGrade, " 等奖")
	absDir, err := os.Getwd()
	if err != nil {
		fmt.Println("获取程序工作目录失败，错误描述：" + err.Error())
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
		fmt.Println("prepare update sql:", updateStr, " 错误:", err)
		return
	}

	ret, err := stmt.Exec(item.Code, item.DetailsLink, item.VideoLink, item.Date, item.Week, item.Red, item.Blue,
		item.Sales, item.PoolMoney, item.Content, item.MyPrizeGrade, item.RedCount, item.BlueCount, item.Id)
	if err != nil {
		fmt.Println("执行更新出错:", err)
		return
	}

	_, err = ret.RowsAffected()
	if err != nil {
		fmt.Println("更新ID:", item.Id, " 失败")
		return
	}
	fmt.Println("更新id:", item.Id, "成功")
}

func loadDataImpl(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		io.WriteString(w, "只允许POST请求")
		return
	}

	results := getDatas()

	bts, err := json.Marshal(results)
	//fmt.Printf("loadData:[%v]\n", string(bts))
	if err != nil {
		io.WriteString(w, "序列化查询结果失败")
		return
	}
	io.WriteString(w, string(bts))
}

func getDatas() []LotteryDatas {
	absDir, err := os.Getwd()
	if err != nil {
		fmt.Println("获取程序工作目录失败，错误描述：" + err.Error())
		return nil
	}
	db, err := sql.Open("sqlite3", absDir+"/serverDB.db")
	if err != nil {
		fmt.Printf("sqlite open failed:[%v]", err.Error())
		return nil
	}
	defer db.Close()

	//querySql := "select count(1) as totalcount from lottery order by create_time desc;"
	querySql := `select count(1) as totalcount, (select count(1) from lottery where my_prize_grade=1) as firstcount,
		(select count(1) from lottery where my_prize_grade=2) as secondcount,
		(select count(1) from lottery where my_prize_grade=3) as thirdcount,
		(select count(1) from lottery where my_prize_grade=4) as forthcount,
		(select count(1) from lottery where my_prize_grade=5) as fifthcount,
		(select count(1) from lottery where my_prize_grade=6) as sixthcount 
		 from lottery`
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

	var results []LotteryDatas
	for rows.Next() {
		var item LotteryDatas
		err = rows.Scan(&item.TotalCount, &item.FirstCount, &item.SecondCount, &item.ThirdCount, &item.ForthCount,
			&item.FifthCount, &item.SixthCount)
		if err != nil {
			fmt.Println("Scan error:", err)
			return nil
		}
		results = append(results, item)
	}
	return results
}
