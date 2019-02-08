package main

import (
	"database/sql"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	_ "github.com/mattn/go-sqlite3"
	"github.com/tarm/serial"
	"gopkg.in/jdkato/prose.v2"
)

const SERIAL_RATE = 9600

var SERIAL_DEV = "/dev/ttyACM0"

const CAT_NUM = 6

var CAT = [CAT_NUM][30]string{
	{"biosphere", "planet", "world", "earth", "nature", "mother", "plants", "forest", "climate", "universe", "wild", "woods", "ocean", "sea", "life", "living", "creature", "animals", "organisms"},
	{"people", "crowd", "man", "men", "women", "anyone", "he", "she", "civilization", "spirits", "girl", "boy", "brother", "sister"},
	{"time", "forward", "times", "stillnes", "circumstances", "conditions", "past", "future", "counteract", "action", "avoid", "deduced", "perseverance", "heed", "seek", "will", "creation", "change"},
	{"laws", "deversion", "wisdom", "care", "observe", "cultivate", "cultivates", "fosters", "increase", "decrease", "form", "part", "sacrifice", "false", "true", "inner", "suffer"},
	{"beauty", "strength", "power", "difficult", "humor", "pleasant"},
	{"relationships", "union", "affection", "coution"},
}

type trigra struct {
	l1 bool
	l2 bool
	l3 bool
}

type hexa struct {
	top    trigra
	bottom trigra
}

func main() {
	port_name := os.Args[1:]
	if len(port_name) != 0 {
		SERIAL_DEV = port_name[0]
	}
	log.Println("SERIAL_DEV = ", SERIAL_DEV)

	db, err := sql.Open("sqlite3", "./iching.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
	create table if not exists questions(
		id integer primary key autoincrement,
		question text not null,
		hexagram text not null,
		result text not null,
		answer text not null)
	`)
	if err != nil {
		log.Fatal(err)
	}

	e := echo.New()
	e.Static("/static", "static")
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(setDB(db))
	e.GET("/", home)
	e.GET("/test/:h", test)
	e.GET("/sound", sound)
	e.POST("/question", question)
	e.Logger.Fatal(e.Start(":4444"))
}

func getDB(c echo.Context) (*sql.DB, error) {
	db, ok := c.Get("db").(*sql.DB)
	if !ok {
		return nil, errors.New("no db in context")
	}
	return db, nil
}

func setDB(db *sql.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("db", db)
			next(c)
			return nil
		}
	}
}

func home(c echo.Context) error {
	return c.HTML(http.StatusOK, HOME)
}

func test(c echo.Context) error {
	h := c.Param("h")
	q := "what the earth future?"
	a := iching(h, q)
	return c.HTML(http.StatusOK, RESULT_BEGIN+a+RESULT_END)
}

func sound(c echo.Context) error {
	com := exec.Command("play", "sound/1.mp3")
	if err := com.Run(); err != nil {
		log.Println("Error: ", err)
	}
	return c.HTML(http.StatusOK, "ok")
}

func question(c echo.Context) error {
	q := c.FormValue("q")
	if q == "" {
		return c.Redirect(http.StatusFound, "/")
	}
	db, err := getDB(c)
	if err != nil {
		log.Fatal(err)
	}
	t1, err := getFirstTrigram()
	if err != nil {
		log.Println("can't get first trigram!", err)
		return c.Redirect(http.StatusFound, "/")
	}
	log.Println("trigram1 =", t1)

	e1, err := getFirstElementReaction()
	if err != nil {
		log.Println("can't get first element!", err)
		return c.Redirect(http.StatusFound, "/")
	}
	log.Println("element1 =", e1)

	playSound(t1)

	t2, err := getSecondTrigram()
	if err != nil {
		log.Println("can't get second trigram!", err)
		return c.Redirect(http.StatusFound, "/")
	}
	log.Println("trigram2 =", t2)

	e2, err := getSecondElementReaction()
	if err != nil {
		log.Println("can't get second element!", err)
		return c.Redirect(http.StatusFound, "/")
	}
	log.Println("element2 =", e2)

	playSound(t2)

	h := buildHexagram(t1, t2)
	hs := hexaToString(h)
	r := resultByte(h, e1, e2)
	a := iching(hs, q)

	_, err = db.Exec("insert into questions (question, hexagram, result, answer) values (?, ?, ?, ?)", q, hs, r, a)
	if err != nil {
		log.Fatal(err)
	}

	return c.HTML(http.StatusOK, RESULT_BEGIN+a+RESULT_END)
}

func getFirstTrigram() (trigra, error) {
	t := trigra{}
	return t, t.getTrigram(1, 2, 3)
}

func getSecondTrigram() (trigra, error) {
	t := trigra{}
	return t, t.getTrigram(5, 6, 7)
}

func (t *trigra) getTrigram(i, j, k int) error {
	c := &serial.Config{Name: SERIAL_DEV, Baud: SERIAL_RATE}
	s, err := serial.OpenPort(c)
	log.Println("open serial")
	if err != nil {
		log.Println("can't open serial!!!", err)
		return err
	}
	log.Println("reading buf")
	buf := make([]byte, 16)
	n, err := s.Read(buf)
	if err != nil {
		log.Println("can't read buff!!!")
		return err
	}
	log.Println("buf = ", buf)
	log.Println("n = ", n)
	if n != 12 {
		return err
	}

	// set line 1
	num1, err := strconv.Atoi(string(buf[:1]))
	if err != nil {
		return err
	}
	val1, err := strconv.Atoi(string(buf[1:2]))
	if err != nil {
		return err
	}
	if num1 == i {
		if val1 == 1 {
			t.l1 = true
		}
	} else {
		return errors.New("wrong number!")
	}

	// set line 2
	num2, err := strconv.Atoi(string(buf[4:5]))
	if err != nil {
		return err
	}
	val2, err := strconv.Atoi(string(buf[5:6]))
	if err != nil {
		return err
	}
	if num2 == j {
		if val2 == 1 {
			t.l2 = true
		}
	} else {
		return errors.New("wrong number!")
	}

	// set line 3
	num3, err := strconv.Atoi(string(buf[8:9]))
	if err != nil {
		return err
	}
	val3, err := strconv.Atoi(string(buf[9:10]))
	if err != nil {
		return err
	}
	if num3 == k {
		if val3 == 1 {
			t.l3 = true
		}
	} else {
		return errors.New("wrong number!")
	}

	return nil
}

func getFirstElementReaction() (bool, error) {
	e, err := getElementByNum(4)
	if err != nil {
		return false, err
	}
	return e, nil
}

func getSecondElementReaction() (bool, error) {
	e, err := getElementByNum(8)
	if err != nil {
		return false, err
	}
	return e, nil
}

func getElementByNum(i int) (bool, error) {
	c := &serial.Config{Name: SERIAL_DEV, Baud: SERIAL_RATE}
	s, err := serial.OpenPort(c)
	if err != nil {
		return false, err
	}
	buf := make([]byte, 8)
	n, err := s.Read(buf)
	if err != nil {
		return false, err
	}
	log.Println(buf)
	log.Println(n)
	if n != 4 {
		return false, errors.New("it's not element value!")
	}
	num, err := strconv.Atoi(string(buf[:1]))
	if err != nil {
		return false, err
	}
	val, err := strconv.Atoi(string(buf[1:2]))
	if err != nil {
		return false, err
	}
	if num == i {
		if val == 1 {
			return true, nil
		} else {
			return false, nil
		}
	} else {
		return false, errors.New("wrong number!")
	}
}

func playSound(t trigra) {
	if t.l1 && t.l2 && t.l3 {
		log.Println("play: earth")
		com := exec.Command("play", "sound/earth.mp3")
		if err := com.Run(); err != nil {
			log.Println("Error: ", err)
		}
	}
	if !t.l1 && !t.l2 && t.l3 {
		log.Println("play: thunder")
		com := exec.Command("play", "sound/thunder.mp3")
		if err := com.Run(); err != nil {
			log.Println("Error: ", err)
		}
	}
	if t.l1 && !t.l2 && !t.l3 {
		log.Println("play: mountain")
		com := exec.Command("play", "sound/mountain.mp3")
		if err := com.Run(); err != nil {
			log.Println("Error: ", err)
		}
	}
}

func buildHexagram(t1 trigra, t2 trigra) hexa {
	h := hexa{
		top:    t1,
		bottom: t2,
	}
	return h
}

func hexaToString(h hexa) string {
	r := ""
	if h.top.l1 {
		r = r + "1"
	} else {
		r = r + "0"
	}
	if h.top.l2 {
		r = r + "1"
	} else {
		r = r + "0"
	}
	if h.top.l3 {
		r = r + "1"
	} else {
		r = r + "0"
	}
	if h.bottom.l1 {
		r = r + "1"
	} else {
		r = r + "0"
	}
	if h.bottom.l2 {
		r = r + "1"
	} else {
		r = r + "0"
	}
	if h.bottom.l3 {
		r = r + "1"
	} else {
		r = r + "0"
	}
	return r
}

func resultByte(h hexa, e1 bool, e2 bool) string {
	r := ""
	if h.top.l1 {
		r = r + "1"
	} else {
		r = r + "0"
	}
	if h.top.l2 {
		r = r + "1"
	} else {
		r = r + "0"
	}
	if h.top.l3 {
		r = r + "1"
	} else {
		r = r + "0"
	}
	if e1 {
		r = r + "1"
	} else {
		r = r + "0"
	}
	if h.bottom.l1 {
		r = r + "1"
	} else {
		r = r + "0"
	}
	if h.bottom.l2 {
		r = r + "1"
	} else {
		r = r + "0"
	}
	if h.bottom.l3 {
		r = r + "1"
	} else {
		r = r + "0"
	}
	if e2 {
		r = r + "1"
	} else {
		r = r + "0"
	}
	return r
}

func getHexNum(h string) int {
	trigrams := map[string]int{
		"111": 1,
		"011": 2,
		"101": 3,
		"001": 4,
		"110": 5,
		"010": 6,
		"100": 7,
		"000": 8,
	}
	c_table := [8][8]int{
		{1, 43, 14, 34, 9, 5, 26, 11},
		{10, 58, 38, 54, 61, 60, 41, 19},
		{13, 49, 30, 55, 37, 63, 22, 36},
		{25, 17, 21, 51, 42, 3, 27, 24},
		{44, 28, 50, 32, 57, 48, 18, 46},
		{6, 47, 64, 40, 59, 29, 4, 7},
		{33, 31, 56, 62, 53, 39, 52, 15},
		{12, 45, 35, 16, 20, 8, 23, 2},
	}
	top := 0
	bottom := 0
	for e, n := range trigrams {
		if e == h[:3] {
			top = n
		}
		if e == h[3:] {
			bottom = n
		}
	}
	return c_table[bottom-1][top-1]
}

func iching(hs string, q string) string {
	doc, err := prose.NewDocument(q)
	if err != nil {
		log.Println(err)
	}
	nns := []string{}
	for _, t := range doc.Tokens() {
		if t.Tag == "NN" {
			nns = append(nns, t.Text)
		}
	}

	hexNum := getHexNum(hs)
	b, err := ioutil.ReadFile("./text/ik-" + strconv.Itoa(hexNum) + ".txt")
	if err != nil {
		log.Print(err)
	}
	str := string(b)
	a := str[strings.Index(str, "THE JUDGMENT")+12 : strings.Index(str, "THE IMAGE")]
	a = strings.Replace(a, "\n", "<br>", -1)

	cat := [30]string{}
	for i := 0; i < CAT_NUM; i++ {
		for _, w := range CAT[i] {
			for _, nn := range nns {
				if w == nn {
					cat = CAT[i]
					// break
					// break
					// break
					words := strings.Fields(a)
					for i, w := range words {
						for _, c := range cat {
							if w == c {
								words[i] = nn
							}
						}
					}
					a = strings.Join(words[:], " ")
				}
			}
		}
	}

	// words := strings.Fields(a)
	// for i, w := range words {
	// 	for _, c := range cat {
	// 		if w == c {
	// 			words[i] = nns[0]
	// 		}
	// 	}
	// }
	//
	// a = strings.Join(words[:], " ")

	return a
}

const HOME = `
<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <link href="/static/bootstrap.min.css" rel="stylesheet" id="bootstrap-css">
        <title>IChing</title>
        <style type="text/css" scoped>
        .container {
            margin-top: 250px;
        }
        .stylish-input-group .input-group-addon{
            background: white !important;
        }
        .stylish-input-group .form-control{
            border-right:0;
            box-shadow:0 0 0;
            border-color:#ccc;
        }
        .stylish-input-group button{
            border:0;
            background:transparent;
        }
        </style>
    </head>
    <body>
        <div class="container">
            <div class="row">
                <div class="col-sm-6 col-sm-offset-3">
                    <div id="imaginary_container">
                        <form method="POST" action="/question">
                            <div class="input-group stylish-input-group">
                                <input type="text" name="q" class="form-control" placeholder="Type your question" >
                                <span class="input-group-addon">
                                    <button type="submit">
                                    </button>
                                </span>
                            </div>
                        </form>
                    </div>
                </div>
            </div>
        </div>
    </body>
</html>
`

const RESULT_BEGIN = `
<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <link href="/static/bootstrap.min.css" rel="stylesheet" id="bootstrap-css">
        <title>IChing</title>
        <style type="text/css" scoped>
        .container {
            margin-top: 42px;
        }
        </style>
    </head>
    <body>
        <div class="container">
            <div class="row">
                <div class="col-sm-6 col-sm-offset-3">
                    <div id="imaginary_container">`

const RESULT_END = `<br>
                    </div>
					<br>
					<div id="imaginary_container" style="text-align:center">
						<a class="btn btn-primary" href="/" role="button">Back</a>
                    </div>
					<br>
					<br>
                </div>
            </div>
        </div>
    </body>
</html>
`
