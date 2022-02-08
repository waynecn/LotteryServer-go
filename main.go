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

	_ "github.com/mattn/go-sqlite3"
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

var port = flag.String("p", "5134", "指定端口")
var db *sql.DB

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
